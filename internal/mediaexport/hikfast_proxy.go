package mediaexport

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reDigestRealm = regexp.MustCompile(`(?i)realm="([^"]+)"`)
	reDigestNonce = regexp.MustCompile(`(?i)nonce="([^"]+)"`)
	reContentLen  = regexp.MustCompile(`(?i)Content-Length:\s*(\d+)`)
)

func md5HexStr(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func buildRTSPDigestHeader(user, pass, method, uri, wwwAuth string) string {
	mRealm := reDigestRealm.FindStringSubmatch(wwwAuth)
	mNonce := reDigestNonce.FindStringSubmatch(wwwAuth)
	if len(mRealm) < 2 || len(mNonce) < 2 {
		return ""
	}
	realm := mRealm[1]
	nonce := mNonce[1]
	ha1 := md5HexStr(fmt.Sprintf("%s:%s:%s", user, realm, pass))
	ha2 := md5HexStr(fmt.Sprintf("%s:%s", method, uri))
	resp := md5HexStr(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))
	return fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		user, realm, nonce, uri, resp)
}

// startHikFastRTSPProxy starts an in-process loopback RTSP proxy that injects
// "Rate-Control: no" and "Scale: 8.0" on PLAY so Hikvision NVR/IPC RTSP
// playback streams at ~8x real-time with 100% frame preservation.
func startHikFastRTSPProxy(ctx context.Context, rawRTSPURL string) (proxyURL string, cleanup func(), ok bool) {
	if !strings.Contains(rawRTSPURL, "/Streaming/tracks/") {
		return "", nil, false
	}
	u, err := url.Parse(rawRTSPURL)
	if err != nil || u.User == nil || u.Host == "" {
		return "", nil, false
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	hostPort := u.Host
	if !strings.Contains(hostPort, ":") {
		hostPort += ":554"
	}
	realURI := fmt.Sprintf("rtsp://%s%s", hostPort, u.RequestURI())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, false
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer ln.Close()
		cli, err := ln.Accept()
		if err != nil {
			return
		}
		defer cli.Close()
		runHikFastRTSPSession(ctx, cli, hostPort, user, pass, realURI)
	}()

	pURL := fmt.Sprintf("rtsp://%s%s", ln.Addr().String(), u.Path)
	cleanFn := func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
	}
	return pURL, cleanFn, true
}

func readRTSPHeaderBlock(r io.Reader) ([]byte, []byte, error) {
	var buf bytes.Buffer
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			if idx := bytes.Index(buf.Bytes(), []byte("\r\n\r\n")); idx >= 0 {
				all := buf.Bytes()
				hdr := append([]byte(nil), all[:idx]...)
				rest := append([]byte(nil), all[idx+4:]...)
				return hdr, rest, nil
			}
		}
		if err != nil {
			return nil, nil, err
		}
		if buf.Len() > 65536 {
			return nil, nil, fmt.Errorf("rtsp header too large")
		}
	}
}

func readRTSPResponse(r io.Reader) ([]byte, []byte, error) {
	hdr, rest, err := readRTSPHeaderBlock(r)
	if err != nil {
		return nil, nil, err
	}
	if m := reContentLen.FindSubmatch(hdr); len(m) >= 2 {
		if cl, err := strconv.Atoi(string(m[1])); err == nil && cl > len(rest) {
			need := cl - len(rest)
			extra := make([]byte, need)
			if _, err := io.ReadFull(r, extra); err != nil {
				return hdr, rest, err
			}
			rest = append(rest, extra...)
		}
	}
	return hdr, rest, nil
}

func runHikFastRTSPSession(ctx context.Context, cli net.Conn, hostPort, user, pass, realURI string) {
	cam, err := net.DialTimeout("tcp", hostPort, 10*time.Second)
	if err != nil {
		return
	}
	defer cam.Close()

	stopCh := make(chan struct{})
	defer close(stopCh)
	go func() {
		select {
		case <-ctx.Done():
			_ = cli.Close()
			_ = cam.Close()
		case <-stopCh:
		}
	}()

	var wwwAuth string
	for {
		reqHdr, _, err := readRTSPHeaderBlock(cli)
		if err != nil {
			return
		}
		lines := strings.Split(string(reqHdr), "\r\n")
		if len(lines) == 0 {
			return
		}
		parts := strings.SplitN(lines[0], " ", 3)
		if len(parts) < 2 {
			return
		}
		method, reqURI := parts[0], parts[1]
		targetURI := realURI
		if idx := strings.Index(reqURI, "/trackID="); idx >= 0 {
			targetURI = realURI + reqURI[idx:]
		}

		buildReq := func(authHeader string) []byte {
			out := make([]string, 0, len(lines)+4)
			out = append(out, fmt.Sprintf("%s %s RTSP/1.0", method, targetURI))
			for _, l := range lines[1:] {
				if l == "" || strings.HasPrefix(strings.ToLower(l), "authorization:") {
					continue
				}
				out = append(out, l)
			}
			if authHeader != "" {
				out = append(out, "Authorization: "+authHeader)
			}
			if method == "PLAY" {
				out = append(out, "Rate-Control: no", "Scale: 8.0")
			}
			return []byte(strings.Join(out, "\r\n") + "\r\n\r\n")
		}

		var authHdr string
		if wwwAuth != "" {
			authHdr = buildRTSPDigestHeader(user, pass, method, realURI, wwwAuth)
		}
		if _, err := cam.Write(buildReq(authHdr)); err != nil {
			return
		}

		respHdr, respBody, err := readRTSPResponse(cam)
		if err != nil {
			return
		}
		respTxt := string(respHdr)
		if strings.Contains(respTxt, "401 Unauthorized") && wwwAuth == "" {
			for _, rl := range strings.Split(respTxt, "\r\n") {
				if strings.HasPrefix(strings.ToLower(rl), "www-authenticate:") && strings.Contains(strings.ToLower(rl), "digest") {
					wwwAuth = strings.TrimSpace(rl[len("www-authenticate:"):])
					break
				}
			}
			if wwwAuth != "" {
				authHdr = buildRTSPDigestHeader(user, pass, method, realURI, wwwAuth)
				if _, err := cam.Write(buildReq(authHdr)); err != nil {
					return
				}
				respHdr, respBody, err = readRTSPResponse(cam)
				if err != nil {
					return
				}
			}
		}

		rawResp := append(append(respHdr, []byte("\r\n\r\n")...), respBody...)
		if _, err := cli.Write(rawResp); err != nil {
			return
		}
		if method == "PLAY" {
			break
		}
	}

	_, _ = io.Copy(cli, cam)
}
