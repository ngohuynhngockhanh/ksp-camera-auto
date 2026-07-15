// Package discovery implements best-effort LAN camera discovery using three
// passive/active UDP protocols — ONVIF WS-Discovery, Dahua's DHDiscover, and
// Hikvision's SADP — plus an nmap-based active TCP port scan for subnets that
// aren't on the local broadcast domain.
//
// REALITY CHECK: UDP multicast/broadcast discovery (ONVIF, Dahua, SADP) only
// finds devices that are reachable via IP multicast/broadcast from the host
// running this tool — i.e. devices on the *same* L2 broadcast domain / VLAN.
// It does NOT traverse routers or NAT, so cameras on a different subnet or
// behind a router will never answer these probes, even if they are otherwise
// reachable over TCP. ScanSubnet (nmap) is the only method here that can
// reach an arbitrary routed subnet, at the cost of only inferring "there is
// something camera-shaped listening on a well-known port" rather than
// performing a full protocol handshake.
package discovery

import (
	"bytes"
	"context"
	"net"
	"sort"
	"sync"
	"time"
)

// Result is one discovered candidate camera. Fields are best-effort: a given
// discovery method may not be able to populate all of them.
type Result struct {
	IP     string `json:"ip"`
	Port   int    `json:"port,omitempty"`
	Vendor string `json:"vendor,omitempty"`
	Model  string `json:"model,omitempty"`
	MAC    string `json:"mac,omitempty"`
	Name   string `json:"name,omitempty"`
	// Via records which method found this entry: "onvif", "dahua",
	// "hikvision-sadp", or "nmap".
	Via string `json:"via"`
}

// score counts how many optional fields are populated, used to pick the
// "more informative" of two Results that resolved to the same IP.
func (r Result) score() int {
	n := 0
	if r.Vendor != "" {
		n++
	}
	if r.Model != "" {
		n++
	}
	if r.MAC != "" {
		n++
	}
	if r.Name != "" {
		n++
	}
	if r.Port != 0 {
		n++
	}
	return n
}

// Scan runs ONVIF WS-Discovery, Dahua DHDiscover, and Hikvision SADP probes
// concurrently on the local network, waits up to timeout for responses, and
// returns the merged/deduped results sorted by IP. An error from any single
// method never fails the whole scan — whatever was collected is returned.
func Scan(ctx context.Context, timeout time.Duration) ([]Result, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		all []Result
	)

	run := func(fn func(context.Context, time.Duration) ([]Result, error)) {
		defer wg.Done()
		res, err := fn(ctx, timeout)
		if err != nil {
			// Best-effort: a single method failing (e.g. no multicast-capable
			// interface, permission denied on broadcast) must not sink the
			// other methods' results.
			return
		}
		mu.Lock()
		all = append(all, res...)
		mu.Unlock()
	}

	wg.Add(3)
	go run(scanONVIF)
	go run(scanDahua)
	go run(scanSADP)
	wg.Wait()

	return mergeResults(all), nil
}

// mergeResults dedupes by IP, keeping the entry with the most populated
// fields for each address, and returns the result sorted by IP.
func mergeResults(in []Result) []Result {
	best := map[string]Result{}
	order := []string{}
	for _, r := range in {
		if r.IP == "" {
			continue
		}
		cur, ok := best[r.IP]
		if !ok {
			order = append(order, r.IP)
			best[r.IP] = r
			continue
		}
		if r.score() > cur.score() {
			best[r.IP] = r
		}
	}
	out := make([]Result, 0, len(order))
	for _, ip := range order {
		out = append(out, best[ip])
	}
	sort.Slice(out, func(i, j int) bool { return lessIP(out[i].IP, out[j].IP) })
	return out
}

// lessIP orders dotted-quad (or any parseable) IPs numerically; falls back to
// a plain string compare for anything net.ParseIP can't handle.
func lessIP(a, b string) bool {
	ipA, ipB := net.ParseIP(a), net.ParseIP(b)
	if ipA == nil || ipB == nil {
		return a < b
	}
	ipA, ipB = ipA.To16(), ipB.To16()
	return bytes.Compare(ipA, ipB) < 0
}
