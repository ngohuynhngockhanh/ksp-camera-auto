//go:build hiksdk

// Package hiksdk is the optional cgo backend that reaches Hikvision devices on
// their proprietary port 8000 via the HCNetSDK, carrying ISAPI XML over
// NET_DVR_STDXMLConfig. It implements isapi.Transport, so all of internal/isapi's
// get/set logic is reused unchanged.
//
// Build with `-tags hiksdk` and provide the SDK paths via env:
//
//	CGO_CPPFLAGS="-I<sdk>/incEn"
//	CGO_LDFLAGS="-L<sdk>/lib -lhcnetsdk -Wl,-rpath,<sdk>/lib"
//
// At runtime set KSPCAM_HIKSDK_PATH to the dir containing libhcnetsdk.so and
// the HCNetSDKCom/ plugin directory. The proprietary SDK is never committed.
package hiksdk

/*
#cgo LDFLAGS: -lstdc++
#include "shim.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

var (
	initOnce sync.Once
	initErr  error
)

func ensureInit() error {
	initOnce.Do(func() {
		libdir := os.Getenv("KSPCAM_HIKSDK_PATH")
		clib := C.CString(libdir)
		defer C.free(unsafe.Pointer(clib))
		if rc := C.hik_init(clib); rc != 0 {
			initErr = fmt.Errorf("HCNetSDK init failed (code %d); set KSPCAM_HIKSDK_PATH to the SDK lib dir", int(rc))
		}
	})
	return initErr
}

// Session is a logged-in HCNetSDK user handle.
type Session struct {
	mu  sync.Mutex
	uid C.long
}

// Open initialises the SDK (once per process) and logs into host:port.
func Open(host string, port int, user, pass string) (*Session, error) {
	if err := ensureInit(); err != nil {
		return nil, err
	}
	cip := C.CString(host)
	cu := C.CString(user)
	cp := C.CString(pass)
	defer C.free(unsafe.Pointer(cip))
	defer C.free(unsafe.Pointer(cu))
	defer C.free(unsafe.Pointer(cp))

	uid := C.hik_login(cip, C.ushort(port), cu, cp)
	if uid < 0 {
		return nil, fmt.Errorf("login failed (SDK code %d)", int(-uid))
	}
	return &Session{uid: uid}, nil
}

// Close logs out the session.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.uid >= 0 {
		C.hik_logout(s.uid)
		s.uid = -1
	}
	return nil
}

// Transport returns an isapi.Transport backed by this session.
func (s *Session) Transport() isapi.Transport { return &transport{s: s} }

type transport struct{ s *Session }

// Do carries one ISAPI request over the SDK. STDXMLConfig returns the resource
// XML in the out buffer and, for state-changing requests, the ResponseStatus in
// the status buffer — so GET wants the out buffer while PUT/POST/DELETE want the
// status buffer (which isapi's checkResponseStatus verifies). ctx is accepted
// for interface compatibility; the SDK call is synchronous with its own timeout.
func (t *transport) Do(_ context.Context, method, path string, body []byte) ([]byte, error) {
	out, status, err := t.s.stdxml(method+" "+path, body)
	if err != nil {
		return nil, err
	}
	if method == "GET" {
		if len(out) > 0 {
			return out, nil
		}
		return status, nil
	}
	if len(status) > 0 {
		return status, nil
	}
	return out, nil
}

func (s *Session) stdxml(url string, body []byte) (out, status []byte, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.uid < 0 {
		return nil, nil, fmt.Errorf("session closed")
	}

	curl := C.CString(url)
	defer C.free(unsafe.Pointer(curl))
	var cbody *C.char
	if len(body) > 0 {
		cbody = (*C.char)(C.CBytes(body))
		defer C.free(unsafe.Pointer(cbody))
	}

	const outcap = 1 << 20
	const statuscap = 8192
	outbuf := C.malloc(C.size_t(outcap))
	defer C.free(outbuf)
	statusbuf := C.malloc(C.size_t(statuscap))
	defer C.free(statusbuf)
	var outlen C.uint

	rc := C.hik_stdxml(s.uid, curl, cbody, C.uint(len(body)),
		(*C.char)(outbuf), C.uint(outcap), &outlen,
		(*C.char)(statusbuf), C.uint(statuscap))

	statusStr := C.GoString((*C.char)(statusbuf))
	if rc != 0 {
		return nil, nil, fmt.Errorf("STDXMLConfig %q failed (SDK code %d) %s", url, int(rc), statusStr)
	}
	return C.GoBytes(outbuf, C.int(outlen)), []byte(statusStr), nil
}
