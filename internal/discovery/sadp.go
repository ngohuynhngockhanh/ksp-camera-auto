package discovery

import (
	"context"
	"encoding/xml"
	"net"
	"time"
)

const sadpMulticastAddr = "239.255.255.250:37020"

// buildSADPProbe renders Hikvision's SADP inquiry probe with a freshly
// generated UUID. Split out from scanSADP so it's unit-testable without
// depending on network I/O.
func buildSADPProbe() string {
	return "<?xml version=\"1.0\" encoding=\"utf-8\"?><Probe><Uuid>" + newUUID() + "</Uuid><Types>inquiry</Types></Probe>"
}

// sadpProbeMatch is Hikvision's SADP ProbeMatch reply. Only the fields we
// care about for a Result are decoded; the wire format carries many more
// (subnet mask, gateway, DHCP flag, etc.) that we don't need here.
type sadpProbeMatch struct {
	XMLName           xml.Name `xml:"ProbeMatch"`
	DeviceType        string   `xml:"DeviceType"`
	DeviceDescription string   `xml:"DeviceDescription"`
	MAC               string   `xml:"MAC"`
	IPv4Address       string   `xml:"IPv4Address"`
	SoftwareVersion   string   `xml:"SoftwareVersion"`
}

func parseSADPProbeMatch(data []byte) (sadpProbeMatch, error) {
	var m sadpProbeMatch
	if err := xml.Unmarshal(data, &m); err != nil {
		return sadpProbeMatch{}, err
	}
	return m, nil
}

// scanSADP multicasts a SADP inquiry probe and collects ProbeMatch replies
// for the given timeout.
func scanSADP(ctx context.Context, timeout time.Duration) ([]Result, error) {
	dst, err := net.ResolveUDPAddr("udp4", sadpMulticastAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	probe := buildSADPProbe()
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
			break
		}
		m, err := parseSADPProbeMatch(buf[:n])
		if err != nil {
			continue
		}
		ip := m.IPv4Address
		if ip == "" {
			ip = addr.IP.String()
		}
		model := m.DeviceType
		if model == "" {
			model = m.DeviceDescription
		}
		results = append(results, Result{
			IP:     ip,
			Vendor: "hikvision",
			Model:  model,
			MAC:    m.MAC,
			Name:   m.SoftwareVersion,
			Via:    "hikvision-sadp",
		})
	}
	return results, nil
}
