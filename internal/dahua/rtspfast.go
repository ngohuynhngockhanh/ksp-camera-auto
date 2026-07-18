package dahua

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// StreamPlaybackFast streams a channel's [start,end] recording to w as MP4 at
// the camera's fast download rate (~6x realtime), versus the ~1x of normal
// playback (StreamPlayback). The speedup comes from the RTSP "Rate-Control: no"
// header, which tells Dahua to send the recording as fast as the link allows
// instead of pacing it to real time — confirmed live at ~6x on a wired cam.
//
// ffmpeg cannot set that header, so a tiny in-process RTSP proxy sits between
// ffmpeg and the camera: ffmpeg connects to it unauthenticated over plain
// interleaved-TCP RTSP, and the proxy (a) rewrites request URIs to the camera,
// (b) performs digest authentication toward the camera, and (c) injects
// "Rate-Control: no" into the PLAY request. All media is TCP-interleaved (no
// UDP loss), remuxed with -c copy, and piped straight to w — nothing is written
// to the box's disk.
func StreamPlaybackFast(ctx context.Context, w io.Writer, host, user, pass string, channel int, start, end time.Time) error {
	select {
	case playbackSem <- struct{}{}:
		defer func() { <-playbackSem }()
	case <-ctx.Done():
		return fmt.Errorf("dahua: fast playback %s: %w (waiting for a slot)", host, ctx.Err())
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("dahua: fast playback: listen: %w", err)
	}
	defer ln.Close()
	proxyAddr := ln.Addr().String()

	const f = "2006_01_02_15_04_05"
	camPath := fmt.Sprintf("/cam/playback?channel=%d&subtype=0&starttime=%s&endtime=%s", channel+1, start.Format(f), end.Format(f))

	proxyErr := make(chan error, 1)
	go func() {
		conn, aerr := ln.Accept()
		if aerr != nil {
			proxyErr <- aerr
			return
		}
		defer conn.Close()
		proxyErr <- runRTSPProxy(ctx, conn, host, user, pass)
	}()

	// ffmpeg talks plain RTSP to the proxy (no creds); the proxy authenticates
	// to the camera. -rtsp_transport tcp keeps everything interleaved.
	proxyURL := fmt.Sprintf("rtsp://%s%s", proxyAddr, camPath)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-rtsp_transport", "tcp",
		"-i", proxyURL,
		"-c", "copy",
		"-fflags", "+genpts",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"-y", "pipe:1",
	)
	cmd.Stdout = w
	stderr := &tailWriter{max: 4096}
	cmd.Stderr = stderr
	runErr := cmd.Run()
	// The proxy exits when ffmpeg closes the connection; drain its result but
	// prefer ffmpeg's error for the caller.
	select {
	case <-proxyErr:
	case <-time.After(time.Second):
	}
	if runErr != nil {
		return fmt.Errorf("dahua: ffmpeg fast playback %s: %w: %s", host, runErr, snapshotTail(stderr.buf, 300))
	}
	return nil
}

// runRTSPProxy relays RTSP between ffmpeg (client) and the camera, handling the
// request phase specially (URI rewrite + digest auth + Rate-Control injection)
// and then relaying the interleaved media transparently until either side ends.
func runRTSPProxy(ctx context.Context, client net.Conn, host, user, pass string) error {
	camConn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "554"), 10*time.Second)
	if err != nil {
		return fmt.Errorf("proxy: dial cam: %w", err)
	}
	defer camConn.Close()
	go func() { <-ctx.Done(); client.Close(); camConn.Close() }()

	camHostPort := net.JoinHostPort(host, "554")
	proxyHostPort := client.LocalAddr().String()
	cr := bufio.NewReader(client)
	camR := bufio.NewReader(camConn)
	auth := &rtspAuth{user: user, pass: pass}

	for {
		method, reqURI, headers, body, err := readRTSPRequest(cr)
		if err != nil {
			return nil // ffmpeg closed / done
		}
		if method == "PLAY_DATA_PHASE" {
			break
		}
		// Rewrite the request-URI host (proxy -> camera) for everything sent on.
		camURI := strings.Replace(reqURI, "rtsp://"+proxyHostPort, "rtsp://"+camHostPort, 1)
		extra := map[string]string{}
		if method == "PLAY" {
			extra["Rate-Control"] = "no"
		}
		resp, err := auth.doCam(camR, camConn, method, camURI, headers, body, extra)
		if err != nil {
			return err
		}
		// Rewrite Content-Base (camera -> proxy) so ffmpeg's follow-up SETUP URIs
		// come back to us.
		resp = bytes.ReplaceAll(resp, []byte("rtsp://"+camHostPort), []byte("rtsp://"+proxyHostPort))
		if _, err := client.Write(resp); err != nil {
			return err
		}
		if method == "PLAY" {
			break
		}
	}

	// Media phase: relay both directions verbatim (interleaved RTP + any
	// keepalive/teardown) until either side closes.
	done := make(chan struct{}, 2)
	go func() { io.Copy(client, io.MultiReader(bytes.NewReader(drain(camR)), camConn)); done <- struct{}{} }()
	go func() { io.Copy(camConn, io.MultiReader(bytes.NewReader(drain(cr)), client)); done <- struct{}{} }()
	<-done
	return nil
}

// drain returns whatever bytes a bufio.Reader has already buffered (so the
// switch from request-parsing to raw relay doesn't lose the first interleaved
// bytes that arrived alongside the PLAY response).
func drain(r *bufio.Reader) []byte {
	n := r.Buffered()
	if n == 0 {
		return nil
	}
	b, _ := r.Peek(n)
	out := make([]byte, len(b))
	copy(out, b)
	r.Discard(n)
	return out
}

