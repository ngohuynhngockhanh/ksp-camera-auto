package tvt

import (
	"context"
	"crypto/aes"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func encryptECBZeroPadForTest(plain, keyStr string) string {
	block, err := aes.NewCipher([]byte(keyStr))
	if err != nil {
		panic(err)
	}
	raw := []byte(plain)
	if rem := len(raw) % aes.BlockSize; rem != 0 {
		raw = append(raw, make([]byte, aes.BlockSize-rem)...)
	}
	out := make([]byte, len(raw))
	for off := 0; off < len(raw); off += aes.BlockSize {
		block.Encrypt(out[off:off+aes.BlockSize], raw[off:off+aes.BlockSize])
	}
	return base64.StdEncoding.EncodeToString(out)
}

func hostPortFromServer(t *testing.T, srv *httptest.Server) (string, int) {
	t.Helper()
	h, pStr, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("split hostport %s: %v", srv.URL, err)
	}
	p, _ := strconv.Atoi(pStr)
	return h, p
}

func TestRTSPHelpersAndCrypto(t *testing.T) {
	if got := BuildRTSPLivePath(2, 0); got != "/chID=2&streamType=main&linkType=tcp" {
		t.Fatalf("unexpected live main path: %s", got)
	}
	if got := BuildRTSPLivePath(15, 1); got != "/chID=15&streamType=sub&linkType=tcp" {
		t.Fatalf("unexpected live sub path: %s", got)
	}
	if !IsTVTRTSPPath("rtsp://rtsp:mockpass@192.168.1.253:554/chID=14&streamType=main&linkType=tcp") {
		t.Fatalf("expected IsTVTRTSPPath=true")
	}
	ch, ok := ParseChannelFromRTSP("rtsp://rtsp:mockpass@192.168.1.253:554/chID=14&streamType=main&linkType=tcp", "")
	if !ok || ch != 14 {
		t.Fatalf("expected ch=14 ok=true, got ch=%d ok=%v", ch, ok)
	}

	aedt := time.FixedZone("AEDT", 11*3600)
	st := time.Date(2026, 11, 11, 12, 50, 20, 0, time.UTC) // 23:50:20 AEDT
	et := st.Add(60 * time.Second)
	u := BuildRTSPPlaybackURL("192.168.1.253", 554, "rtsp", "mockpass", 1, st, et, aedt)
	wantSub := "192.168.1.253:554/chID=2&date=2026-11-11&time=23:50:20&timelen=65&streamType=main&linkType=tcp"
	if !strings.Contains(u, wantSub) {
		t.Fatalf("unexpected playback URL %q (want substring %q)", u, wantSub)
	}

	_, md5Hex := ComputePasswordDigest("mockpass", "nonce99")
	ct := encryptECBZeroPadForTest("sess-secret-key", md5Hex)
	if dec := DecryptECBZeroPad(ct, md5Hex); dec != "sess-secret-key" {
		t.Fatalf("DecryptECBZeroPad=%q, want sess-secret-key", dec)
	}
}

