package tvt

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

var (
	onvifEntityTokenRe = regexp.MustCompile(`(?i)^entities_(\d+)_(\d+)_\d+$`)
	onvifProfileRe     = regexp.MustCompile(`(?i)^Profile_(\d+)_(\d+)$`)
	onvifRecordTokenRe = regexp.MustCompile(`(?i)^Record_(\d+)_\d+$`)
)

func buildWSSEHeader(user, pass string) (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	created := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	h := sha1.New()
	h.Write(nonce[:])
	h.Write([]byte(created))
	h.Write([]byte(pass))
	digestB64 := base64.StdEncoding.EncodeToString(h.Sum(nil))
	nonceB64 := base64.StdEncoding.EncodeToString(nonce[:])

	return fmt.Sprintf(`<s:Header><Security s:mustUnderstand="1" xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"><UsernameToken><Username>%s</Username><Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</Password><Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">%s</Nonce><Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">%s</Created></UsernameToken></Security></s:Header>`,
		xmlEscapeText(user), digestB64, nonceB64, created), nil
}

func (c *Client) soapCall(ctx context.Context, path, bodyXML string) ([]byte, error) {
	hdr, err := buildWSSEHeader(c.username, c.password)
	if err != nil {
		return nil, err
	}
	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:tds="http://www.onvif.org/ver10/device/wsdl" xmlns:trt="http://www.onvif.org/ver10/media/wsdl" xmlns:tr2="http://www.onvif.org/ver20/media/wsdl" xmlns:trc="http://www.onvif.org/ver10/recording/wsdl" xmlns:tse="http://www.onvif.org/ver10/search/wsdl" xmlns:trp="http://www.onvif.org/ver10/replay/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema" xmlns:tns1="http://www.onvif.org/ver10/topics">%s<s:Body>%s</s:Body></s:Envelope>`,
		hdr, bodyXML)

	u := fmt.Sprintf("%s/%s", c.baseURL, strings.TrimLeft(path, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(envelope))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tvt onvif %s: %w", path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w: onvif %s HTTP %d", ErrAuthFailed, path, resp.StatusCode)
	}
	if bytes.Contains(data, []byte("NotAuthorized")) || bytes.Contains(data, []byte("FailedAuthentication")) {
		return nil, fmt.Errorf("%w: onvif %s SOAP Fault", ErrAuthFailed, path)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tvt onvif %s HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func (c *Client) getDeviceInfoONVIF(ctx context.Context) (DeviceInfo, error) {
	data, err := c.soapCall(ctx, "/onvif/device_service", `<tds:GetDeviceInformation/>`)
	if err != nil {
		return DeviceInfo{}, err
	}
	var env struct {
		Body struct {
			Resp struct {
				Manufacturer    string `xml:"Manufacturer"`
				Model           string `xml:"Model"`
				FirmwareVersion string `xml:"FirmwareVersion"`
				SerialNumber    string `xml:"SerialNumber"`
				HardwareId      string `xml:"HardwareId"`
			} `xml:"GetDeviceInformationResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(data, &env); err != nil {
		return DeviceInfo{}, fmt.Errorf("tvt onvif GetDeviceInformation decode: %w", err)
	}
	r := env.Body.Resp
	if r.Model == "" && r.SerialNumber == "" {
		return DeviceInfo{}, fmt.Errorf("tvt onvif GetDeviceInformation empty response")
	}
	return DeviceInfo{
		Manufacturer:    strings.TrimSpace(r.Manufacturer),
		Model:           strings.TrimSpace(r.Model),
		FirmwareVersion: strings.TrimSpace(r.FirmwareVersion),
		SerialNumber:    strings.TrimSpace(r.SerialNumber),
		HardwareID:      strings.TrimSpace(r.HardwareId),
	}, nil
}

