package importer

import (
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestParseShinobi(t *testing.T) {
	// Placeholder creds/IPs — same shape as a Shinobi export. Hik RTSP path is
	// /Streaming/Channels/<n>; Dahua is /cam/realmonitor.
	data := `[
	 {"ke":"wifi","mid":"camera01","name":"Camera01","host":"10.0.0.10",
	  "details":{"auto_host":"rtsp://user1:secretA@10.0.0.10:554/Streaming/Channels/101","muser":"user1","mpass":"secretA"}},
	 {"mid":"dh01","name":"DahuaCam",
	  "details":{"auto_host":"rtsp://user2:secretB@10.0.0.20:554/cam/realmonitor?channel=1&subtype=0"}},
	 {"mid":"nohost","name":"Broken","details":{"auto_host":""}}
	]`
	res, err := ParseShinobi([]byte(data), 80, 37777)
	if err != nil {
		t.Fatalf("ParseShinobi: %v", err)
	}
	if len(res.Devices) != 2 || res.Skipped != 1 {
		t.Fatalf("got %d devices / %d skipped, want 2/1", len(res.Devices), res.Skipped)
	}
	hik := res.Devices[0]
	if hik.Name != "Camera01" || hik.Host != "10.0.0.10" || hik.Vendor != config.VendorHikvision ||
		hik.Username != "user1" || hik.Password != "secretA" || hik.Port != 80 {
		t.Errorf("hik parse wrong: %+v", hik)
	}
	dh := res.Devices[1]
	if dh.Vendor != config.VendorDahua || dh.Host != "10.0.0.20" || dh.Username != "user2" ||
		dh.Password != "secretB" || dh.Port != 37777 {
		t.Errorf("dahua detect wrong: %+v", dh)
	}
}

func TestParseShinobiStringifiedDetails(t *testing.T) {
	// Live Shinobi API returns details as a JSON-encoded STRING.
	data := `[{"mid":"cam1","name":"StrCam","details":"{\"auto_host\":\"rtsp://u:p@10.0.0.7:554/Streaming/Channels/101\",\"muser\":\"u\",\"mpass\":\"p\"}"}]`
	res, err := ParseShinobi([]byte(data), 80, 37777)
	if err != nil {
		t.Fatalf("ParseShinobi: %v", err)
	}
	if len(res.Devices) != 1 {
		t.Fatalf("got %d devices, want 1", len(res.Devices))
	}
	d := res.Devices[0]
	if d.Host != "10.0.0.7" || d.Vendor != config.VendorHikvision || d.Username != "u" || d.Password != "p" {
		t.Errorf("stringified details parsed wrong: %+v", d)
	}
}

func TestParseShinobiTVTMultiChannel(t *testing.T) {
	data := `[
	 {"mid":"camera01","name":"Camera01","host":"192.168.1.253",
	  "details":{"auto_host":"rtsp://rtsp:secret%40@192.168.1.253:554/chID=2&streamType=main&linkType=tcp","muser":"rtsp","mpass":"secret%40"}},
	 {"mid":"camera02","name":"Camera02","host":"192.168.1.253",
	  "details":{"auto_host":"rtsp://rtsp:secret%40@192.168.1.253:554/chID=3&streamType=main&linkType=tcp","muser":"rtsp","mpass":"secret%40"}}
	]`
	res, err := ParseShinobi([]byte(data), 8000, 37777)
	if err != nil {
		t.Fatalf("ParseShinobi TVT: %v", err)
	}
	if len(res.Devices) != 2 {
		t.Fatalf("got %d devices, want 2", len(res.Devices))
	}
	if res.Devices[0].Vendor != config.VendorTVT || res.Devices[0].Port != 80 || res.Devices[0].NVRChannel != 2 || res.Devices[0].ID != "192.168.1.253:80-c2" {
		t.Fatalf("unexpected TVT device[0]: %+v", res.Devices[0])
	}
	if res.Devices[1].Vendor != config.VendorTVT || res.Devices[1].NVRChannel != 3 || res.Devices[1].Password != "secret@" {
		t.Fatalf("unexpected TVT device[1]: %+v", res.Devices[1])
	}
}