// readRTSPRequest reads one RTSP request (request line + headers + optional
// body). Returns method "PLAY_DATA_PHASE" if the next byte is interleaved data
// ('$') rather than a request.
func readRTSPRequest(r *bufio.Reader) (method, uri string, headers []string, body []byte, err error) {
	first, err := r.Peek(1)
	if err != nil {
		return "", "", nil, nil, err
	}
	if first[0] == '$' {
		return "PLAY_DATA_PHASE", "", nil, nil, nil
	}
	line, err := readLine(r)
	if err != nil {
		return "", "", nil, nil, err
	}
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return "", "", nil, nil, fmt.Errorf("bad request line %q", line)
	}
	method, uri = parts[0], parts[1]
	contentLen := 0
	for {
		h, herr := readLine(r)
		if herr != nil {
			return "", "", nil, nil, herr
		}
		if h == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(h), "content-length:") {
			contentLen, _ = strconv.Atoi(strings.TrimSpace(h[len("content-length:"):]))
		}
		// Drop the client's CSeq/Authorization/User-Agent? Keep CSeq + others;
		// strip Authorization (we add our own to the camera).
		if strings.HasPrefix(strings.ToLower(h), "authorization:") {
			continue
		}
		headers = append(headers, h)
	}
	if contentLen > 0 {
		body = make([]byte, contentLen)
		if _, err = io.ReadFull(r, body); err != nil {
			return "", "", nil, nil, err
		}
	}
	return method, uri, headers, body, nil
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// rtspAuth performs digest auth toward the camera, learning realm/nonce from
// the first 401 and reusing them.
type rtspAuth struct {
	user, pass   string
	realm, nonce string
	qop          string
}

// doCam sends one RTSP request to the camera, retrying once with digest auth on
// a 401, and returns the final raw response (status line + headers + body).
func (a *rtspAuth) doCam(camR *bufio.Reader, cam net.Conn, method, uri string, headers []string, body []byte, extra map[string]string) ([]byte, error) {
	send := func() ([]byte, error) {
		var b strings.Builder
		fmt.Fprintf(&b, "%s %s RTSP/1.0\r\n", method, uri)
		// Preserve ffmpeg's own CSeq (and every other header) verbatim so the
		// response it gets back echoes the CSeq it is waiting for.
		for _, h := range headers {
			b.WriteString(h)
			b.WriteString("\r\n")
		}
		for k, v := range extra {
			fmt.Fprintf(&b, "%s: %s\r\n", k, v)
		}
		if a.nonce != "" {
			b.WriteString("Authorization: " + a.authHeader(method, uri) + "\r\n")
		}
		b.WriteString("\r\n")
		if len(body) > 0 {
			b.Write(body)
		}
		_ = cam.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := cam.Write([]byte(b.String())); err != nil {
			return nil, err
		}
		return readRTSPResponse(camR)
	}
	resp, err := send()
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(resp, []byte("RTSP/1.0 401")) {
		a.parseChallenge(resp)
		// Rewrite the CSeq we echo to ffmpeg to match its original request; but
		// ffmpeg tracks its own CSeq, so return the retried response as-is with
		// its CSeq rewritten below by the caller path. Simpler: retry.
		resp, err = send()
		if err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// readRTSPResponse reads one RTSP response (status line + headers + body by
// Content-Length).
func readRTSPResponse(r *bufio.Reader) ([]byte, error) {
	var raw bytes.Buffer
	contentLen := 0
	// status line
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	raw.WriteString(line)
	for {
		h, herr := r.ReadString('\n')
		if herr != nil {
			return nil, herr
		}
		raw.WriteString(h)
		t := strings.TrimRight(h, "\r\n")
		if t == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(t), "content-length:") {
			contentLen, _ = strconv.Atoi(strings.TrimSpace(t[len("content-length:"):]))
		}
	}
	if contentLen > 0 {
		body := make([]byte, contentLen)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, err
		}
		raw.Write(body)
	}
	return raw.Bytes(), nil
}

func (a *rtspAuth) parseChallenge(resp []byte) {
	s := string(resp)
	if m := reBetween(s, `realm="`, `"`); m != "" {
		a.realm = m
	}
	if m := reBetween(s, `nonce="`, `"`); m != "" {
		a.nonce = m
	}
	if m := reBetween(s, `qop="`, `"`); m != "" {
		a.qop = m
	}
}

func (a *rtspAuth) authHeader(method, uri string) string {
	ha1 := md5hex(a.user + ":" + a.realm + ":" + a.pass)
	ha2 := md5hex(method + ":" + uri)
	if a.qop != "" {
		const nc, cnonce = "00000001", "0a0a0a0a"
		resp := md5hex(strings.Join([]string{ha1, a.nonce, nc, cnonce, a.qop, ha2}, ":"))
		return fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", qop=%s, nc=%s, cnonce="%s"`,
			a.user, a.realm, a.nonce, uri, resp, a.qop, nc, cnonce)
	}
	resp := md5hex(ha1 + ":" + a.nonce + ":" + ha2)
	return fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`, a.user, a.realm, a.nonce, uri, resp)
}

// removed: reBetween below

func md5hex(s string) string {
	sum := md5.Sum([]byte(s)) // #nosec G401 -- RTSP digest auth mandates MD5
	return hex.EncodeToString(sum[:])
}

// reBetween returns the substring between the first occurrence of pre and the
// next occurrence of post after it, or "".
func reBetween(s, pre, post string) string {
	i := strings.Index(s, pre)
	if i < 0 {
		return ""
	}
	i += len(pre)
	j := strings.Index(s[i:], post)
	if j < 0 {
		return ""
	}
	return s[i : i+j]
}
