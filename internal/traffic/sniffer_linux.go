//go:build linux

package traffic

import (
	"context"
	"fmt"
	"log"
	"net"
	"syscall"
	"time"
)

func htons(v uint16) uint16 {
	return (v << 8) | (v >> 8)
}

func (m *Manager) sniffLoop(ctx context.Context, ifaceName string) {
	// Open raw AF_PACKET socket (captures IP packets)
	// 0x0800 is ETH_P_IP in host order, htons(0x0800) is 0x0008 in network order on little-endian
	ethPIP := htons(0x0800)
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(ethPIP))
	if err != nil {
		log.Printf("[Traffic] Raw socket error (running as non-root or unsupported): %v", err)
		return
	}
	defer syscall.Close(fd)

	// Bind to specific interface if specified
	if ifaceName != "" {
		if ifi, err := net.InterfaceByName(ifaceName); err == nil {
			sll := syscall.SockaddrLinklayer{
				Protocol: ethPIP,
				Ifindex:  ifi.Index,
			}
			_ = syscall.Bind(fd, &sll)
		}
	}

	// Set non-blocking / read timeout with poll or cancel channel
	_ = syscall.SetNonblock(fd, true)

	buf := make([]byte, 65536)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}

		if n < 34 { // Min ethernet (14) + IPv4 (20)
			continue
		}

		// Check Ethernet header (14 bytes)
		etherType := (uint16(buf[12]) << 8) | uint16(buf[13])
		if etherType != 0x0800 { // Only IPv4
			continue
		}

		// IPv4 Header
		ipHeader := buf[14:]
		version := ipHeader[0] >> 4
		if version != 4 {
			continue
		}
		ihl := int(ipHeader[0]&0x0f) * 4
		if ihl < 20 || n < 14+ihl {
			continue
		}

		totalLen := int((uint16(ipHeader[2]) << 8) | uint16(ipHeader[3]))
		if totalLen == 0 || totalLen > n-14 {
			totalLen = n - 14
		}

		protoNum := ipHeader[9]
		protoStr := "OTHER"
		if protoNum == 6 {
			protoStr = "TCP"
		} else if protoNum == 17 {
			protoStr = "UDP"
		} else {
			continue
		}

		srcIP := net.IPv4(ipHeader[12], ipHeader[13], ipHeader[14], ipHeader[15]).String()
		dstIP := net.IPv4(ipHeader[16], ipHeader[17], ipHeader[18], ipHeader[19]).String()

		transportHeader := ipHeader[ihl:]
		if len(transportHeader) < 4 {
			continue
		}

		srcPort := int((uint16(transportHeader[0]) << 8) | uint16(transportHeader[1]))
		dstPort := int((uint16(transportHeader[2]) << 8) | uint16(transportHeader[3]))

		// Update stats in memory
		m.recordPacket(srcIP, dstIP, srcPort, dstPort, protoStr, uint64(totalLen))
	}
}

func (m *Manager) recordPacket(srcIP, dstIP string, srcPort, dstPort int, proto string, pktLen uint64) {
	keyStr := fmt.Sprintf("%s:%d->%s:%d/%s", srcIP, srcPort, dstIP, dstPort, proto)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update interface totals
	m.rxTotal += pktLen
	m.rxPeriod += pktLen

	tr, exists := m.trackers[keyStr]
	if !exists {
		tr = &flowTracker{
			key: flowKey{
				srcIP:   srcIP,
				dstIP:   dstIP,
				srcPort: srcPort,
				dstPort: dstPort,
				proto:   proto,
			},
			protocol: proto,
		}
		m.trackers[keyStr] = tr
	}

	tr.mu.Lock()
	tr.totalBytes += pktLen
	tr.bytes2s += pktLen
	tr.lastPacket = time.Now()
	tr.mu.Unlock()
}
