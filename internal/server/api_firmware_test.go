package server

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func makeTestDHFirmwarePkg(t *testing.T, hwid string, touchBootloader bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name string, b []byte) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(b)
	}
	hwidJSON, _ := json.Marshal(map[string]any{
		"Vendor":                  "Dahua",
		"Build":                   "2024-04-02",
		"SecurityBaselineVersion": "V2.4",
		"HardwareID":              []string{hwid},
	})
	add("hwid", hwidJSON)
	cmds := []string{"burn kernel.img kernel", "burn romfs-x.squashfs.img rootfs"}
	if touchBootloader {
		cmds = append(cmds, "burn dhboot.bin.img bootloader")
	}
	instJSON, _ := json.Marshal(map[string]any{"Commands": cmds})
	add("Install", append(instJSON, []byte("\n//Product-version: 2.860.0000000.3.R\n")...))
	add("kernel.img", bytes.Repeat([]byte{0x11}, 1024))
	_ = zw.Close()

	raw := append([]byte(nil), buf.Bytes()...)
	raw[0] = 'D'
	raw[1] = 'H'
	return raw
}

func startTestFirmwareDVRIP(t *testing.T, hwid, sn, swVer string) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	addrHost, addrPort, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(addrPort)

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				hdr := make([]byte, 32)
				const sess = uint32(0x99887766)
				for {
					if _, err := io.ReadFull(conn, hdr); err != nil {
						return
					}
					magic := binary.BigEndian.Uint32(hdr[0:4])
					clen := binary.LittleEndian.Uint32(hdr[4:8])
					payload := make([]byte, clen)
					if clen > 0 {
						if _, err := io.ReadFull(conn, payload); err != nil {
							return
						}
					}
					switch magic {
					case 0xa0010000:
						body := []byte("Realm:Login to MOCK\r\nRandom:87654321\r\n\r\n")
						rh := make([]byte, 32)
						binary.BigEndian.PutUint32(rh[0:4], 0xb0000000)
						binary.LittleEndian.PutUint32(rh[4:8], uint32(len(body)))
						_, _ = conn.Write(append(rh, body...))
					case 0xa0050000:
						rh := make([]byte, 32)
						binary.BigEndian.PutUint32(rh[0:4], 0xb0000000)
						rh[9] = 0x08
						binary.LittleEndian.PutUint32(rh[16:20], sess)
						_, _ = conn.Write(rh)
					case 0xf6000000:
						jlen := binary.LittleEndian.Uint32(hdr[16:20])
						jpart := payload
						if jlen > 0 && int(jlen) <= len(payload) {
							jpart = payload[:jlen]
						}
						for len(jpart) > 0 && jpart[len(jpart)-1] == 0 {
							jpart = jpart[:len(jpart)-1]
						}
						var req struct {
							ID     int    `json:"id"`
							Method string `json:"method"`
						}
						_ = json.Unmarshal(jpart, &req)
						var resp map[string]any
						switch req.Method {
						case "upgrader.factory.instance":
							resp = map[string]any{"id": req.ID, "session": sess, "result": 2}
						case "upgrader.getUpdateSerial":
							resp = map[string]any{
								"id": req.ID, "session": sess, "result": true,
								"params": map[string]any{
									"list": []map[string]any{{"serial": hwid, "sn": sn, "SWVersion": swVer}},
								},
							}
						case "magicBox.getSoftwareVersion":
							resp = map[string]any{
								"id": req.ID, "session": sess, "result": true,
								"params": map[string]any{"version": map[string]any{"Version": swVer, "BuildDate": "2026-05-30"}},
							}
						case "upgrader.getState":
							resp = map[string]any{
								"id": req.ID, "session": sess, "result": true,
								"params": map[string]any{"File": "", "Progress": 0, "State": "Downloading"},
							}
						default:
							resp = map[string]any{"id": req.ID, "session": sess, "result": true}
						}
						b, _ := json.Marshal(resp)
						rh := make([]byte, 32)
						binary.BigEndian.PutUint32(rh[0:4], 0xf6000000)
						binary.LittleEndian.PutUint32(rh[4:8], uint32(len(b)))
						binary.LittleEndian.PutUint32(rh[8:12], uint32(req.ID))
						binary.LittleEndian.PutUint32(rh[16:20], uint32(len(b)))
						binary.LittleEndian.PutUint32(rh[24:28], sess)
						_, _ = conn.Write(append(rh, b...))
					}
				}
			}(c)
		}
	}()
	return addrHost, p
}

