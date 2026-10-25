package dahua

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func buildMockDHPackage(t *testing.T, hwids []string, commands []string, restoreDefault bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	writeMember := func(name string, data []byte) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip member %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write zip member %s: %v", name, err)
		}
	}

	hwidDoc := map[string]any{
		"Vendor":                  "Dahua",
		"Build":                   "2024-04-02",
		"BuildDateTime":           "2024-04-02 11:47:09",
		"SecurityBaselineVersion": "V2.4",
		"HardwareID":              hwids,
		"Devices":                 [][]string{{"IPC-HX3XXX", "1.00"}},
	}
	hwidBytes, _ := json.Marshal(hwidDoc)
	writeMember("hwid", hwidBytes)

	installDoc := map[string]any{
		"Commands": commands,
		"Devices":  [][]string{{"IPC-HX3XXX", "1.00"}},
		"Vendor":   "General",
	}
	installJSON, _ := json.Marshal(installDoc)
	var installScript bytes.Buffer
	installScript.Write(installJSON)
	installScript.WriteString("\n//Product-version: 2.860.0000000.3.R\n")
	if restoreDefault {
		installScript.WriteString("//IPC_RestoreDefault\n")
	}
	writeMember("Install", installScript.Bytes())
	writeMember("kernel.img", bytes.Repeat([]byte{0xAA}, 4096))
	writeMember("romfs-x.squashfs.img", bytes.Repeat([]byte{0xBB}, 40000))
	writeMember("dhboot.bin.img", bytes.Repeat([]byte{0xCC}, 2048))
	writeMember("sign.img", bytes.Repeat([]byte{0xDD}, 256))

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	raw := buf.Bytes()
	out := make([]byte, len(raw))
	copy(out, raw)
	// Replace PK\x03\x04 with DH\x03\x04 like real Dahua firmware packages
	out[0] = 'D'
	out[1] = 'H'
	return out
}

func TestInspectFirmwarePackage(t *testing.T) {
	targetHWID := "IPC-C5AP:01:02:00:100:68:00:01:10:01:01:04:4B0:00:02:00:00:00:01:00:00:40"
	pkg := buildMockDHPackage(
		t,
		[]string{targetHWID, "IPC-H5AE:01:02:00:100:68:00:01:10:01:01:04:4B0:00:02:00:00:00:01:00:00:40"},
		[]string{"burn kernel.img kernel", "burn romfs-x.squashfs.img rootfs", "burn dhboot.bin.img bootloader"},
		true,
	)

	info, err := InspectFirmwarePackage(pkg)
	if err != nil {
		t.Fatalf("InspectFirmwarePackage unexpected error: %v", err)
	}
	if info.MagicHeader != "DH" {
		t.Errorf("MagicHeader = %q, want DH", info.MagicHeader)
	}
	if !info.TouchesBootloader {
		t.Errorf("TouchesBootloader = false, want true")
	}
	if !info.RestoresDefault {
		t.Errorf("RestoresDefault = false, want true")
	}
	if !info.HasSignature || info.SignatureBytes != 256 {
		t.Errorf("HasSignature = %v (%d bytes), want true (256)", info.HasSignature, info.SignatureBytes)
	}
	if info.ProductVersion != "2.860.0000000.3.R" {
		t.Errorf("ProductVersion = %q, want 2.860.0000000.3.R", info.ProductVersion)
	}
	if info.SecurityBaselineVersion != "V2.4" {
		t.Errorf("SecurityBaselineVersion = %q, want V2.4", info.SecurityBaselineVersion)
	}
	if !info.SupportsHWID(targetHWID) {
		t.Errorf("SupportsHWID(%q) = false, want true", targetHWID)
	}
	if info.SupportsHWID("IPC-OTHER:00:00") {
		t.Errorf("SupportsHWID(IPC-OTHER) = true, want false")
	}

	// Corrupt a byte inside the compressed stream and verify CRC failure is caught.
	// Corrupt a byte inside the first member's data payload and verify CRC failure is caught.
	corrupted := make([]byte, len(pkg))
	copy(corrupted, pkg)
	corrupted[64] ^= 0xFF
	if _, err := InspectFirmwarePackage(corrupted); !errors.Is(err, ErrCorruptFirmwarePackage) {
		t.Errorf("InspectFirmwarePackage(corrupted) = %v, want ErrCorruptFirmwarePackage", err)
	}

	// Invalid magic
	if _, err := InspectFirmwarePackage([]byte("NOT_A_FIRMWARE")); !errors.Is(err, ErrInvalidFirmwareMagic) {
		t.Errorf("InspectFirmwarePackage(bad magic) = %v, want ErrInvalidFirmwareMagic", err)
	}
}

type mockUpgraderDVRIP struct {
	ln           net.Listener
	mu           sync.Mutex
	deviceHWID   string
	deviceSN     string
	swVersion    string
	rtspPort     int
	prepared     bool
	canceled     int
	executed     int
	receivedData bytes.Buffer
	stateCalls   int
}

