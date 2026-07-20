package isapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeContentMgmtServer emulates just enough of an NVR's ContentMgmt surface
// to exercise GetStorage/GetInputProxyChannels: Digest auth (reusing
// fakeISAPIServer's credential check) plus canned
// /ISAPI/ContentMgmt/{Storage,InputProxy/channels} bodies.
type fakeContentMgmtServer struct {
	auth *fakeISAPIServer

	mu             sync.Mutex
	storageBody    string
	inputProxyBody string
}

func newFakeContentMgmtServer(user, pass string) *fakeContentMgmtServer {
	return &fakeContentMgmtServer{auth: newFakeISAPIServer(user, pass)}
}

func (s *fakeContentMgmtServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.checkAuth(r.Header.Get("Authorization"), r.Method, r.URL.RequestURI()) {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Digest realm="%s", nonce="%s", qop="auth"`, s.auth.realm, s.auth.nonce))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
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

func newContentMgmtTestClient(t *testing.T, srv *httptest.Server, user, pass string) *Client {
	t.Helper()
	host, portStr, ok := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	if !ok {
		t.Fatalf("could not split host:port from %s", srv.URL)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return New(host, port, false, user, pass, 5*time.Second)
}

// TestGetStorageParsesHDDList verifies the live-verified DS-7632NXI-K2 shape:
// one healthy hdd with capacity/freeSpace in MB.
func TestGetStorageParsesHDDList(t *testing.T) {
	fake := newFakeContentMgmtServer("admin", "duyanh68A")
	// Real DS-7632NXI-K2 root element is lowercase <storage> (verified live);
	// the multi-HDD case below uses <Storage> to prove both casings parse.
	fake.storageBody = `<?xml version="1.0" encoding="UTF-8"?>
<storage version="1.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<hddList>
<hdd version="1.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<id>1</id>
<hddName>hdd1</hddName>
<hddType>SATA</hddType>
<status>ok</status>
<capacity>3815447</capacity>
<freeSpace>0</freeSpace>
<property>RW</property>
</hdd>
</hddList>
</storage>`
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newContentMgmtTestClient(t, srv, "admin", "duyanh68A")

	hdds, err := c.GetStorage(context.Background())
	if err != nil {
		t.Fatalf("GetStorage: %v", err)
	}
	if len(hdds) != 1 {
		t.Fatalf("got %d HDDs, want 1: %+v", len(hdds), hdds)
	}
	h := hdds[0]
	if h.ID != 1 || h.Name != "hdd1" || h.Type != "SATA" || h.Status != "ok" || h.Property != "RW" {
		t.Fatalf("unexpected HDD: %+v", h)
	}
	if h.CapacityMB != 3815447 {
		t.Fatalf("CapacityMB = %d, want 3815447", h.CapacityMB)
	}
	if h.FreeMB != 0 {
		t.Fatalf("FreeMB = %d, want 0", h.FreeMB)
	}
}

// TestGetStorageMultipleHDDsIncludingUnformatted covers a second bay that
// isn't "ok" (e.g. freshly inserted, unformatted) — callers use Status to
// decide usability, not presence alone.
func TestGetStorageMultipleHDDsIncludingUnformatted(t *testing.T) {
	fake := newFakeContentMgmtServer("admin", "duyanh68A")
	fake.storageBody = `<?xml version="1.0" encoding="UTF-8"?>
<Storage>
<hddList>
<hdd><id>1</id><hddName>hdd1</hddName><hddType>SATA</hddType><status>ok</status><capacity>3815447</capacity><freeSpace>1200000</freeSpace><property>RW</property></hdd>
<hdd><id>2</id><hddName>hdd2</hddName><hddType>SATA</hddType><status>unformatted</status><capacity>0</capacity><freeSpace>0</freeSpace><property>RW</property></hdd>
</hddList>
</Storage>`
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newContentMgmtTestClient(t, srv, "admin", "duyanh68A")

	hdds, err := c.GetStorage(context.Background())
	if err != nil {
		t.Fatalf("GetStorage: %v", err)
	}
	if len(hdds) != 2 {
		t.Fatalf("got %d HDDs, want 2: %+v", len(hdds), hdds)
	}
	if hdds[1].Status != "unformatted" {
		t.Fatalf("hdds[1].Status = %q, want unformatted", hdds[1].Status)
	}
	if hdds[1].CapacityMB != 0 {
		t.Fatalf("hdds[1].CapacityMB = %d, want 0", hdds[1].CapacityMB)
	}
}

// TestGetStorageEmptyHddList covers a device with no bays populated: no
// error, just an empty slice (the signal that feeds dahua.HasUsableStorage
// -> false -> NVR fallback).
func TestGetStorageEmptyHddList(t *testing.T) {
	fake := newFakeContentMgmtServer("admin", "duyanh68A")
	fake.storageBody = `<?xml version="1.0" encoding="UTF-8"?><Storage><hddList></hddList></Storage>`
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newContentMgmtTestClient(t, srv, "admin", "duyanh68A")

	hdds, err := c.GetStorage(context.Background())
	if err != nil {
		t.Fatalf("GetStorage: %v", err)
	}
	if len(hdds) != 0 {
		t.Fatalf("got %d HDDs, want 0", len(hdds))
	}
}