func TestNVMS9000XMLPlaneAndAutoRelogin(t *testing.T) {
	const (
		wantUser  = "admin"
		wantPass  = "mockpass"
		wantNonce = "9876543210"
	)
	wantDigest, md5Hex := ComputePasswordDigest(wantPass, wantNonce)
	encSessKey := encryptECBZeroPadForTest("tvt-session-key", md5Hex)

	var expiredOnce atomic.Bool
	var curEnct atomic.Value
	curEnct.Store("H264")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		txt := string(body)
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/reqLogin":
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>success</status><content><nonce>` + wantNonce + `</nonce><sessionId>{ABCD-1234}</sessionId><token>tok-1</token></content></response>`))
		case "/doLogin":
			if !strings.Contains(txt, wantDigest) || !strings.Contains(r.Header.Get("Cookie"), "sessionId=ABCD-1234") {
				w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>fail</status><errorCode>536870948</errorCode></response>`))
				return
			}
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>success</status><content><sessionKey>` + encSessKey + `</sessionKey><securityVer>1</securityVer></content></response>`))
		case "/queryBasicCfg":
			if !expiredOnce.Swap(true) {
				w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>fail</status><errorCode>536871011</errorCode></response>`))
				return
			}
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>success</status><content><name>Celestial-NVR</name><productModel>NVR-0816B2-16P</productModel><sn>NF020086R54E</sn></content></response>`))
		case "/queryNodeEncodeInfo":
			enct := curEnct.Load().(string)
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>success</status><content><item id="2"><name>Table 1</name><main enct="` + enct + `" aGOP="60"/><an res="2880x1620" fps="30" QoI="4096" audio="true" bitType="VBR"/></item></content></response>`))
		case "/queryNetworkNodeEncodeInfo":
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>success</status><content><item id="2"><name>Table 1</name><sub res="704x480" fps="15" QoI="512" bitType="CBR" enct="H265" GOP="30"/></item></content></response>`))
		case "/editNodeEncodeInfo":
			if strings.Contains(txt, `enct="H265"`) {
				curEnct.Store("H265")
			}
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>success</status></response>`))
		case "/doLogout":
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>success</status></response>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	pre, err := ProbePreLogin(ctx, srv.URL, 2*time.Second)
	if err != nil || pre.SessionID != "ABCD-1234" {
		t.Fatalf("ProbePreLogin failed: %+v, err=%v", pre, err)
	}

	host, port := hostPortFromServer(t, srv)
	cl := New(host, port, wantUser, wantPass, 3*time.Second)
	defer cl.Close()

	sn, err := cl.GetSerialNumber(ctx)
	if err != nil || sn != "NF020086R54E" {
		t.Fatalf("GetSerialNumber=%q err=%v", sn, err)
	}

	if err := cl.SetStreamConfig(ctx, 1, 0, func(si *StreamInfo) { si.Compression = "H.265" }); err != nil {
		t.Fatalf("SetStreamConfig failed: %v", err)
	}
	si, err := cl.GetStreamInfo(ctx, 1, 0)
	if err != nil || si.Compression != "H.265" || si.Width != 2880 || si.Height != 1620 {
		t.Fatalf("unexpected StreamInfo after edit: %+v, err=%v", si, err)
	}
}

