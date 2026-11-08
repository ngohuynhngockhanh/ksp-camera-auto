package discovery

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/xiongmai"
)

// Xiongmai Sofia UDP broadcast discovery constants (UDP 34569, MsgID 1530 / 1531).
const (
	xmDiscoverUDPPort  = 34569
	xmDiscoverReqMsgID = 1530
	xmDiscoverRspMsgID = 1531
)

// scanXiongmai broadcasts a Sofia NetIP search packet (MsgID 1530) on UDP 34569
// to discover local L2 Xiongmai / XMEye / iCSee / Vitacam cameras and DVRs/NVRs.
func scanXiongmai(ctx context.Context, timeout time.Duration) ([]Result, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Build 20-byte Sofia header with MsgID 1530 (IPSearchReq) and empty body.
	var pkt [20]byte
	pkt[0] = 0xFF
	pkt[1] = 0x01
	binary.LittleEndian.PutUint16(pkt[14:16], xmDiscoverReqMsgID)

	bcast := &net.UDPAddr{IP: net.IPv4bcast, Port: xmDiscoverUDPPort}
	_, _ = conn.WriteTo(pkt[:], bcast)

	deadline := time.Now().Add(timeout)
	if cdl, ok := ctx.Deadline(); ok && cdl.Before(deadline) {
		deadline = cdl
	}
	_ = conn.SetReadDeadline(deadline)

	var results []Result
	buf := make([]byte, 8192)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			break
		}
		if n < 20 || buf[0] != 0xFF {
			continue
		}
		msgID := binary.LittleEndian.Uint16(buf[14:16])
		if msgID != xmDiscoverRspMsgID && msgID != xmDiscoverReqMsgID+1 {
			continue
		}
		body := bytes.TrimRight(buf[20:n], "\x00\r\n ")
		var env struct {
			NetCommon struct {
				HostIP   string `json:"HostIP"`
				HostName string `json:"HostName"`
				MAC      string `json:"MAC"`
				TCPPort  int    `json:"TCPPort"`
				HttpPort int    `json:"HttpPort"`
			} `json:"NetWork.NetCommon"`
		}
		if json.Unmarshal(body, &env) != nil {
			continue
		}
		ip, iperr := xiongmai.HexIPToDecIP(env.NetCommon.HostIP)
		if iperr != nil || ip == "" || ip == "0.0.0.0" {
			if ua, ok := addr.(*net.UDPAddr); ok && ua.IP != nil {
				ip = ua.IP.String()
			}
		}
		if ip == "" {
			continue
		}
		port := env.NetCommon.TCPPort
		if port <= 0 {
			port = xiongmai.DefaultSofiaPort
		}
		results = append(results, Result{
			IP:     ip,
			Port:   port,
			Vendor: "xiongmai",
			MAC:    strings.ToLower(strings.TrimSpace(env.NetCommon.MAC)),
			Name:   strings.TrimSpace(env.NetCommon.HostName),
			Via:    "xiongmai",
		})
	}
	return results, nil
}

// enrichPort80Xiongmai probes discovered port-80 hosts whose Vendor is empty
// against POST /cgi-bin/login.cgi {"Name":"GetPreLoginInfo"} so CGI-capable
// Xiongmai NVRs/HVRs are accurately labeled even if TCP 34567 was filtered.
func enrichPort80Xiongmai(ctx context.Context, results []Result) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range results {
		if results[i].Port != 80 || results[i].Vendor != "" || results[i].IP == "" {
			continue
		}
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			baseURL := fmt.Sprintf("http://%s:80", results[idx].IP)
			info, err := xiongmai.ProbePreLoginCGI(ctx, baseURL, 1500*time.Millisecond)
			if err != nil || info == nil {
				return
			}
			results[idx].Vendor = "xiongmai"
			if info.TCPPort > 0 {
				results[idx].Port = info.TCPPort
			}
		}()
	}
	wg.Wait()
}
