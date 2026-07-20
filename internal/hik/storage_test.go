package hik

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

// hikStorageChannelsServer serves canned ISAPI ContentMgmt bodies (no auth
// gate needed — FindRecordings's tests already exercise the digest round
// trip via fakeHikServer; these tests focus on the storage/channel mapping).
type hikStorageChannelsServer struct {
	storageBody    string
	inputProxyBody string
}

func (s *hikStorageChannelsServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/ISAPI/ContentMgmt/Storage" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, s.storageBody)
		case r.URL.Path == "/ISAPI/ContentMgmt/InputProxy/channels" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, s.inputProxyBody)
		default:
			http.NotFound(w, r)
		}
	}
}

func newHikStorageTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	host, portStr, ok := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	if !ok {
		t.Fatalf("could not split host:port from %s", srv.URL)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	// No credentials needed: hikStorageChannelsServer doesn't gate on auth.
	return Dial(host, port, false, "admin", "duyanh68A", 5*time.Second)
}

// TestGetStorageInfoMapsHealthyHDDToUsableStorage verifies the live-verified
// DS-7632NXI-K2 shape maps to a dahua.StorageDevice that
// dahua.HasUsableStorage reports as usable, and that MB->bytes conversion
// is applied (capacity/freeSpace are MB on the wire, unlike Dahua's already-
// byte TotalBytes/UsedBytes).
func TestGetStorageInfoMapsHealthyHDDToUsableStorage(t *testing.T) {
	fake := &hikStorageChannelsServer{storageBody: `<?xml version="1.0"?>
<Storage><hddList>
<hdd><id>1</id><hddName>hdd1</hddName><hddType>SATA</hddType><status>ok</status><capacity>3815447</capacity><freeSpace>1000000</freeSpace><property>RW</property></hdd>
</hddList></Storage>`}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newHikStorageTestClient(t, srv)

	devs, err := c.GetStorageInfo(context.Background())
	if err != nil {
		t.Fatalf("GetStorageInfo: %v", err)
	}
	if len(devs) != 1 || len(devs[0].Details) != 1 {
		t.Fatalf("unexpected devices: %+v", devs)
	}
	d := devs[0].Details[0]
	if d.TotalBytes != 3815447*(1<<20) {
		t.Fatalf("TotalBytes = %d, want %d (MB->bytes)", d.TotalBytes, 3815447*(1<<20))
	}
	if d.UsedBytes != (3815447-1000000)*(1<<20) {
		t.Fatalf("UsedBytes = %d, want %d", d.UsedBytes, (3815447-1000000)*(1<<20))
	}
	if d.IsError || d.IsNeedFormat {
		t.Fatalf("healthy hdd should not be IsError/IsNeedFormat: %+v", d)
	}
	if !dahua.HasUsableStorage(devs) {
		t.Fatalf("HasUsableStorage = false, want true for a healthy RW hdd")
	}
}

// TestGetStorageInfoMapsUnformattedHDDToUnusable verifies a non-"ok" status
// sets both IsError and IsNeedFormat (for an unformatted disk), and that
// HasUsableStorage correctly reports no usable storage — the signal that
// drives NVR fallback.
func TestGetStorageInfoMapsUnformattedHDDToUnusable(t *testing.T) {
	fake := &hikStorageChannelsServer{storageBody: `<?xml version="1.0"?>
<Storage><hddList>
<hdd><id>1</id><hddName>hdd1</hddName><hddType>SATA</hddType><status>uninitialized</status><capacity>0</capacity><freeSpace>0</freeSpace><property>RW</property></hdd>
</hddList></Storage>`}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newHikStorageTestClient(t, srv)

	devs, err := c.GetStorageInfo(context.Background())
	if err != nil {
		t.Fatalf("GetStorageInfo: %v", err)
	}
	d := devs[0].Details[0]
	if !d.IsError {
		t.Fatalf("non-ok status should set IsError: %+v", d)
	}
	if !d.IsNeedFormat {
		t.Fatalf("uninitialized status should set IsNeedFormat: %+v", d)
	}
	if dahua.HasUsableStorage(devs) {
		t.Fatalf("HasUsableStorage = true, want false for an uninitialized hdd")
	}
}

// TestGetStorageInfoNoHDDsReturnsEmptySlice covers a device with no bays —
// dahua.HasUsableStorage must report false (NoStorage -> NVR fallback).
func TestGetStorageInfoNoHDDsReturnsEmptySlice(t *testing.T) {
	fake := &hikStorageChannelsServer{storageBody: `<?xml version="1.0"?><Storage><hddList></hddList></Storage>`}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newHikStorageTestClient(t, srv)

	devs, err := c.GetStorageInfo(context.Background())
	if err != nil {
		t.Fatalf("GetStorageInfo: %v", err)
	}
	if len(devs) != 0 {
		t.Fatalf("got %d devices, want 0", len(devs))
	}
	if dahua.HasUsableStorage(devs) {
		t.Fatalf("HasUsableStorage = true, want false for no HDDs")
	}
}