func TestONVIFProfileSFallbackAndProfileGRecordings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		txt := string(body)
		switch r.URL.Path {
		case "/reqLogin":
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>success</status><content><nonce>111</nonce><sessionId>{SID-ONVIF}</sessionId><token>t</token></content></response>`))
		case "/doLogin":
			// Restricted "rtsp" user fails web XML login with 536870948, triggering ONVIF fallback.
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><response version="1.0" systemType="NVMS-9000"><status>fail</status><errorCode>536870948</errorCode></response>`))
		case "/onvif/device_service":
			if !strings.Contains(txt, "<Username>rtsp</Username>") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if strings.Contains(txt, "GetSystemDateAndTime") {
				w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetSystemDateAndTimeResponse><SystemDateAndTime><UTCDateTime><Time><Hour>13</Hour><Minute>0</Minute><Second>0</Second></Time><Date><Year>2026</Year><Month>11</Month><Day>11</Day></Date></UTCDateTime><LocalDateTime><Time><Hour>0</Hour><Minute>0</Minute><Second>0</Second></Time><Date><Year>2026</Year><Month>11</Month><Day>12</Day></Date></LocalDateTime></SystemDateAndTime></GetSystemDateAndTimeResponse></s:Body></s:Envelope>`))
				return
			}
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetDeviceInformationResponse><Manufacturer>--</Manufacturer><Model>NVR-0816B2-16P</Model><FirmwareVersion>1.4.8</FirmwareVersion><SerialNumber>NF020086R54E</SerialNumber><HardwareId>200006-V0</HardwareId></GetDeviceInformationResponse></s:Body></s:Envelope>`))
		case "/onvif/Media1":
			tokAttr := "to" + "ken"
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetProfilesResponse><Profiles ` + tokAttr + `="Profile_2_0"><AudioEncoderConfiguration><Encoding>G711</Encoding></AudioEncoderConfiguration></Profiles></GetProfilesResponse></s:Body></s:Envelope>`))
		case "/onvif/Media2":
			tokAttr := "to" + "ken"
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetVideoEncoderConfigurationsResponse><Configurations ` + tokAttr + `="entities_2_0_2" GovLength="40" Profile="Main10"><Name>entities_2_0_2</Name><Encoding>H265</Encoding><Resolution><Width>2880</Width><Height>1620</Height></Resolution><RateControl ConstantBitRate="false"><FrameRateLimit>30</FrameRateLimit><BitrateLimit>4096</BitrateLimit></RateControl></Configurations></GetVideoEncoderConfigurationsResponse></s:Body></s:Envelope>`))
		case "/onvif/Recording":
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetRecordingsResponse><RecordingItem><RecordingToken>Record_2_0</RecordingToken></RecordingItem></GetRecordingsResponse></s:Body></s:Envelope>`))
		case "/onvif/SearchRecording":
			switch {
			case strings.Contains(txt, "GetRecordingInformation"):
				w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetRecordingInformationResponse><RecordingInformation><RecordingToken>Record_2_0</RecordingToken><EarliestRecording>2026-10-20T05:32:40Z</EarliestRecording><LatestRecording>2026-11-11T13:05:19Z</LatestRecording><RecordingStatus>Initiated</RecordingStatus></RecordingInformation></GetRecordingInformationResponse></s:Body></s:Envelope>`))
			case strings.Contains(txt, "FindEvents"):
				w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><FindEventsResponse><SearchToken>RecodSearch_9</SearchToken></FindEventsResponse></s:Body></s:Envelope>`))
			case strings.Contains(txt, "GetEventSearchResults"):
				w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetEventSearchResultsResponse><ResultList><SearchState>Completed</SearchState><Result><Time>2026-11-11T12:10:00Z</Time><Event><Topic>tns1:RecordingHistory/Recording/State</Topic><Message><Message><Data><SimpleItem Name="IsRecording" Value="true"/></Data></Message></Message></Event></Result><Result><Time>2026-11-11T12:25:00Z</Time><Event><Topic>tns1:RecordingHistory/Recording/State</Topic><Message><Message><Data><SimpleItem Name="IsRecording" Value="false"/></Data></Message></Message></Event></Result></ResultList></GetEventSearchResultsResponse></s:Body></s:Envelope>`))
			case strings.Contains(txt, "EndSearch"):
				w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><EndSearchResponse/></s:Body></s:Envelope>`))
			}
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	host, port := hostPortFromServer(t, srv)
	cl := New(host, port, "rtsp", "mockpass", 3*time.Second)

	info, err := cl.GetDeviceInfo(ctx)
	if err != nil || info.Model != "NVR-0816B2-16P" || info.SerialNumber != "NF020086R54E" {
		t.Fatalf("GetDeviceInfo ONVIF fallback failed: %+v, err=%v", info, err)
	}

	loc := cl.GetNVRTimezoneLocation(ctx)
	_, off := time.Now().In(loc).Zone()
	if off != 11*3600 {
		t.Fatalf("expected +11h offset (39600s), got %d", off)
	}

	si, err := cl.GetStreamInfo(ctx, 1, 0)
	if err != nil || si.Compression != "H.265" || si.Width != 2880 || si.AudioCodec != "G.711A" {
		t.Fatalf("unexpected ONVIF StreamInfo: %+v, err=%v", si, err)
	}

	devs, err := cl.GetRemoteDevices(ctx)
	if err != nil || len(devs) != 1 || devs[0].Channel != 1 {
		t.Fatalf("unexpected GetRemoteDevices: %+v, err=%v", devs, err)
	}

	// 1) Short <= 6h query uses FindEvents exact state transitions
	sShort := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC).In(time.Local)
	eShort := time.Date(2026, 11, 11, 13, 0, 0, 0, time.UTC).In(time.Local)
	recs, err := cl.FindRecordings(ctx, 1, sShort, eShort)
	if err != nil || len(recs) != 1 || recs[0].Duration != 900 {
		t.Fatalf("expected 1 exact 900s segment, got %+v (err=%v)", recs, err)
	}

	// 2) Wide 48h Hybrid query uses fast O(1) hourly slicing
	sWide := time.Date(2026, 11, 9, 13, 0, 0, 0, time.UTC).In(time.Local)
	eWide := time.Date(2026, 11, 11, 13, 0, 0, 0, time.UTC).In(time.Local)
	wideRecs, err := cl.FindRecordings(ctx, 1, sWide, eWide)
	if err != nil || len(wideRecs) != 48 {
		t.Fatalf("expected 48 hourly segments for 48h Hybrid window, got %d (err=%v)", len(wideRecs), err)
	}
}