// GetNVRTimezoneLocation determines the NVR's local timezone offset from ONVIF
// GetSystemDateAndTime (comparing LocalDateTime vs UTCDateTime) so RTSP replay
// URLs always use the exact NVR wall-clock date & time.
func (c *Client) GetNVRTimezoneLocation(ctx context.Context) *time.Location {
	c.mu.Lock()
	if c.nvrLoc != nil {
		loc := c.nvrLoc
		c.mu.Unlock()
		return loc
	}
	c.mu.Unlock()

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	data, err := c.soapCall(tctx, "/onvif/device_service", `<tds:GetSystemDateAndTime/>`)
	if err != nil {
		return time.Local
	}
	type dtPart struct {
		Time struct {
			Hour   int `xml:"Hour"`
			Minute int `xml:"Minute"`
			Second int `xml:"Second"`
		} `xml:"Time"`
		Date struct {
			Year  int `xml:"Year"`
			Month int `xml:"Month"`
			Day   int `xml:"Day"`
		} `xml:"Date"`
	}
	var env struct {
		Body struct {
			Resp struct {
				Sys struct {
					UTC   dtPart `xml:"UTCDateTime"`
					Local dtPart `xml:"LocalDateTime"`
				} `xml:"SystemDateAndTime"`
			} `xml:"GetSystemDateAndTimeResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(data, &env); err != nil {
		return time.Local
	}
	u := env.Body.Resp.Sys.UTC
	l := env.Body.Resp.Sys.Local
	if u.Date.Year < 2000 || l.Date.Year < 2000 {
		return time.Local
	}
	utcT := time.Date(u.Date.Year, time.Month(u.Date.Month), u.Date.Day, u.Time.Hour, u.Time.Minute, u.Time.Second, 0, time.UTC)
	locAsUTC := time.Date(l.Date.Year, time.Month(l.Date.Month), l.Date.Day, l.Time.Hour, l.Time.Minute, l.Time.Second, 0, time.UTC)
	diffMin := int(math.Round(locAsUTC.Sub(utcT).Minutes()))
	if diffMin < -14*60 || diffMin > 14*60 {
		return time.Local
	}
	loc := time.FixedZone(fmt.Sprintf("TVT%+03d:%02d", diffMin/60, absInt(diffMin%60)), diffMin*60)
	c.mu.Lock()
	c.nvrLoc = loc
	c.mu.Unlock()
	return loc
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func normalizeONVIFCodec(enc string) string {
	switch strings.ToUpper(strings.TrimSpace(enc)) {
	case "H265", "HEVC":
		return "H.265"
	case "H264", "AVC":
		return "H.264"
	case "JPEG", "MJPEG", "MJPG":
		return "MJPEG"
	default:
		return strings.TrimSpace(enc)
	}
}

func normalizeONVIFAudio(enc string) string {
	switch strings.ToUpper(strings.TrimSpace(enc)) {
	case "AAC", "MPEG4-GENERIC":
		return "AAC"
	case "G711", "PCMA":
		return "G.711A"
	case "PCMU":
		return "G.711U"
	default:
		return strings.TrimSpace(enc)
	}
}

// probeStreamsONVIF queries /onvif/Media2 GetVideoEncoderConfigurations (which
// accurately distinguishes H265 vs H264 on TVT NVRs) and falls back to /onvif/Media1 GetProfiles.
func (c *Client) probeStreamsONVIF(ctx context.Context) ([]StreamInfo, error) {
	audioByCh := c.queryAudioCodecMapONVIF(ctx)

	if data2, err := c.soapCall(ctx, "/onvif/Media2", `<tr2:GetVideoEncoderConfigurations/>`); err == nil {
		var env2 struct {
			Body struct {
				Resp struct {
					Configs []struct {
						Token      string `xml:"token,attr"`
						GovLength  int    `xml:"GovLength,attr"`
						Profile    string `xml:"Profile,attr"`
						Name       string `xml:"Name"`
						Encoding   string `xml:"Encoding"`
						Resolution struct {
							Width  int `xml:"Width"`
							Height int `xml:"Height"`
						} `xml:"Resolution"`
						RateControl struct {
							CBR          string  `xml:"ConstantBitRate,attr"`
							FrameRate    float64 `xml:"FrameRateLimit"`
							BitrateLimit int     `xml:"BitrateLimit"`
						} `xml:"RateControl"`
					} `xml:"Configurations"`
				} `xml:"GetVideoEncoderConfigurationsResponse"`
			} `xml:"Body"`
		}
		if xml.Unmarshal(data2, &env2) == nil && len(env2.Body.Resp.Configs) > 0 {
			var out []StreamInfo
			for _, cfg := range env2.Body.Resp.Configs {
				ch1, st := 1, 0
				if m := onvifEntityTokenRe.FindStringSubmatch(strings.TrimSpace(cfg.Token)); len(m) == 3 {
					ch1, _ = strconv.Atoi(m[1])
					st, _ = strconv.Atoi(m[2])
				}
				if st > 1 || cfg.Resolution.Width <= 0 {
					continue
				}
				mode := "VBR"
				if strings.EqualFold(cfg.RateControl.CBR, "true") {
					mode = "CBR"
				}
				ac, aen := audioByCh[ch1], audioByCh[ch1] != ""
				out = append(out, StreamInfo{
					Channel:        ch1,
					Stream:         st,
					Width:          cfg.Resolution.Width,
					Height:         cfg.Resolution.Height,
					FPS:            int(math.Round(cfg.RateControl.FrameRate)),
					Compression:    normalizeONVIFCodec(cfg.Encoding),
					Profile:        strings.TrimSpace(cfg.Profile),
					AudioCodec:     ac,
					AudioEnable:    aen,
					GOP:            cfg.GovLength,
					BitRate:        cfg.RateControl.BitrateLimit,
					BitRateControl: mode,
					Name:           fmt.Sprintf("Channel %02d", ch1),
				})
			}
			if len(out) > 0 {
				sort.Slice(out, func(i, j int) bool {
					if out[i].Channel != out[j].Channel {
						return out[i].Channel < out[j].Channel
					}
					return out[i].Stream < out[j].Stream
				})
				return out, nil
			}
		}
	}

	// Fallback: ONVIF Media1 GetProfiles
	data1, err := c.soapCall(ctx, "/onvif/Media1", `<trt:GetProfiles/>`)
	if err != nil {
		return nil, err
	}
	var env1 struct {
		Body struct {
			Resp struct {
				Profiles []struct {
					Token string `xml:"token,attr"`
					Name  string `xml:"Name"`
					Video struct {
						Encoding   string `xml:"Encoding"`
						Resolution struct {
							Width  int `xml:"Width"`
							Height int `xml:"Height"`
						} `xml:"Resolution"`
						RateControl struct {
							FrameRateLimit int `xml:"FrameRateLimit"`
							BitrateLimit   int `xml:"BitrateLimit"`
						} `xml:"RateControl"`
						H264 struct {
							GovLength   int    `xml:"GovLength"`
							H264Profile string `xml:"H264Profile"`
						} `xml:"H264"`
					} `xml:"VideoEncoderConfiguration"`
					Audio struct {
						Encoding string `xml:"Encoding"`
					} `xml:"AudioEncoderConfiguration"`
				} `xml:"Profiles"`
			} `xml:"GetProfilesResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(data1, &env1); err != nil {
		return nil, fmt.Errorf("tvt onvif GetProfiles decode: %w", err)
	}
	var out []StreamInfo
	for idx, p := range env1.Body.Resp.Profiles {
		if p.Video.Resolution.Width <= 0 {
			continue
		}
		ch1, st := idx+1, 0
		if m := onvifProfileRe.FindStringSubmatch(strings.TrimSpace(p.Token)); len(m) == 3 {
			ch1, _ = strconv.Atoi(m[1])
			st, _ = strconv.Atoi(m[2])
		}
		if st > 1 {
			continue
		}
		ac := normalizeONVIFAudio(p.Audio.Encoding)
		out = append(out, StreamInfo{
			Channel:        ch1,
			Stream:         st,
			Width:          p.Video.Resolution.Width,
			Height:         p.Video.Resolution.Height,
			FPS:            p.Video.RateControl.FrameRateLimit,
			Compression:    normalizeONVIFCodec(p.Video.Encoding),
			Profile:        strings.TrimSpace(p.Video.H264.H264Profile),
			AudioCodec:     ac,
			AudioEnable:    ac != "",
			GOP:            p.Video.H264.GovLength,
			BitRate:        p.Video.RateControl.BitrateLimit,
			BitRateControl: "VBR",
			Name:           fmt.Sprintf("Channel %02d", ch1),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("tvt onvif ProbeAll returned 0 streams")
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Channel != out[j].Channel {
			return out[i].Channel < out[j].Channel
		}
		return out[i].Stream < out[j].Stream
	})
	return out, nil
}

func (c *Client) queryAudioCodecMapONVIF(ctx context.Context) map[int]string {
	out := make(map[int]string)
	data, err := c.soapCall(ctx, "/onvif/Media1", `<trt:GetProfiles/>`)
	if err != nil {
		return out
	}
	var env struct {
		Body struct {
			Resp struct {
				Profiles []struct {
					Token string `xml:"token,attr"`
					Audio struct {
						Encoding string `xml:"Encoding"`
					} `xml:"AudioEncoderConfiguration"`
				} `xml:"Profiles"`
			} `xml:"GetProfilesResponse"`
		} `xml:"Body"`
	}
	if xml.Unmarshal(data, &env) == nil {
		for _, p := range env.Body.Resp.Profiles {
			if m := onvifProfileRe.FindStringSubmatch(strings.TrimSpace(p.Token)); len(m) == 3 {
				ch1, _ := strconv.Atoi(m[1])
				if ac := normalizeONVIFAudio(p.Audio.Encoding); ac != "" {
					out[ch1] = ac
				}
			}
		}
	}
	return out
}

func (c *Client) getStreamInfoONVIF(ctx context.Context, channel0Based, stream int) (StreamInfo, error) {
	all, err := c.probeStreamsONVIF(ctx)
	if err != nil {
		return StreamInfo{}, err
	}
	wantCh1 := channel0Based + 1
	for _, s := range all {
		if s.Channel == wantCh1 && s.Stream == stream {
			return s, nil
		}
	}
	return StreamInfo{}, fmt.Errorf("tvt onvif stream not found for channel=%d stream=%d", wantCh1, stream)
}

func (c *Client) snapshotONVIF(ctx context.Context, channel0Based, stream int) ([]byte, error) {
	profToken := fmt.Sprintf("Profile_%d_%d", channel0Based+1, stream)
	data, err := c.soapCall(ctx, "/onvif/Media1", fmt.Sprintf(`<trt:GetSnapshotUri><trt:ProfileToken>%s</trt:ProfileToken></trt:GetSnapshotUri>`, xmlEscapeText(profToken)))
	if err != nil {
		return nil, err
	}
	var env struct {
		Body struct {
			Resp struct {
				MediaUri struct {
					Uri string `xml:"Uri"`
				} `xml:"MediaUri"`
			} `xml:"GetSnapshotUriResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(data, &env); err != nil || strings.TrimSpace(env.Body.Resp.MediaUri.Uri) == "" {
		return nil, fmt.Errorf("tvt onvif GetSnapshotUri empty")
	}
	snapURL := strings.TrimSpace(env.Body.Resp.MediaUri.Uri)
	if u, err := url.Parse(snapURL); err == nil {
		u.Host = net.JoinHostPort(c.host, strconv.Itoa(c.port))
		snapURL = u.String()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, snapURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.password)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	img, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil || len(img) < 200 || img[0] != 0xff || img[1] != 0xd8 {
		return nil, fmt.Errorf("tvt onvif snapshot HTTP %d did not return JPEG", resp.StatusCode)
	}
	return img, nil
}

func (c *Client) getRemoteDevicesONVIF(ctx context.Context) ([]dahua.RemoteChannel, error) {
	seen := make(map[int]bool)
	if data, err := c.soapCall(ctx, "/onvif/Recording", `<trc:GetRecordings/>`); err == nil {
		var env struct {
			Body struct {
				Resp struct {
					Items []struct {
						Token string `xml:"RecordingToken"`
					} `xml:"RecordingItem"`
				} `xml:"GetRecordingsResponse"`
			} `xml:"Body"`
		}
		if xml.Unmarshal(data, &env) == nil {
			for _, it := range env.Body.Resp.Items {
				if m := onvifRecordTokenRe.FindStringSubmatch(strings.TrimSpace(it.Token)); len(m) == 2 {
					if ch1, _ := strconv.Atoi(m[1]); ch1 > 0 {
						seen[ch1] = true
					}
				}
			}
		}
	}
	if streams, err := c.probeStreamsONVIF(ctx); err == nil {
		for _, s := range streams {
			if s.Stream == 0 && s.Channel > 0 {
				seen[s.Channel] = true
			}
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("tvt onvif GetRemoteDevices found no channels")
	}
	chList := make([]int, 0, len(seen))
	for ch1 := range seen {
		chList = append(chList, ch1)
	}
	sort.Ints(chList)

	out := make([]dahua.RemoteChannel, 0, len(chList))
	for _, ch1 := range chList {
		out = append(out, dahua.RemoteChannel{
			Channel: ch1 - 1,
			Name:    fmt.Sprintf("Channel %02d", ch1),
			Enable:  true,
		})
	}
	return out, nil
}

// findRecordingsONVIF queries ONVIF Profile G (/onvif/SearchRecording):
//  1. Calls GetRecordingInformation(Record_<ch>_0) in 1 roundtrip (~25ms) to verify
//     [EarliestRecording, LatestRecording] overlap.
//  2. For windows <= 6h (timeline / CheckVAR inspection), runs FindEvents filtered
//     strictly to tns1:RecordingHistory/Recording/State for second-accurate motion/continuous intervals.
//  3. For wide multi-day windows (> 6h, such as 72h Hybrid NVR virtual slicing),
//     emits clamped 1-hour segments across [max(start, Earliest), min(end, Latest)]
//     in O(1) latency without paging thousands of motion transitions.
func (c *Client) findRecordingsONVIF(ctx context.Context, channel0Based int, start, end time.Time) ([]dahua.Recording, error) {
	ch1 := channel0Based + 1
	recToken := fmt.Sprintf("Record_%d_0", ch1)

	infoXML, err := c.soapCall(ctx, "/onvif/SearchRecording",
		fmt.Sprintf(`<tse:GetRecordingInformation><tse:RecordingToken>%s</tse:RecordingToken></tse:GetRecordingInformation>`, xmlEscapeText(recToken)))
	if err != nil {
		return nil, err
	}
	var infoEnv struct {
		Body struct {
			Resp struct {
				Info struct {
					Earliest string `xml:"EarliestRecording"`
					Latest   string `xml:"LatestRecording"`
					Status   string `xml:"RecordingStatus"`
				} `xml:"RecordingInformation"`
			} `xml:"GetRecordingInformationResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(infoXML, &infoEnv); err != nil {
		return nil, fmt.Errorf("tvt onvif GetRecordingInformation decode: %w", err)
	}
	earliest, err1 := time.Parse(time.RFC3339, strings.TrimSpace(infoEnv.Body.Resp.Info.Earliest))
	latest, err2 := time.Parse(time.RFC3339, strings.TrimSpace(infoEnv.Body.Resp.Info.Latest))
	if err1 != nil || err2 != nil || !latest.After(earliest) {
		return nil, nil
	}
	winStart := start
	if earliest.After(winStart) {
		winStart = earliest.In(time.Local)
	}
	winEnd := end
	if latest.Before(winEnd) {
		winEnd = latest.In(time.Local)
	}
	if !winEnd.After(winStart) {
		return nil, nil
	}

	// Fast O(1) path for wide multi-day Hybrid NVR queries (> 6h).
	if winEnd.Sub(winStart) > 6*time.Hour {
		return sliceHourlyRecordings(ch1, winStart, winEnd), nil
	}

	// Fine-grained FindEvents path for <= 6h windows.
	findBody := fmt.Sprintf(`<tse:FindEvents><tse:StartPoint>%s</tse:StartPoint><tse:EndPoint>%s</tse:EndPoint><tse:Scope><tt:IncludedRecordings>%s</tt:IncludedRecordings></tse:Scope><tse:SearchFilter><tt:TopicExpression Dialect="http://www.onvif.org/ver10/tev/topicExpression/ConcreteSet">tns1:RecordingHistory/Recording/State</tt:TopicExpression></tse:SearchFilter><tse:IncludeStartState>true</tse:IncludeStartState><tse:KeepAliveTime>PT10S</tse:KeepAliveTime></tse:FindEvents>`,
		winStart.UTC().Format("2006-01-02T15:04:05Z"),
		winEnd.UTC().Format("2006-01-02T15:04:05Z"),
		xmlEscapeText(recToken),
	)
	feData, err := c.soapCall(ctx, "/onvif/SearchRecording", findBody)
	if err != nil {
		return sliceHourlyRecordings(ch1, winStart, winEnd), nil
	}
	var feEnv struct {
		Body struct {
			Resp struct {
				SearchToken string `xml:"SearchToken"`
			} `xml:"FindEventsResponse"`
		} `xml:"Body"`
	}
	if xml.Unmarshal(feData, &feEnv) != nil || strings.TrimSpace(feEnv.Body.Resp.SearchToken) == "" {
		return sliceHourlyRecordings(ch1, winStart, winEnd), nil
	}
	searchTok := strings.TrimSpace(feEnv.Body.Resp.SearchToken)
	defer func() {
		ectx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = c.soapCall(ectx, "/onvif/SearchRecording", fmt.Sprintf(`<tse:EndSearch><tse:SearchToken>%s</tse:SearchToken></tse:EndSearch>`, xmlEscapeText(searchTok)))
	}()

	type stateEv struct {
		at          time.Time
		isRecording bool
	}
	var events []stateEv
	for page := 0; page < 20; page++ {
		resBody := fmt.Sprintf(`<tse:GetEventSearchResults><tse:SearchToken>%s</tse:SearchToken><tse:MinResults>1</tse:MinResults><tse:MaxResults>200</tse:MaxResults><tse:WaitTime>PT2S</tse:WaitTime></tse:GetEventSearchResults>`,
			xmlEscapeText(searchTok))
		rData, err := c.soapCall(ctx, "/onvif/SearchRecording", resBody)
		if err != nil {
			break
		}
		var rEnv struct {
			Body struct {
				Resp struct {
					List struct {
						State   string `xml:"SearchState"`
						Results []struct {
							Time  string `xml:"Time"`
							Event struct {
								Topic   string `xml:"Topic"`
								Message struct {
									Inner struct {
										Data struct {
											Items []struct {
												Name  string `xml:"Name,attr"`
												Value string `xml:"Value,attr"`
											} `xml:"SimpleItem"`
										} `xml:"Data"`
									} `xml:"Message"`
								} `xml:"Message"`
							} `xml:"Event"`
						} `xml:"Result"`
					} `xml:"ResultList"`
				} `xml:"GetEventSearchResultsResponse"`
			} `xml:"Body"`
		}
		if xml.Unmarshal(rData, &rEnv) != nil {
			break
		}
		for _, item := range rEnv.Body.Resp.List.Results {
			if !strings.Contains(item.Event.Topic, "Recording/State") {
				continue
			}
			tm, err := time.Parse(time.RFC3339, strings.TrimSpace(item.Time))
			if err != nil {
				continue
			}
			for _, si := range item.Event.Message.Inner.Data.Items {
				if si.Name == "IsRecording" {
					events = append(events, stateEv{
						at:          tm.In(time.Local),
						isRecording: strings.EqualFold(si.Value, "true"),
					})
				}
			}
		}
		if strings.EqualFold(strings.TrimSpace(rEnv.Body.Resp.List.State), "Completed") {
			break
		}
	}

	if len(events) == 0 {
		return sliceHourlyRecordings(ch1, winStart, winEnd), nil
	}

	sort.Slice(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })
	var out []dahua.Recording
	var curStart time.Time
	recording := false
	for _, ev := range events {
		t := ev.at
		if t.Before(winStart) {
			t = winStart
		}
		if t.After(winEnd) {
			t = winEnd
		}
		if ev.isRecording {
			if !recording {
				curStart = t
				recording = true
			}
		} else if recording {
			if t.After(curStart) {
				out = append(out, makeDahuaRecording(channel0Based, curStart, t, 0))
			}
			recording = false
		}
	}
	if recording && winEnd.After(curStart) {
		out = append(out, makeDahuaRecording(channel0Based, curStart, winEnd, 0))
	}
	return coalesceRecordings(out), nil
}

func makeDahuaRecording(ch0 int, start, end time.Time, length int64) dahua.Recording {
	st := start.In(time.Local)
	et := end.In(time.Local)
	dur := int(et.Sub(st).Seconds() + 0.5)
	if dur < 1 {
		dur = 1
	}
	return dahua.Recording{
		Channel:     ch0,
		StartTime:   st.Format("2006-01-02 15:04:05"),
		EndTime:     et.Format("2006-01-02 15:04:05"),
		Duration:    dur,
		Type:        "dav",
		VideoStream: "Main",
		Length:      length,
		FilePath:    fmt.Sprintf("tvt://ch%d/%d-%d", ch0+1, st.Unix(), et.Unix()),
	}
}

func sliceHourlyRecordings(ch1 int, start, end time.Time) []dahua.Recording {
	var out []dahua.Recording
	for t := start; t.Before(end); {
		next := t.Add(time.Hour)
		if next.After(end) {
			next = end
		}
		out = append(out, makeDahuaRecording(ch1-1, t, next, 0))
		t = next
	}
	return out
}

func coalesceRecordings(in []dahua.Recording) []dahua.Recording {
	if len(in) <= 1 {
		return in
	}
	out := make([]dahua.Recording, 0, len(in))
	cur := in[0]
	for _, r := range in[1:] {
		curEnd, err1 := time.ParseInLocation("2006-01-02 15:04:05", cur.EndTime, time.Local)
		rStart, err2 := time.ParseInLocation("2006-01-02 15:04:05", r.StartTime, time.Local)
		rEnd, err3 := time.ParseInLocation("2006-01-02 15:04:05", r.EndTime, time.Local)
		curStart, err4 := time.ParseInLocation("2006-01-02 15:04:05", cur.StartTime, time.Local)
		if err1 == nil && err2 == nil && err3 == nil && err4 == nil && rStart.Sub(curEnd) <= 3*time.Second {
			if rEnd.After(curEnd) {
				cur.EndTime = r.EndTime
				cur.Duration = int(rEnd.Sub(curStart).Seconds() + 0.5)
			}
			cur.Length += r.Length
			continue
		}
		out = append(out, cur)
		cur = r
	}
	out = append(out, cur)
	return out
}

func (c *Client) rebootONVIF(ctx context.Context) error {
	_, err := c.soapCall(ctx, "/onvif/device_service", `<tds:SystemReboot/>`)
	return err
}
