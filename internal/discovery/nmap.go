package discovery

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// nmapPorts are the well-known ports checked when probing a routed subnet
// that UDP discovery can't reach: Hikvision/ONVIF HTTP (80, 8000) and Dahua's
// private protocol (37777 TCP, 37778 sub-stream).
const nmapPorts = "80,8000,37777,37778"

var (
	nmapHostRe = regexp.MustCompile(`^Nmap scan report for (?:\S+ \()?([0-9.]+)\)?$`)
	nmapPortRe = regexp.MustCompile(`^(\d+)/tcp\s+open`)
)

// safeScanTargetRe allows only IPv4 dotted-quads, an optional /prefix, and an
// optional trailing dashed range (e.g. 192.168.1.0/24, 10.0.0.5, 192.168.1.1-254).
var safeScanTargetRe = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){3}(-[0-9]{1,3})?(/[0-9]{1,2})?$`)

// isSafeScanTarget rejects anything that isn't a plain IPv4 target/range/CIDR,
// so a value can never be treated as an nmap flag or shell metacharacter.
func isSafeScanTarget(s string) bool {
	if strings.HasPrefix(s, "-") || strings.ContainsAny(s, " \t;|&$`") {
		return false
	}
	return safeScanTargetRe.MatchString(s)
}

// vendorForPort maps a well-known port to the vendor family it most likely
// belongs to. Port 80 is ambiguous (both vendors expose a web UI there) so it
// intentionally yields an empty vendor.
func vendorForPort(port int) string {
	switch port {
	case 8000:
		return "hikvision"
	case 37777, 37778:
		return "dahua"
	default:
		return ""
	}
}

// ScanSubnet shells out to nmap to find hosts with a camera-typical port
// open across cidr (e.g. "192.168.1.0/24"). Unlike Scan, this works across
// routed subnets since it's a plain TCP connect probe rather than
// UDP multicast/broadcast discovery — see the package doc comment.
func ScanSubnet(ctx context.Context, cidr string) ([]Result, error) {
	cidr = strings.TrimSpace(cidr)
	if cidr == "" {
		return nil, fmt.Errorf("subnet is required")
	}
	// Strictly validate the target so it can never be interpreted as an nmap
	// option (argument injection): accept only a CIDR, a single IP, or a simple
	// dashed range like 192.168.1.1-254. No leading '-', no spaces, no flags.
	if !isSafeScanTarget(cidr) {
		return nil, fmt.Errorf("invalid subnet %q (use CIDR like 192.168.1.0/24, an IP, or a.b.c.d-e range)", cidr)
	}
	path, err := exec.LookPath("nmap")
	if err != nil {
		return nil, fmt.Errorf("nmap not found in PATH: %w", err)
	}

	// "--" terminates option parsing so cidr can never be treated as an nmap
	// flag even if validation above is ever weakened (defense in depth).
	cmd := exec.CommandContext(ctx, path, "-Pn", "-sT", "-p", nmapPorts, "--open", "--", cidr)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("nmap scan failed: %w (stderr: %s)", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("nmap scan failed: %w", err)
	}
	return parseNmapOutput(string(out)), nil
}

// parseNmapOutput extracts one Result per (host, open camera-port) pair from
// nmap's default (-oN-style) stdout format:
//
//	Nmap scan report for 192.168.1.10
//	Host is up (0.00050s latency).
//
//	PORT      STATE SERVICE
//	80/tcp    open  http
//	8000/tcp  open  http-alt
func parseNmapOutput(output string) []Result {
	var results []Result
	currentIP := ""
	sc := bufio.NewScanner(strings.NewReader(output))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if m := nmapHostRe.FindStringSubmatch(line); m != nil {
			currentIP = m[1]
			continue
		}
		if currentIP == "" {
			continue
		}
		if m := nmapPortRe.FindStringSubmatch(line); m != nil {
			port, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			results = append(results, Result{
				IP:     currentIP,
				Port:   port,
				Vendor: vendorForPort(port),
				Via:    "nmap",
			})
		}
	}
	return results
}