func TestFirmwareHTTPHandlers_InspectPreflightAndDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	invFile := filepath.Join(tmpDir, "cameras.yaml")
	_ = os.WriteFile(invFile, []byte("[]"), 0644)
	inv, err := config.LoadInventory(invFile)
	if err != nil {
		t.Fatal(err)
	}

	targetHWID := "IPC-C5AP:01:02:00:100:68:00:01:10:01:01:04:4B0:00:02:00:00:00:01:00:00:40"
	host, port := startTestFirmwareDVRIP(t, targetHWID, "AD0786FPAG01146", "2.860.0000000.26.R")

	_ = inv.Upsert(config.Device{
		ID:       "camera05",
		Name:     "Bàn 05",
		Host:     host,
		Port:     port,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "secret",
	})

	cfg := config.Default()
	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	adminToken := srv.session.create("admin")

	// 1. Inspect firmware package
	pkgBytes := makeTestDHFirmwarePkg(t, targetHWID, true)
	inspectBody, _ := json.Marshal(map[string]any{
		"fileName":   "DH_IPC-HX3XXX_20240402.bin",
		"dataBase64": base64.StdEncoding.EncodeToString(pkgBytes),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/firmware/inspect", bytes.NewReader(inspectBody))
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: adminToken})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/firmware/inspect got %d: %s", rec.Code, rec.Body.String())
	}
	var inspectRes stagedFirmware
	if err := json.Unmarshal(rec.Body.Bytes(), &inspectRes); err != nil {
		t.Fatal(err)
	}
	if inspectRes.ID == "" || !inspectRes.Info.TouchesBootloader {
		t.Fatalf("unexpected inspectRes: %+v", inspectRes)
	}

	// 2. Preflight check against camera05
	preBody, _ := json.Marshal(map[string]any{
		"fwId":      inspectRes.ID,
		"deviceIds": []string{"camera05"},
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/firmware/preflight", bytes.NewReader(preBody))
	req2.AddCookie(&http.Cookie{Name: sessionCookie, Value: adminToken})
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("POST /api/firmware/preflight got %d: %s", rec2.Code, rec2.Body.String())
	}
	var preRes struct {
		Devices []FirmwarePreflightRow `json:"devices"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &preRes)
	if len(preRes.Devices) != 1 || !preRes.Devices[0].Compatible || preRes.Devices[0].SN != "AD0786FPAG01146" {
		t.Fatalf("unexpected preflight rows: %+v", preRes.Devices)
	}

	// 3. Apply without confirmBootloaderWrite when dryRun=false should be blocked with 400
	badApply, _ := json.Marshal(map[string]any{
		"fwId":                   inspectRes.ID,
		"deviceIds":              []string{"camera05"},
		"dryRun":                 false,
		"confirmBootloaderWrite": false,
	})
	req3 := httptest.NewRequest(http.MethodPost, "/api/firmware/apply", bytes.NewReader(badApply))
	req3.AddCookie(&http.Cookie{Name: sessionCookie, Value: adminToken})
	rec3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when confirmBootloaderWrite=false on bootloader package, got %d", rec3.Code)
	}

	// 4. Dry-Run SSE stream succeeds
	dryApply, _ := json.Marshal(map[string]any{
		"fwId":      inspectRes.ID,
		"deviceIds": []string{"camera05"},
		"dryRun":    true,
	})
	req4 := httptest.NewRequest(http.MethodPost, "/api/firmware/apply", bytes.NewReader(dryApply))
	req4.AddCookie(&http.Cookie{Name: sessionCookie, Value: adminToken})
	rec4 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("POST /api/firmware/apply dryRun got %d: %s", rec4.Code, rec4.Body.String())
	}
	sseOut := rec4.Body.String()
	if !strings.Contains(sseOut, `"phase":"dry_run_done"`) || !strings.Contains(sseOut, `"type":"done"`) {
		t.Fatalf("SSE output missing expected dry_run_done / done events: %s", sseOut)
	}
}
