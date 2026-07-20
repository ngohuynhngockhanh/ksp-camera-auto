package discovery

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const onvifMulticastAddr = "239.255.255.250:3702"

// onvifProbeTemplate is the WS-Discovery SOAP Probe envelope, scoped to
// NetworkVideoTransmitter devices per the ONVIF WS-Discovery profile. %s is
// replaced with a fresh "urn:uuid:..." MessageID on every call.
const onvifProbeTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope"
            xmlns:w="http://schemas.xmlsoap.org/ws/2004/08/addressing"
            xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery"
            xmlns:dn="http://www.onvif.org/ver10/network/wsdl">
  <e:Header>
    <w:MessageID>%s</w:MessageID>
    <w:To e:mustUnderstand="1">urn:schemas-xmlsoap-org:ws:2005:04:discovery</w:To>
    <w:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</w:Action>
  </e:Header>
  <e:Body>
    <d:Probe>
      <d:Types>dn:NetworkVideoTransmitter</d:Types>
    </d:Probe>
  </e:Body>
</e:Envelope>`

// newUUID returns a random RFC 4122 version-4 UUID string.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// buildONVIFProbe renders the SOAP Probe with a freshly generated MessageID.
func buildONVIFProbe() string {
	return fmt.Sprintf(onvifProbeTemplate, "urn:uuid:"+newUUID())
}

// scanONVIF broadcasts a WS-Discovery Probe to the ONVIF multicast group and
// collects ProbeMatch responses for the given timeout.
func scanONVIF(ctx context.Context, timeout time.Duration) ([]Result, error) {
	dst, err := net.ResolveUDPAddr("udp4", onvifMulticastAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	probe := buildONVIFProbe()
	if _, err := conn.WriteToUDP([]byte(probe), dst); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetReadDeadline(deadline)

	var results []Result
	buf := make([]byte, 65535)
	for {
		if ctx.Err() != nil {
			break
		}
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			break // timeout or closed: stop collecting
		}
		xaddrs, scopes, err := parseONVIFProbeMatch(buf[:n])
		if err != nil || len(xaddrs) == 0 {
			continue
		}
		ip := hostFromXAddr(xaddrs[0])
		if ip == "" {
			ip = addr.IP.String()
		}
		name, model := scopeNameModel(scopes)
		results = append(results, Result{
			IP:     ip,
			Vendor: vendorFromText(name + " " + model),
			Model:  model,
			Name:   name,
			Via:    "onvif",
		})
	}
	return results, nil
}

// parseONVIFProbeMatch streams the SOAP response, extracting the text of any
// element whose local name is XAddrs or Scopes, regardless of the XML
// namespace prefix used by the responding vendor (these vary widely across
// ONVIF implementations).
func parseONVIFProbeMatch(data []byte) (xaddrs []string, scopes []string, err error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var capture string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "XAddrs", "Scopes":
				capture = t.Name.Local
			}
		case xml.CharData:
			if capture != "" {
				text := strings.TrimSpace(string(t))
				if text != "" {
					switch capture {
					case "XAddrs":
						xaddrs = append(xaddrs, strings.Fields(text)...)
					case "Scopes":
						scopes = append(scopes, strings.Fields(text)...)
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == capture {
				capture = ""
			}
		}
	}
	if len(xaddrs) == 0 && len(scopes) == 0 {
		return nil, nil, fmt.Errorf("no ProbeMatch data found")
	}
	return xaddrs, scopes, nil
}

// hostFromXAddr extracts the host (IP) portion of an XAddrs URL, e.g.
// "http://192.168.1.64/onvif/device_service" -> "192.168.1.64".
func hostFromXAddr(xaddr string) string {
	u, err := url.Parse(xaddr)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	return host
}

// scopeNameModel pulls a human name and model/hardware id out of the ONVIF
// Scopes list, e.g. "onvif://www.onvif.org/name/HIKVISION%20DS-2CD2143G0-I"
// and "onvif://www.onvif.org/hardware/DS-2CD2143G0-I".
func scopeNameModel(scopes []string) (name, model string) {
	for _, s := range scopes {
		lower := strings.ToLower(s)
		switch {
		case strings.Contains(lower, "/name/"):
			if v := afterLastSegment(s, "/name/"); v != "" {
				name = v
			}
		case strings.Contains(lower, "/hardware/"):
			if v := afterLastSegment(s, "/hardware/"); v != "" {
				model = v
			}
		}
	}
	return name, model
}

// afterLastSegment returns the URL-decoded remainder of s after the given
// marker substring (case-insensitive search, original-case result).
func afterLastSegment(s, marker string) string {
	idx := strings.Index(strings.ToLower(s), marker)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(marker):]
	if decoded, err := url.QueryUnescape(rest); err == nil {
		return decoded
	}
	return rest
}

// vendorFromText does a cheap keyword match against common vendor names
// found in ONVIF Scopes text.
func vendorFromText(s string) string {
	lower := strings.ToLower(s)
	switch {
	case strings.Contains(lower, "hikvision") || strings.Contains(lower, "hik "):
		return "hikvision"
	case strings.Contains(lower, "dahua"):
		return "dahua"
	case strings.Contains(lower, "kbvision"):
		return "dahua"
	case strings.Contains(lower, "tiandy"):
		return "tiandy"
	default:
		return ""
	}
}