func startMockUpgraderDVRIP(t *testing.T, hwid, sn, swVer string) *mockUpgraderDVRIP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	m := &mockUpgraderDVRIP{
		ln:         ln,
		deviceHWID: hwid,
		deviceSN:   sn,
		swVersion:  swVer,
		rtspPort:   8554,
	}
	t.Cleanup(func() { _ = ln.Close() })
	go m.serve()
	return m
}

func (m *mockUpgraderDVRIP) serve() {
	for {
		c, err := m.ln.Accept()
		if err != nil {
			return
		}
		go m.handle(c)
	}
}

func (m *mockUpgraderDVRIP) handle(c net.Conn) {
	defer c.Close()
	hdr := make([]byte, headerLen)
	const sessID = uint32(0x12345678)
	for {
		if _, err := io.ReadFull(c, hdr); err != nil {
			return
		}
		magic := binary.BigEndian.Uint32(hdr[0:4])
		chunkLen := binary.LittleEndian.Uint32(hdr[4:8])
		payload := make([]byte, chunkLen)
		if chunkLen > 0 {
			if _, err := io.ReadFull(c, payload); err != nil {
				return
			}
		}

		switch magic {
		case 0xa0010000:
			body := []byte("Realm:Login to MOCK\r\nRandom:12345678\r\n\r\n")
			rh := make([]byte, headerLen)
			binary.BigEndian.PutUint32(rh[0:4], 0xb0000000)
			binary.LittleEndian.PutUint32(rh[4:8], uint32(len(body)))
			_, _ = c.Write(append(rh, body...))
		case 0xa0050000:
			rh := make([]byte, headerLen)
			binary.BigEndian.PutUint32(rh[0:4], 0xb0000000)
			rh[9] = 0x08
			binary.LittleEndian.PutUint32(rh[16:20], sessID)
			_, _ = c.Write(rh)
		case 0xf6000000:
			jsonLen := binary.LittleEndian.Uint32(hdr[16:20])
			binLen := binary.LittleEndian.Uint32(hdr[20:24])
			jsonPart := payload
			var binPart []byte
			if binLen > 0 && int(jsonLen+binLen) <= len(payload) {
				jsonPart = payload[:jsonLen]
				binPart = payload[jsonLen : jsonLen+binLen]
			}
			var req struct {
				ID     int             `json:"id"`
				Method string          `json:"method"`
				Object int64           `json:"object"`
				Params json.RawMessage `json:"params"`
			}
			_ = json.Unmarshal(bytesTrimNull(jsonPart), &req)

			m.mu.Lock()
			var resp map[string]any
			switch req.Method {
			case "upgrader.factory.instance":
				resp = map[string]any{"id": req.ID, "session": sessID, "result": 2}
			case "upgrader.getUpdateSerial":
				resp = map[string]any{
					"id": req.ID, "session": sessID, "result": true,
					"params": map[string]any{
						"list": []map[string]any{{
							"Vendor":    "DH",
							"serial":    m.deviceHWID,
							"sn":        m.deviceSN,
							"SWVersion": m.swVersion,
						}},
					},
				}
			case "magicBox.getSoftwareVersion":
				resp = map[string]any{
					"id": req.ID, "session": sessID, "result": true,
					"params": map[string]any{
						"version": map[string]any{
							"Version":                 m.swVersion,
							"BuildDate":               "2026-05-30",
							"SecurityBaseLineVersion": "V2.4",
						},
					},
				}
			case "upgrader.getState":
				if m.executed > 0 {
					m.stateCalls++
					prog := m.stateCalls * 50
					file := "kernel.img"
					if prog >= 100 {
						prog = 100
						file = ""
					}
					resp = map[string]any{
						"id": req.ID, "session": sessID, "result": true,
						"params": map[string]any{"File": file, "Progress": prog, "State": "Upgrading"},
					}
				} else {
					resp = map[string]any{
						"id": req.ID, "session": sessID, "result": true,
						"params": map[string]any{"File": "", "Progress": 0, "State": "Downloading"},
					}
				}
			case "upgrader.prepare":
				if m.prepared {
					resp = map[string]any{"id": req.ID, "session": sessID, "result": false}
				} else {
					m.prepared = true
					m.receivedData.Reset()
					resp = map[string]any{"id": req.ID, "session": sessID, "result": true}
				}
			case "upgrader.cancel":
				m.prepared = false
				m.canceled++
				resp = map[string]any{"id": req.ID, "session": sessID, "result": true}
			case "upgrader.appendData":
				// Verify JSON part ended with '\n\x00' as required by sonia F6Services_Req
				if len(jsonPart) < 2 || jsonPart[len(jsonPart)-2] != '\n' || jsonPart[len(jsonPart)-1] != 0 {
					resp = map[string]any{"id": req.ID, "session": sessID, "result": false}
				} else {
					m.receivedData.Write(binPart)
					resp = map[string]any{"id": req.ID, "session": sessID, "result": true}
				}
			case "upgrader.execute":
				m.executed++
				m.swVersion = "2.860.0000000.3.R"
				resp = map[string]any{"id": req.ID, "session": sessID, "result": true}
			case "configManager.getConfig":
				var p struct {
					Name string `json:"name"`
				}
				_ = json.Unmarshal(req.Params, &p)
				if p.Name == "RTSP" {
					resp = map[string]any{
						"id": req.ID, "session": sessID, "result": true,
						"params": map[string]any{"table": map[string]any{"Enable": true, "Port": m.rtspPort}},
					}
				} else {
					resp = map[string]any{
						"id": req.ID, "session": sessID, "result": true,
						"params": map[string]any{"table": map[string]any{}},
					}
				}
			case "configManager.setConfig":
				var p struct {
					Name  string         `json:"name"`
					Table map[string]any `json:"table"`
				}
				_ = json.Unmarshal(req.Params, &p)
				if p.Name == "RTSP" {
					if v, ok := p.Table["Port"].(float64); ok {
						m.rtspPort = int(v)
					}
				}
				resp = map[string]any{"id": req.ID, "session": sessID, "result": true}
			default:
				resp = map[string]any{"id": req.ID, "session": sessID, "result": true}
			}
			m.mu.Unlock()

			outBytes, _ := json.Marshal(resp)
			rh := make([]byte, headerLen)
			binary.BigEndian.PutUint32(rh[0:4], 0xf6000000)
			binary.LittleEndian.PutUint32(rh[4:8], uint32(len(outBytes)))
			binary.LittleEndian.PutUint32(rh[8:12], uint32(req.ID))
			binary.LittleEndian.PutUint32(rh[16:20], uint32(len(outBytes)))
			binary.LittleEndian.PutUint32(rh[24:28], sessID)
			_, _ = c.Write(append(rh, outBytes...))
		}
	}
}

