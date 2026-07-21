package isapi

import (
	"context"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// hikTimeLayout is the timestamp format ISAPI's content-search/download
// interfaces accept and return: RFC3339 with a literal "Z" suffix
// (e.g. "2026-07-19T00:00:00Z"), confirmed live against a DS-7632NXI-K2.
//
// CRITICAL: despite the "Z", this NVR treats the value as **device-LOCAL
// wall-clock, ignoring the zone designator entirely** — proven live: a search
// returned a still-recording segment ending "14:49:23Z" while the device's own
// clock read 14:48 local (a recording cannot end in the future, so 14:49 must
// be local, not UTC), and the same numeric range sent with "Z" vs a "+07:00"
// offset returned identical results (the offset is ignored). So callers pass
// the device-local wall-clock and read the results as device-local — NO UTC
// conversion. A numeric offset other than a bare Z is mis-handled, so the "Z"
// form is what we always send.
const hikTimeLayout = "2006-01-02T15:04:05Z"

// Segment is one recorded media segment returned by SearchTrack. Start/End
// carry the device-LOCAL wall-clock (see hikTimeLayout — the device's "Z" is
// decorative), so format them directly for display; do NOT shift by any zone
// offset. Size is in bytes: read from the response's
// <size> element when present, else recovered from playbackURI's own "size"
// query parameter (Hikvision embeds it there — confirmed live — so a
// firmware that omits the XML element is still covered).
type Segment struct {
	Start, End             time.Time
	PlaybackURI            string
	ContentType, CodecType string
	Size                   int64
}

// cmSearchDescription is the POST /ISAPI/ContentMgmt/search request body.
// Field order/shape matches Hikvision's schema EXACTLY as verified live: a
// request missing contentTypeList or metadataList is rejected by the NVR
// with "Invalid XML Content / two root tags" (a misleading error for a
// missing-required-element problem). One trackID/timeSpan per request —
// this package always searches a single track/range at a time.
type cmSearchDescription struct {
	XMLName             xml.Name          `xml:"CMSearchDescription"`
	SearchID            string            `xml:"searchID"`
	TrackList           cmTrackList       `xml:"trackList"`
	TimeSpanList        cmTimeSpanList    `xml:"timeSpanList"`
	ContentTypeList     cmContentTypeList `xml:"contentTypeList"`
	MaxResults          int               `xml:"maxResults"`
	SearchResultPostion int               `xml:"searchResultPostion"` // sic — Hikvision's own (misspelled) tag name
	MetadataList        cmMetadataList    `xml:"metadataList"`
}

type cmTrackList struct {
	TrackID int `xml:"trackID"`
}

type cmTimeSpanList struct {
	TimeSpan cmTimeSpan `xml:"timeSpan"`
}

type cmTimeSpan struct {
	StartTime string `xml:"startTime"`
	EndTime   string `xml:"endTime"`
}

type cmContentTypeList struct {
	ContentType string `xml:"contentType"`
}

type cmMetadataList struct {
	MetadataDescriptor string `xml:"metadataDescriptor"`
}

// cmSearchResult is the CMSearchResult response to a search POST.
// responseStatus is "OK"/"MORE" on a page with matches, "NO MATCHES" when the
// range has none (NOT an error — SearchTrack treats it as an empty result).
type cmSearchResult struct {
	XMLName        xml.Name    `xml:"CMSearchResult"`
	SearchID       string      `xml:"searchID"`
	ResponseStatus string      `xml:"responseStatus"`
	NumOfMatches   int         `xml:"numOfMatches"`
	MatchList      cmMatchList `xml:"matchList"`
}

type cmMatchList struct {
	SearchMatchItem []cmSearchMatchItem `xml:"searchMatchItem"`
}

type cmSearchMatchItem struct {
	TrackID                int                      `xml:"trackID"`
	TimeSpan               cmTimeSpan               `xml:"timeSpan"`
	MediaSegmentDescriptor cmMediaSegmentDescriptor `xml:"mediaSegmentDescriptor"`
}

type cmMediaSegmentDescriptor struct {
	ContentType string `xml:"contentType"`
	CodecType   string `xml:"codecType"`
	Size        int64  `xml:"size"`
	PlaybackURI string `xml:"playbackURI"`
}

// SearchTrack lists recorded segments on one track over [start,end] via POST
// /ISAPI/ContentMgmt/search, paging through searchResultPostion until every
// numOfMatches segment has been collected. start/end are device-LOCAL
// wall-clock (see hikTimeLayout — the device ignores the zone designator and
// interprets the value as local); their wall-clock fields are sent verbatim,
// with no UTC conversion. trackID is channel*100+1 for a channel's main-stream
// recording track (verified live: 101 = channel 1). max sets the device's own
// page size (<maxResults>); pass <=0 for a sane default. The response can
// exceed the 1 MiB cap the config-call path (do) enforces, so this bypasses it
// entirely via doUnbounded.
func (c *Client) SearchTrack(ctx context.Context, trackID int, start, end time.Time, max int) ([]Segment, error) {
	if max <= 0 {
		max = 40
	}
	searchID := genSearchID()
	var out []Segment
	pos := 0
	for {
		reqBody := cmSearchDescription{
			SearchID:  searchID,
			TrackList: cmTrackList{TrackID: trackID},
			TimeSpanList: cmTimeSpanList{TimeSpan: cmTimeSpan{
				// Wall-clock sent verbatim (device-local; the "Z" is decorative).
				StartTime: start.Format(hikTimeLayout),
				EndTime:   end.Format(hikTimeLayout),
			}},
			ContentTypeList:     cmContentTypeList{ContentType: "video"},
			MaxResults:          max,
			SearchResultPostion: pos,
			MetadataList:        cmMetadataList{MetadataDescriptor: "//recordType.meta.std-cgi.com"},
		}
		body, err := xml.Marshal(reqBody)
		if err != nil {
			return out, fmt.Errorf("isapi: encode CMSearchDescription: %w", err)
		}
		full := append([]byte(xml.Header), body...)

		respBody, err := c.doUnbounded(ctx, http.MethodPost, "/ISAPI/ContentMgmt/search", full)
		if err != nil {
			return out, fmt.Errorf("isapi: search track %d: %w", trackID, err)
		}
		var res cmSearchResult
		if err := xml.Unmarshal(respBody, &res); err != nil {
			return out, fmt.Errorf("isapi: decode CMSearchResult: %w (body: %s)", err, truncate(respBody, 300))
		}
		if strings.EqualFold(strings.TrimSpace(res.ResponseStatus), "NO MATCHES") {
			break
		}

		for _, m := range res.MatchList.SearchMatchItem {
			seg := Segment{
				PlaybackURI: m.MediaSegmentDescriptor.PlaybackURI,
				ContentType: m.MediaSegmentDescriptor.ContentType,
				CodecType:   m.MediaSegmentDescriptor.CodecType,
				Size:        m.MediaSegmentDescriptor.Size,
			}
			if seg.Size == 0 {
				seg.Size = segmentSizeFromURI(seg.PlaybackURI)
			}
			if t, err := time.Parse(hikTimeLayout, m.TimeSpan.StartTime); err == nil {
				seg.Start = t
			}
			if t, err := time.Parse(hikTimeLayout, m.TimeSpan.EndTime); err == nil {
				seg.End = t
			}
			out = append(out, seg)
		}

		got := len(res.MatchList.SearchMatchItem)
		pos += got
		// Stop once every reported match has been collected, the device sent
		// an empty page (avoids spinning forever on a miscounted numOfMatches),
		// or we've hit a sanity ceiling far above any real recording history.
		if got == 0 || pos >= res.NumOfMatches || len(out) >= 20000 {
			break
		}
	}
	return out, nil
}

// doUnbounded is SearchTrack's request path: like do, but reads the FULL
// response body with no 1 MiB cap, over the timeout-unbound client (a dense
// search result list can be slow to fully enumerate on an NVR with a long
// recording history). Only the stdlib HTTP transport implements it — the
// optional cgo SDK transport doesn't do content search/download in this
// milestone (ISAPI-over-HTTP is the pure-Go path per docs/DEPLOYMENT.md).
func (c *Client) doUnbounded(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	t, ok := c.rt.(*httpTransport)
	if !ok {
		return nil, fmt.Errorf("isapi: %s %s: requires the HTTP transport (unsupported over this Client's Transport)", method, path)
	}
	resp, err := t.rawRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("isapi: read response %s %s: %w", method, path, err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return data, fmt.Errorf("isapi: %s %s: unauthorized (digest auth failed): %s", method, path, truncate(data, 300))
	}
	if resp.StatusCode >= 300 {
		if statusErr := checkResponseStatus(data); statusErr != nil {
			return data, fmt.Errorf("isapi: %s %s: HTTP %d: %w", method, path, resp.StatusCode, statusErr)
		}
		return data, fmt.Errorf("isapi: %s %s: HTTP %d: %s", method, path, resp.StatusCode, truncate(data, 300))
	}
	return data, nil
}

// genSearchID returns a fresh UUID-v4-shaped searchID for one SearchTrack
// call's pages (Hikvision's schema wants one; it's used only to correlate
// pages of a single logical search, not as a security token, so a fallback
// fixed value on the near-impossible rand failure is acceptable).
func genSearchID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-1000-8000-000000000000"
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

// segmentSizeFromURI recovers a segment's byte size from its playbackURI's
// own "size" query parameter (Hikvision embeds it there — confirmed live —
// e.g. "...&name=00000001195000000&size=1062807408"), used when the response
// XML's <size> element is absent.
func segmentSizeFromURI(uri string) int64 {
	u, err := url.Parse(uri)
	if err != nil {
		return 0
	}
	s := u.Query().Get("size")
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
