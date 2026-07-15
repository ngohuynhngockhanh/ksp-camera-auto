package dahua

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// TestDialUnreachableWrapsErrDialUnreachable confirms a TCP-level connect
// failure (nothing listening) is classified as ErrDialUnreachable — the
// signal camera.Open's KBVision port-fallback (37777 -> 8888) relies on to
// decide whether retrying on another port is worth it.
func TestDialUnreachableWrapsErrDialUnreachable(t *testing.T) {
	// Port 1 requires root to bind almost everywhere and nothing listens on
	// it in a test environment, so this fails fast (connection refused)
	// without needing any live device or fake server.
	_, err := Dial("127.0.0.1:1", "admin", "wrong", 2*time.Second)
	if err == nil {
		t.Fatal("expected a dial error")
	}
	if !errors.Is(err, ErrDialUnreachable) {
		t.Errorf("expected ErrDialUnreachable, got: %v", err)
	}
}

// startFakeLoginFailServer accepts exactly one DVRIP connection, replies to
// the realm request, then replies to the login request with an
// authentication-failed error code (header[8:12] = 0x01,0x00 per
// dvripErrString) — reproducing a real device rejecting bad credentials,
// with no network dependency beyond loopback.
func startFakeLoginFailServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Realm request: read the 32-byte header, ignore contents.
		hdr := make([]byte, headerLen)
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return
		}
		body := []byte("Realm:Login to TESTREALM\r\nRandom:1234\r\n\r\n")
		resp := make([]byte, headerLen)
		binary.LittleEndian.PutUint32(resp[4:8], uint32(len(body)))
		if _, err := conn.Write(append(resp, body...)); err != nil {
			return
		}

		// Login request: read header + its payload (chunk length).
		hdr2 := make([]byte, headerLen)
		if _, err := io.ReadFull(conn, hdr2); err != nil {
			return
		}
		chunk := binary.LittleEndian.Uint32(hdr2[4:8])
		if chunk > 0 {
			if _, err := io.CopyN(io.Discard, conn, int64(chunk)); err != nil {
				return
			}
		}
		// Failing login response: errCode at [8:12] = auth failed.
		loginResp := make([]byte, headerLen)
		loginResp[8], loginResp[9] = 0x01, 0x00
		_, _ = conn.Write(loginResp)
	}()
	return ln.Addr().String()
}

// TestDialLoginFailureNotWrappedAsUnreachable confirms a bad-credentials
// login failure (TCP connect succeeded) is NOT classified as
// ErrDialUnreachable — otherwise camera.Open's port-fallback would retry a
// second port on a real auth failure, masking the actual problem.
func TestDialLoginFailureNotWrappedAsUnreachable(t *testing.T) {
	addr := startFakeLoginFailServer(t)
	_, err := Dial(addr, "admin", "wrongpass", 3*time.Second)
	if err == nil {
		t.Fatal("expected a login error")
	}
	if errors.Is(err, ErrDialUnreachable) {
		t.Errorf("login failure must not be classified as unreachable: %v", err)
	}
}
