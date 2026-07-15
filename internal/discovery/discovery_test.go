package discovery

import (
	"encoding/binary"
	"encoding/xml"
	"regexp"
	"strings"
	"testing"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestBuildONVIFProbe(t *testing.T) {
	probe := buildONVIFProbe()

	for _, want := range []string{
		"<d:Probe>",
		"<d:Types>dn:NetworkVideoTransmitter</d:Types>",
		"<w:MessageID>urn:uuid:",
		"urn:schemas-xmlsoap-org:ws:2005:04:discovery",
	} {
		if !strings.Contains(probe, want) {
			t.Errorf("ONVIF probe missing %q; got:\n%s", want, probe)
		}
	}

	m := regexp.MustCompile(`<w:MessageID>urn:uuid:([0-9a-f-]+)</w:MessageID>`).FindStringSubmatch(probe)
	if m == nil {
		t.Fatalf("MessageID element not found in probe:\n%s", probe)
	}
	if !uuidRe.MatchString(m[1]) {
		t.Errorf("MessageID %q is not a valid v4 uuid", m[1])
	}
}

func TestBuildONVIFProbeUniqueMessageID(t *testing.T) {
	a := buildONVIFProbe()
	b := buildONVIFProbe()
	if a == b {
		t.Fatalf("two probes produced identical MessageIDs (not unique):\n%s", a)
	}
}

func TestBuildSADPProbe(t *testing.T) {
	probe := buildSADPProbe()

	for _, want := range []string{
		"<Probe>",
		"<Uuid>",
		"<Types>inquiry</Types>",
		"</Probe>",
	} {
		if !strings.Contains(probe, want) {
			t.Errorf("SADP probe missing %q; got:\n%s", want, probe)
		}
	}

	m := regexp.MustCompile(`<Uuid>([0-9a-f-]+)</Uuid>`).FindStringSubmatch(probe)
	if m == nil {
		t.Fatalf("Uuid element not found in probe:\n%s", probe)
	}
	if !uuidRe.MatchString(m[1]) {
		t.Errorf("Uuid %q is not a valid v4 uuid", m[1])
	}

	// Must parse as well-formed XML.
	var doc struct {
		Uuid  string `xml:"Uuid"`
		Types string `xml:"Types"`
	}
	if err := xml.Unmarshal([]byte(probe), &doc); err != nil {
		t.Fatalf("SADP probe is not valid XML: %v", err)
	}
	if doc.Types != "inquiry" {
		t.Errorf("Types = %q, want %q", doc.Types, "inquiry")
	}
}

// sampleONVIFProbeMatch is a representative WS-Discovery ProbeMatch envelope
// (trimmed to the elements this package reads), modeled on a Hikvision
// camera's response.
const sampleONVIFProbeMatch = `<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://www.w3.org/2003/05/soap-envelope"
                    xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing"
                    xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery"
                    xmlns:dn="http://www.onvif.org/ver10/network/wsdl">
  <SOAP-ENV:Header>
    <wsa:MessageID>urn:uuid:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee</wsa:MessageID>
    <wsa:RelatesTo>urn:uuid:11111111-2222-3333-4444-555555555555</wsa:RelatesTo>
    <wsa:To SOAP-ENV:mustUnderstand="1">http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</wsa:To>
    <wsa:Action SOAP-ENV:mustUnderstand="1">http://schemas.xmlsoap.org/ws/2005/04/discovery/ProbeMatches</wsa:Action>
  </SOAP-ENV:Header>
  <SOAP-ENV:Body>
    <d:ProbeMatches>
      <d:ProbeMatch>
        <wsa:EndpointReference>
          <wsa:Address>urn:uuid:44444444-5555-6666-7777-888888888888</wsa:Address>
        </wsa:EndpointReference>
        <d:Types>dn:NetworkVideoTransmitter</d:Types>
        <d:Scopes>onvif://www.onvif.org/type/video_encoder onvif://www.onvif.org/hardware/DS-2CD2143G0-I onvif://www.onvif.org/name/HIKVISION%20DS-2CD2143G0-I</d:Scopes>
        <d:XAddrs>http://192.168.1.64/onvif/device_service</d:XAddrs>
        <d:MetadataVersion>1</d:MetadataVersion>
      </d:ProbeMatch>
    </d:ProbeMatches>
  </SOAP-ENV:Body>
</SOAP-ENV:Envelope>`

func TestParseONVIFProbeMatch(t *testing.T) {
	xaddrs, scopes, err := parseONVIFProbeMatch([]byte(sampleONVIFProbeMatch))
	if err != nil {
		t.Fatalf("parseONVIFProbeMatch: %v", err)
	}
	if len(xaddrs) != 1 || xaddrs[0] != "http://192.168.1.64/onvif/device_service" {
		t.Fatalf("xaddrs = %v, want [http://192.168.1.64/onvif/device_service]", xaddrs)
	}
	ip := hostFromXAddr(xaddrs[0])
	if ip != "192.168.1.64" {
		t.Errorf("hostFromXAddr = %q, want 192.168.1.64", ip)
	}

	name, model := scopeNameModel(scopes)
	if model != "DS-2CD2143G0-I" {
		t.Errorf("model = %q, want DS-2CD2143G0-I", model)
	}
	if name != "HIKVISION DS-2CD2143G0-I" {
		t.Errorf("name = %q, want %q", name, "HIKVISION DS-2CD2143G0-I")
	}
	if vendor := vendorFromText(name); vendor != "hikvision" {
		t.Errorf("vendorFromText(%q) = %q, want hikvision", name, vendor)
	}
}

// sampleNmapOutput is representative nmap stdout for -Pn -sT -p 80,8000,37777,37778 --open.
const sampleNmapOutput = `Starting Nmap 7.94SVN ( https://nmap.org ) at 2026-07-15 10:00 +07
Nmap scan report for 192.168.1.10
Host is up (0.00050s latency).

PORT      STATE SERVICE
80/tcp    open  http
8000/tcp  open  http-alt

Nmap scan report for dvr.local (192.168.1.20)
Host is up (0.00040s latency).

PORT      STATE SERVICE
37777/tcp open  unknown
37778/tcp open  unknown

Nmap scan report for 192.168.1.30
Host is up (0.00060s latency).

PORT      STATE SERVICE
80/tcp    open  http

Nmap done: 256 IP addresses (3 hosts up) scanned in 4.20 seconds
`

func TestParseNmapOutput(t *testing.T) {
	got := parseNmapOutput(sampleNmapOutput)

	want := []Result{
		{IP: "192.168.1.10", Port: 80, Vendor: "", Via: "nmap"},
		{IP: "192.168.1.10", Port: 8000, Vendor: "hikvision", Via: "nmap"},
		{IP: "192.168.1.20", Port: 37777, Vendor: "dahua", Via: "nmap"},
		{IP: "192.168.1.20", Port: 37778, Vendor: "dahua", Via: "nmap"},
		{IP: "192.168.1.30", Port: 80, Vendor: "", Via: "nmap"},
	}
	if len(got) != len(want) {
		t.Fatalf("parseNmapOutput returned %d results, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("result[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseDahuaResponse(t *testing.T) {
	body := []byte(`{"method":"client.notifyDeviceInfo","params":{"deviceInfo":{"DeviceType":"IPC-HFW2431S","Mac":"3c:ef:8c:bf:a2:04","IPv4Address":"192.168.1.108"}}}`)
	pkt := make([]byte, dahuaHeaderLen+len(body))
	pkt[0] = 0x20
	copy(pkt[4:8], "DHIP")
	binary.LittleEndian.PutUint32(pkt[16:20], uint32(len(body)))
	binary.LittleEndian.PutUint32(pkt[24:28], uint32(len(body)))
	copy(pkt[dahuaHeaderLen:], body)

	r, ok := parseDahuaResponse(pkt, "192.168.1.108")
	if !ok {
		t.Fatalf("parseDahuaResponse: not ok")
	}
	if r.IP != "192.168.1.108" {
		t.Errorf("IP = %q, want 192.168.1.108", r.IP)
	}
	if r.MAC != "3c:ef:8c:bf:a2:04" {
		t.Errorf("MAC = %q, want 3c:ef:8c:bf:a2:04", r.MAC)
	}
	if r.Model != "IPC-HFW2431S" {
		t.Errorf("Model = %q, want IPC-HFW2431S", r.Model)
	}
	if r.Via != "dahua" {
		t.Errorf("Via = %q, want dahua", r.Via)
	}
}

func TestMergeResultsPrefersMoreInfo(t *testing.T) {
	in := []Result{
		{IP: "192.168.1.5", Via: "nmap", Port: 80},
		{IP: "192.168.1.5", Via: "onvif", Vendor: "hikvision", Model: "DS-2CD", Name: "cam1"},
		{IP: "192.168.1.1", Via: "dahua", Vendor: "dahua"},
	}
	out := mergeResults(in)
	if len(out) != 2 {
		t.Fatalf("mergeResults returned %d entries, want 2: %+v", len(out), out)
	}
	if out[0].IP != "192.168.1.1" || out[1].IP != "192.168.1.5" {
		t.Fatalf("not sorted by IP: %+v", out)
	}
	if out[1].Via != "onvif" {
		t.Errorf("expected the more informative (onvif) entry to win, got %+v", out[1])
	}
}

func TestIsSafeScanTarget(t *testing.T) {
	ok := []string{"192.168.1.0/24", "10.0.0.5", "192.168.1.1-254", "172.16.0.0/16"}
	for _, s := range ok {
		if !isSafeScanTarget(s) {
			t.Errorf("should accept %q", s)
		}
	}
	bad := []string{"-oX /etc/passwd", "--script=x", "1.2.3.4; rm -rf /", "1.2.3.4 8.8.8.8", "$(whoami)", "1.2.3.4|nc", "", "example.com"}
	for _, s := range bad {
		if isSafeScanTarget(s) {
			t.Errorf("should REJECT %q", s)
		}
	}
}