func TestRunFirmwareSessionDryRunAndExecute(t *testing.T) {
	targetHWID := "IPC-C5AP:01:02:00:100:68:00:01:10:01:01:04:4B0:00:02:00:00:00:01:00:00:40"
	pkg := buildMockDHPackage(
		t,
		[]string{targetHWID},
		[]string{"burn kernel.img kernel", "burn romfs-x.squashfs.img rootfs", "burn dhboot.bin.img bootloader"},
		true,
	)
	pkgInfo, err := InspectFirmwarePackage(pkg)
	if err != nil {
		t.Fatalf("InspectFirmwarePackage: %v", err)
	}

	srv := startMockUpgraderDVRIP(t, targetHWID, "AD0786FPAG01146", "2.860.0000000.26.R")

	// 1. Test HWID Mismatch rejection
	mismatchPkg := buildMockDHPackage(t, []string{"IPC-WRONG:01:02"}, []string{"burn kernel.img kernel"}, false)
	c1, err := Dial(srv.ln.Addr().String(), "admin", "pass123", 5*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if _, err := c1.RunFirmwareSession(context.Background(), mismatchPkg, nil, true, nil); !errors.Is(err, ErrHWIDMismatch) {
		t.Fatalf("expected ErrHWIDMismatch, got %v", err)
	}
	_ = c1.Close()

	// 2. Test Dry-Run (transfers 100% of bytes in chunks, then cancels without calling execute)
	c2, err := Dial(srv.ln.Addr().String(), "admin", "pass123", 5*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	var phases []string
	ident, err := c2.RunFirmwareSession(context.Background(), pkg, pkgInfo, true, func(ev UpgradePhaseEvent) {
		phases = append(phases, ev.Phase)
	})
	_ = c2.Close()
	if err != nil {
		t.Fatalf("DryRun failed: %v", err)
	}
	if ident.UpdateSerial.SN != "AD0786FPAG01146" {
		t.Errorf("SN = %q, want AD0786FPAG01146", ident.UpdateSerial.SN)
	}
	srv.mu.Lock()
	gotBytes := srv.receivedData.Bytes()
	canceled := srv.canceled
	executed := srv.executed
	srv.mu.Unlock()
	if !bytes.Equal(gotBytes, pkg) {
		t.Fatalf("DryRun transferred %d bytes, want exact match of %d bytes", len(gotBytes), len(pkg))
	}
	if canceled != 1 || executed != 0 {
		t.Errorf("DryRun canceled=%d executed=%d, want 1 and 0", canceled, executed)
	}

	// 3. Test Real Execute + RTSP 554 restoration
	c3, err := Dial(srv.ln.Addr().String(), "admin", "pass123", 5*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	_, err = c3.RunFirmwareSession(context.Background(), pkg, pkgInfo, false, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	changed, err := c3.EnsureRTSPPort554()
	if err != nil || !changed {
		t.Fatalf("EnsureRTSPPort554 changed=%v err=%v, want true, nil", changed, err)
	}
	_ = c3.Close()

	srv.mu.Lock()
	if srv.executed != 1 {
		t.Errorf("executed = %d, want 1", srv.executed)
	}
	if srv.rtspPort != 554 {
		t.Errorf("rtspPort = %d, want 554", srv.rtspPort)
	}
	srv.mu.Unlock()
}
