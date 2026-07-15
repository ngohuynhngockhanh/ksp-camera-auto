package discovery

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Dahua's DHDiscover protocol listens on UDP/37810. Devices answer both a
// subnet broadcast and a dedicated multicast group.
const (
	dahuaPort          = 37810
	dahuaBroadcastAddr = "255.255.255.255:37810"
	dahuaMulticastAddr = "239.255.255.251:37810"
	dahuaHeaderLen     = 32
)

// dahuaMACRe matches a colon-separated MAC address anywhere in a decoded
// DHDiscover response, used as a schema-agnostic way to pull the MAC out of
// whatever JSON shape a given firmware happens to send back.
var dahuaMACRe = regexp.MustCompile(`^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$`)

// dahuaDiscoverPacket builds the DHIP-framed DHDiscover.search request.
//
// Framing reverse-engineered from docs-sdk/dahua/DahuaConsole-net.py
// (dh_discover, 'dhip' branch): a 32-byte header —
//
//	[0:4]   0x20000000
//	[4:8]   "DHIP"
//	[8:16]  zero
//	[16:20] payload length, little-endian
//	[20:24] zero
//	[24:28] payload length, little-endian (repeated)
//	[28:32] zero
//
// — followed by the JSON payload. This mirrors the header conventions
// already used for the DVRIP/DHIP TCP protocol in internal/dahua/dhip.go
// (big-endian opcode marker, little-endian length fields).
func dahuaDiscoverPacket() []byte {
	payload, _ := json.Marshal(map[string]any{
		"method": "DHDiscover.search",
		"params": map[string]any{
			"mac": "",
			"uni": 1,
		},
	})
	pkt := make([]byte, dahuaHeaderLen+len(payload))
	pkt[0] = 0x20
	copy(pkt[4:8], "DHIP")
	binary.LittleEndian.PutUint32(pkt[16:20], uint32(len(payload)))
	binary.LittleEndian.PutUint32(pkt[24:28], uint32(len(payload)))
	copy(pkt[dahuaHeaderLen:], payload)
	return pkt
}

// scanDahua broadcasts + multicasts a DHDiscover.search request and collects
// JSON replies for the given timeout.
func scanDahua(ctx context.Context, timeout time.Duration) ([]Result, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	pkt := dahuaDiscoverPacket()

	if bcast, err := net.ResolveUDPAddr("udp4", dahuaBroadcastAddr); err == nil {
		if err := setBroadcast(conn); err == nil {
			_, _ = conn.WriteToUDP(pkt, bcast)
		}
	}
	if mcast, err := net.ResolveUDPAddr("udp4", dahuaMulticastAddr); err == nil {
		_, _ = conn.WriteToUDP(pkt, mcast)
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
		r, ok := parseDahuaResponse(buf[:n], addr.IP.String())
		if ok {
			results = append(results, r)
		}
	}
	return results, nil
}

// parseDahuaResponse decodes a DHIP-framed DHDiscover reply. srcIP (the UDP
// packet's source address) is always used for Result.IP since it is the one
// piece of information guaranteed correct regardless of the JSON body's
// (undocumented, firmware-dependent) field names; MAC/Model are filled in on
// a best-effort basis by scanning the decoded JSON for MAC-shaped strings and
// common device-type keys.
func parseDahuaResponse(data []byte, srcIP string) (Result, bool) {
	if len(data) < dahuaHeaderLen || string(data[4:8]) != "DHIP" {
		return Result{}, false
	}
	body := data[dahuaHeaderLen:]
	if n := binary.LittleEndian.Uint32(data[16:20]); int(n) > 0 && int(n) <= len(body) {
		body = body[:n]
	}
	body = []byte(strings.Trim(string(body), "\x00"))
	if len(body) == 0 {
		return Result{}, false
	}

	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		// Not JSON we can parse, but we still know a Dahua device answered.
		return Result{IP: srcIP, Vendor: "dahua", Via: "dahua"}, true
	}
	mac, model := findDahuaFields(doc)
	return Result{
		IP:     srcIP,
		Vendor: "dahua",
		Model:  model,
		MAC:    mac,
		Via:    "dahua",
	}, true
}

// findDahuaFields walks an arbitrary decoded JSON document looking for a
// MAC-shaped string value and a string value under a device-type-ish key.
func findDahuaFields(node any) (mac, model string) {
	var walk func(any)
	walk = func(n any) {
		switch t := n.(type) {
		case map[string]any:
			for k, v := range t {
				if s, ok := v.(string); ok {
					if mac == "" && dahuaMACRe.MatchString(s) {
						mac = s
					}
					lk := strings.ToLower(k)
					if model == "" && (lk == "devicetype" || lk == "type" || lk == "detail" || lk == "devtype") {
						model = s
					}
				}
				walk(v)
			}
		case []any:
			for _, v := range t {
				walk(v)
			}
		}
	}
	walk(node)
	return mac, model
}

// setBroadcast enables SO_BROADCAST on conn so a datagram addressed to
// 255.255.255.255 is actually sent instead of rejected by the kernel.
func setBroadcast(conn *net.UDPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	})
	if err != nil {
		return err
	}
	return sockErr
}
