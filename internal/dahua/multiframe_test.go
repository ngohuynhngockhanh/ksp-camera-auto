package dahua

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// buildJSONFrame builds one DVRIP JSON (\xf6) frame: 32-byte header + chunk.
func buildJSONFrame(chunk []byte, total uint32) []byte {
	h := make([]byte, headerLen)
	binary.BigEndian.PutUint32(h[0:4], 0xf6000000)
	binary.LittleEndian.PutUint32(h[4:8], uint32(len(chunk)))
	binary.LittleEndian.PutUint32(h[16:20], total)
	return append(h, chunk...)
}

func readWire(t *testing.T, wire []byte) []byte {
	t.Helper()
	cli, srv := net.Pipe()
	c := &Client{conn: cli, timeout: 5 * time.Second}
	go func() { srv.Write(wire); srv.Close() }()
	_, payload, err := c.readFrame()
	cli.Close()
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	return payload
}

func TestReadFrameMultiFrame(t *testing.T) {
	// 5000-byte payload split across 3 fragmented frames (the NVR bug case).
	full := make([]byte, 5000)
	for i := range full {
		full[i] = byte('A' + i%26)
	}
	var wire bytes.Buffer
	wire.Write(buildJSONFrame(full[0:2000], 5000))
	wire.Write(buildJSONFrame(full[2000:4500], 5000))
	wire.Write(buildJSONFrame(full[4500:5000], 5000))

	got := readWire(t, wire.Bytes())
	if !bytes.Equal(got, full) {
		t.Fatalf("reassembled %d bytes, want %d", len(got), len(full))
	}
}

func TestReadFrameSingleFrame(t *testing.T) {
	// chunk == total: must NOT wait for more frames.
	full := []byte(`{"result":true,"params":{"table":[]}}`)
	got := readWire(t, buildJSONFrame(full, uint32(len(full))))
	if !bytes.Equal(got, full) {
		t.Fatalf("single frame: got %q", got)
	}
}

func TestReadFrameLoginFrameIgnoresTotal(t *testing.T) {
	// A \xb0 login frame: header[16:20] is NOT a length (garbage on real NVRs).
	// readFrame must read exactly header[4:8] bytes, not treat [16:20] as total.
	payload := []byte("Realm:Login to X\r\nRandom:Y\r\n\r\n")
	h := make([]byte, headerLen)
	h[0] = 0xb0 // login marker
	binary.LittleEndian.PutUint32(h[4:8], uint32(len(payload)))
	binary.LittleEndian.PutUint32(h[16:20], 0xFFFFFFFF) // garbage "total"
	got := readWire(t, append(h, payload...))
	if !bytes.Equal(got, payload) {
		t.Fatalf("login frame: got %q, want %q", got, payload)
	}
}
