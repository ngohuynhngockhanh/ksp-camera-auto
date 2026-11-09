package dahua

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

func resolveCGIHTTPPort(port int) int {
	if port <= 0 || port == 37777 || port == 37778 || port == 8888 || port == 554 {
		return 80
	}
	return port
}

func escapeCGITime(t time.Time) string {
	return strings.ReplaceAll(url.QueryEscape(t.Format(deviceTimeLayout)), "+", "%20")
}

// FindRecordingsCGI lists recorded .dav segments on a Dahua/KBVision NVR via
// stateless HTTP Digest CGI (/cgi-bin/mediaFileFind.cgi). Unlike DVRIP 0xf6
// mediaFileFind (which can stall on ST7108/NVR4X-4KS2 firmware or restricted
// ONVIF/RTSP accounts such as "root"), mediaFileFind.cgi completes a 72h scan
// in ~450ms and releases its handle cleanly.
//
// channel is 0-based (matching Client.FindRecordings); Dahua's HTTP CGI API
// uses 1-based channel numbers (condition.Channel=1 -> /000/dav/).
func FindRecordingsCGI(ctx context.Context, host string, port int, user, pass string, channel int, start, end time.Time) ([]Recording, error) {
	httpPort := resolveCGIHTTPPort(port)
	baseTransport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 4 * time.Second}).DialContext,
		ResponseHeaderTimeout: 8 * time.Second,
		IdleConnTimeout:       15 * time.Second,
	}
	client := &http.Client{
		Transport: isapi.NewDigestTransport(user, pass, baseTransport),
		Timeout:   12 * time.Second,
	}
	baseURL := fmt.Sprintf("http://%s/cgi-bin/mediaFileFind.cgi", net.JoinHostPort(host, strconv.Itoa(httpPort)))

	doGet := func(reqCtx context.Context, query string) (string, error) {
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"?"+query, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, snapshotTruncate(body, 160))
		}
		return strings.TrimSpace(string(body)), nil
	}

	createOut, err := doGet(ctx, "action=factory.create")
	if err != nil {
		return nil, fmt.Errorf("dahua cgi mediaFileFind create %s: %w", host, err)
	}
	objID := ""
	for _, line := range strings.Split(createOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "result=") {
			objID = strings.TrimSpace(strings.TrimPrefix(line, "result="))
			break
		}
	}
	if objID == "" {
		return nil, fmt.Errorf("dahua cgi mediaFileFind create %s: missing object id in %q", host, createOut)
	}

	defer func() {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = doGet(cleanCtx, "action=close&object="+url.QueryEscape(objID))
		_, _ = doGet(cleanCtx, "action=destroy&object="+url.QueryEscape(objID))
	}()

	ch1 := channel + 1
	if ch1 < 1 {
		ch1 = 1
	}
	findQuery := fmt.Sprintf(
		"action=findFile&object=%s&condition.Channel=%d&condition.StartTime=%s&condition.EndTime=%s",
		url.QueryEscape(objID),
		ch1,
		escapeCGITime(start),
		escapeCGITime(end),
	)
	findOut, err := doGet(ctx, findQuery)
	if err != nil {
		return nil, fmt.Errorf("dahua cgi findFile %s: %w", host, err)
	}
	if !strings.Contains(findOut, "OK") {
		return nil, nil
	}

	const pageSize = 100
	var out []Recording
	for {
		nextQuery := fmt.Sprintf("action=findNextFile&object=%s&count=%d", url.QueryEscape(objID), pageSize)
		nextOut, err := doGet(ctx, nextQuery)
		if err != nil {
			return out, nil
		}
		found, items := parseCGIFindNextFile(nextOut, channel)
		if found <= 0 || len(items) == 0 {
			break
		}
		out = append(out, items...)
		if found < pageSize || len(out) >= 20000 {
			break
		}
	}
	return out, nil
}

func parseCGIFindNextFile(raw string, normChannel int) (int, []Recording) {
	found := 0
	byIdx := make(map[int]*Recording)
	var indices []int

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "found=") {
			found, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "found=")))
			continue
		}
		if !strings.HasPrefix(line, "items[") {
			continue
		}
		closeBr := strings.IndexByte(line, ']')
		eq := strings.IndexByte(line, '=')
		if closeBr <= 6 || eq <= closeBr+1 {
			continue
		}
		idx, err := strconv.Atoi(line[6:closeBr])
		if err != nil {
			continue
		}
		field := line[closeBr+2 : eq]
		val := strings.TrimSpace(line[eq+1:])

		rec, exists := byIdx[idx]
		if !exists {
			rec = &Recording{Channel: normChannel, Type: "dav"}
			byIdx[idx] = rec
			indices = append(indices, idx)
		}
		switch field {
		case "StartTime":
			rec.StartTime = val
		case "EndTime":
			rec.EndTime = val
		case "Length":
			rec.Length, _ = strconv.ParseInt(val, 10, 64)
		case "Duration":
			rec.Duration, _ = strconv.Atoi(val)
		case "FilePath":
			rec.FilePath = val
		case "Type":
			if val != "" {
				rec.Type = val
			}
		case "VideoStream":
			rec.VideoStream = val
		}
	}

	sort.Ints(indices)
	out := make([]Recording, 0, len(indices))
	for _, idx := range indices {
		r := byIdx[idx]
		if r.StartTime == "" || r.EndTime == "" {
			continue
		}
		if r.Duration <= 0 {
			if st, e1 := time.ParseInLocation(deviceTimeLayout, r.StartTime, time.Local); e1 == nil {
				if et, e2 := time.ParseInLocation(deviceTimeLayout, r.EndTime, time.Local); e2 == nil && et.After(st) {
					r.Duration = int(et.Sub(st).Seconds())
				}
			}
		}
		out = append(out, *r)
	}
	return found, out
}

// StreamDavCGI downloads a [start, end] recording slice as raw DHAV (.dav)
// bytes via HTTP Digest GET /cgi-bin/loadfile.cgi?action=startLoad (~5+ MB/s
// on LAN, 0% CPU). channel is 0-based and converted to 1-based for loadfile.cgi.
func StreamDavCGI(ctx context.Context, w io.Writer, host string, port int, user, pass string, channel int, start, end time.Time) error {
	select {
	case playbackSem <- struct{}{}:
		defer func() { <-playbackSem }()
	case <-ctx.Done():
		return fmt.Errorf("dahua cgi loadfile %s: %w", host, ctx.Err())
	}

	httpPort := resolveCGIHTTPPort(port)
	baseTransport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		ResponseHeaderTimeout: 12 * time.Second,
		IdleConnTimeout:       15 * time.Second,
	}
	client := &http.Client{
		Transport: isapi.NewDigestTransport(user, pass, baseTransport),
	}

	ch1 := channel + 1
	if ch1 < 1 {
		ch1 = 1
	}
	u := fmt.Sprintf(
		"http://%s/cgi-bin/loadfile.cgi?action=startLoad&channel=%d&startTime=%s&endTime=%s&subtype=0",
		net.JoinHostPort(host, strconv.Itoa(httpPort)),
		ch1,
		escapeCGITime(start),
		escapeCGITime(end),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("dahua cgi loadfile %s: %w", host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("dahua cgi loadfile %s: HTTP %d: %s", host, resp.StatusCode, strings.TrimSpace(string(b)))
	}

	written, err := io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("dahua cgi loadfile %s copy: %w", host, err)
	}
	if written < 1024 {
		return fmt.Errorf("dahua cgi loadfile %s: short stream (%d bytes)", host, written)
	}
	return nil
}
