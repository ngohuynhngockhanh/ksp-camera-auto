package isapi

import (
	"strings"
	"testing"
)

// TestBuildAuthorizationRFC2617Vector checks BuildAuthorization against the
// worked example from RFC 2617 §3.5 (qop=auth, MD5): username="Mufasa",
// realm="testrealm@host.com", password="Circle Of Life", method=GET,
// uri="/dir/index.html", nonce="dcd98b7102dd2f0e8b11d0f600bfb0c093",
// nc=00000001, cnonce="0a4f113b", giving the well-known
// response="6629fae49393a05397450978507c4ef1". That value is also
// recomputed independently below (HA1/HA2/response) via the RFC 2617
// formula, so the test doesn't rely purely on a memorized constant.
func TestBuildAuthorizationRFC2617Vector(t *testing.T) {
	chal := Challenge{
		Realm:  "testrealm@host.com",
		Nonce:  "dcd98b7102dd2f0e8b11d0f600bfb0c093",
		QOP:    "auth",
		Opaque: "5ccc069c403ebaf9f0171e9517f40e41",
	}
	const (
		username     = "Mufasa"
		password     = "Circle Of Life"
		method       = "GET"
		uri          = "/dir/index.html"
		cnonce       = "0a4f113b"
		nc           = "00000001"
		wantResponse = "6629fae49393a05397450978507c4ef1"
	)

	// Independently recompute the expected response using the RFC 2617
	// formula so this test cross-checks the hardcoded RFC constant too.
	recomputedHA1 := md5hex(username + ":" + chal.Realm + ":" + password)
	recomputedHA2 := md5hex(method + ":" + uri)
	recomputedResponse := md5hex(recomputedHA1 + ":" + chal.Nonce + ":" + nc + ":" + cnonce + ":auth:" + recomputedHA2)
	if recomputedResponse != wantResponse {
		t.Fatalf("recomputed response %s != RFC 2617 constant %s", recomputedResponse, wantResponse)
	}

	got := BuildAuthorization(chal, method, uri, username, password, cnonce, nc)

	wantAuth := `Digest username="Mufasa", realm="testrealm@host.com", nonce="dcd98b7102dd2f0e8b11d0f600bfb0c093", uri="/dir/index.html", response="` +
		wantResponse + `", qop=auth, nc=00000001, cnonce="0a4f113b", opaque="5ccc069c403ebaf9f0171e9517f40e41"`

	if got != wantAuth {
		t.Fatalf("BuildAuthorization mismatch\n got: %s\nwant: %s", got, wantAuth)
	}
}

// TestBuildAuthorizationNoOpaque checks the header is well-formed when the
// challenge carries no opaque value (some Hikvision firmware omits it).
func TestBuildAuthorizationNoOpaque(t *testing.T) {
	chal := Challenge{Realm: "IP Camera", Nonce: "abc123"}
	got := BuildAuthorization(chal, "PUT", "/ISAPI/Streaming/channels/101", "admin", "secret", "cnonceval", "00000002")
	want := `Digest username="admin", realm="IP Camera", nonce="abc123", uri="/ISAPI/Streaming/channels/101", response="`
	if len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("BuildAuthorization missing expected prefix: %s", got)
	}
	if strings.Contains(got, "opaque=") {
		t.Fatalf("expected no opaque segment when challenge has none: %s", got)
	}
}

// TestParseChallenge covers a realistic Hikvision WWW-Authenticate header,
// including a quoted realm and an unquoted qop value.
func TestParseChallenge(t *testing.T) {
	header := `Digest qop="auth", realm="DS-2CD2143G0-I", nonce="63c1f4a4000000000123456789abcdef", opaque="deadbeef"`
	c, err := ParseChallenge(header)
	if err != nil {
		t.Fatalf("ParseChallenge: %v", err)
	}
	if c.Realm != "DS-2CD2143G0-I" {
		t.Errorf("realm = %q, want DS-2CD2143G0-I", c.Realm)
	}
	if c.Nonce != "63c1f4a4000000000123456789abcdef" {
		t.Errorf("nonce = %q", c.Nonce)
	}
	if c.QOP != "auth" {
		t.Errorf("qop = %q, want auth", c.QOP)
	}
	if c.Opaque != "deadbeef" {
		t.Errorf("opaque = %q", c.Opaque)
	}
}

func TestParseChallengeRejectsBasic(t *testing.T) {
	if _, err := ParseChallenge(`Basic realm="foo"`); err == nil {
		t.Fatalf("expected error for non-Digest scheme")
	}
}

func TestParseChallengeRejectsIncomplete(t *testing.T) {
	if _, err := ParseChallenge(`Digest qop="auth"`); err == nil {
		t.Fatalf("expected error for challenge missing realm/nonce")
	}
}
