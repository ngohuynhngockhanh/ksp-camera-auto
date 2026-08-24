package traffic

import (
	"context"
	"math"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// FlowDirection indicates direction relative to local host.
type FlowDirection string

const (
	DirInbound  FlowDirection = "inbound"
	DirOutbound FlowDirection = "outbound"
	DirInternal FlowDirection = "internal"
)

// FlowEntry represents an active IP connection pair (source <=> destination).
type FlowEntry struct {
	SrcHost      string        `json:"srcHost"`
	SrcPort      int           `json:"srcPort"`
	SrcLabel     string        `json:"srcLabel,omitempty"`
	DstHost      string        `json:"dstHost"`
	DstPort      int           `json:"dstPort"`
	DstLabel     string        `json:"dstLabel,omitempty"`
	Protocol     string        `json:"protocol"`
	Service      string        `json:"service"`
	Direction    FlowDirection `json:"direction"`
	IsCamera     bool          `json:"isCamera"`
	CameraName   string        `json:"cameraName,omitempty"`

	// EWMA Bitrates (bits per second)
	Rate2sBps  float64 `json:"rate2sBps"`
	Rate10sBps float64 `json:"rate10sBps"`
	Rate40sBps float64 `json:"rate40sBps"`

	// Cumulative stats
	TotalBytes uint64 `json:"totalBytes"`
	LastActive string `json:"lastActive"`
}

// InterfaceStats contains overall interface counters and live rates.
type InterfaceStats struct {
	Name       string  `json:"name"`
	IP         string  `json:"ip,omitempty"`
	RXRate2s   float64 `json:"rxRate2sBps"`
	TXRate2s   float64 `json:"txRate2sBps"`
	RXTotal    uint64  `json:"rxTotalBytes"`
	TXTotal    uint64  `json:"txTotalBytes"`
	PeakRate   float64 `json:"peakRateBps"`
	ActiveFlows int     `json:"activeFlows"`
}

// Snapshot is the payload sent to SSE clients and MCP tools.
type Snapshot struct {
	Timestamp string         `json:"timestamp"`
	Interface string         `json:"interface"`
	Stats     InterfaceStats `json:"stats"`
	Flows     []FlowEntry    `json:"flows"`
}

// FlowKey identifies a unidirectional or bidirectional flow.
type flowKey struct {
	srcIP   string
	dstIP   string
	srcPort int
	dstPort int
	proto   string
}

type flowTracker struct {
	mu          sync.Mutex
	key         flowKey
	protocol    string
	totalBytes  uint64
	bytes2s     uint64
	bytes10s    uint64
	bytes40s    uint64
	rate2s      float64
	rate10s     float64
	rate40s     float64
	lastUpdated time.Time
	lastPacket  time.Time
}

// Manager coordinates raw socket capture sessions on demand.
type Manager struct {
	mu          sync.RWMutex
	inv         *config.Inventory
	trackers    map[string]*flowTracker
	subscribers map[chan Snapshot]struct{}
	activeIface string
	peakRate    float64
	rxTotal     uint64
	txTotal     uint64
	rxPeriod    uint64
	txPeriod    uint64
	rxRate2s    float64
	txRate2s    float64
	running     bool
	cancel      context.CancelFunc
}

// NewManager creates a new Traffic Manager.
func NewManager(inv *config.Inventory) *Manager {
	return &Manager{
		inv:         inv,
		trackers:    make(map[string]*flowTracker),
		subscribers: make(map[chan Snapshot]struct{}),
	}
}

// ListEligibleInterfaces returns physical/ethernet interfaces, excluding wlan* and lo.
func ListEligibleInterfaces() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var valid []string
	for _, ifi := range ifaces {
		name := strings.ToLower(ifi.Name)
		// Exclude wlan, wifi, loopback, docker bridge
		if strings.HasPrefix(name, "wlan") ||
			strings.HasPrefix(name, "wl") ||
			strings.HasPrefix(name, "lo") ||
			strings.HasPrefix(name, "docker") ||
			strings.HasPrefix(name, "br-") {
			continue
		}
		if ifi.Flags&net.FlagUp == 0 {
			continue
		}
		valid = append(valid, ifi.Name)
	}

	sort.Strings(valid)
	return valid, nil
}

// DetectDefaultInterface picks the primary ethernet interface (e.g. eth0, end0, enp*).
func DetectDefaultInterface() string {
	list, err := ListEligibleInterfaces()
	if err == nil && len(list) > 0 {
		for _, name := range list {
			if strings.HasPrefix(name, "eth") || strings.HasPrefix(name, "en") || strings.HasPrefix(name, "end") {
				return name
			}
		}
		return list[0]
	}
	return "eth0"
}

// Subscribe starts capturing if not already running and returns a channel of Snapshots.
func (m *Manager) Subscribe(ctx context.Context, iface string) (<-chan Snapshot, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if iface == "" {
		iface = DetectDefaultInterface()
	}

	ch := make(chan Snapshot, 10)
	m.subscribers[ch] = struct{}{}

	if !m.running {
		m.activeIface = iface
		m.startCaptureLocked()
	}

	unsubscribe := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.subscribers, ch)
		close(ch)
		if len(m.subscribers) == 0 && m.running {
			m.stopCaptureLocked()
		}
	}

	return ch, unsubscribe
}

// GetSnapshot returns a single point-in-time snapshot.
func (m *Manager) GetSnapshot(iface string) Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if iface == "" {
		iface = m.activeIface
		if iface == "" {
			iface = DetectDefaultInterface()
		}
	}

	return m.buildSnapshotLocked(iface)
}

func (m *Manager) startCaptureLocked() {
	if m.running {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.running = true
	m.trackers = make(map[string]*flowTracker)

	go m.sniffLoop(ctx, m.activeIface)
	go m.pulseLoop(ctx)
}

func (m *Manager) stopCaptureLocked() {
	if !m.running {
		return
	}
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.running = false
	m.trackers = make(map[string]*flowTracker)
}

// pulseLoop calculates EWMA rates and broadcasts snapshots every 1 second.
func (m *Manager) pulseLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// EWMA weights for dt = 1s:
	// alpha = 1 - exp(-dt / tau)
	a2 := 1.0 - math.Exp(-1.0/2.0)
	a10 := 1.0 - math.Exp(-1.0/10.0)
	a40 := 1.0 - math.Exp(-1.0/40.0)

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			m.mu.Lock()

			// Update interface rates (bps)
			rxInstBps := float64(m.rxPeriod*8) / 1.0
			txInstBps := float64(m.txPeriod*8) / 1.0
			m.rxRate2s = a2*rxInstBps + (1-a2)*m.rxRate2s
			m.txRate2s = a2*txInstBps + (1-a2)*m.txRate2s
			m.rxPeriod = 0
			m.txPeriod = 0

			totalRate := m.rxRate2s + m.txRate2s
			if totalRate > m.peakRate {
				m.peakRate = totalRate
			}

			// Decay & prune trackers
			cutoff := now.Add(-60 * time.Second)
			for k, tr := range m.trackers {
				tr.mu.Lock()
				if tr.lastPacket.Before(cutoff) && tr.rate2s < 100 && tr.rate10s < 100 {
					tr.mu.Unlock()
					delete(m.trackers, k)
					continue
				}

				instBps := float64(tr.bytes2s * 8)
				tr.rate2s = a2*instBps + (1-a2)*tr.rate2s
				tr.rate10s = a10*instBps + (1-a10)*tr.rate10s
				tr.rate40s = a40*instBps + (1-a40)*tr.rate40s
				tr.bytes2s = 0
				tr.lastUpdated = now
				tr.mu.Unlock()
			}

			snap := m.buildSnapshotLocked(m.activeIface)

			// Broadcast
			for ch := range m.subscribers {
				select {
				case ch <- snap:
				default:
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *Manager) buildSnapshotLocked(iface string) Snapshot {
	cams := m.inv.List()
	camMap := make(map[string]string) // IP -> Name
	for _, c := range cams {
		camMap[c.Host] = c.Name
	}

	var localIPs []string
	if ifi, err := net.InterfaceByName(iface); err == nil {
		if addrs, err := ifi.Addrs(); err == nil {
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
					localIPs = append(localIPs, ipnet.IP.String())
				}
			}
		}
	}

	isLocal := func(ip string) bool {
		for _, l := range localIPs {
			if ip == l {
				return true
			}
		}
		return strings.HasPrefix(ip, "127.")
	}

	var flows []FlowEntry
	for _, tr := range m.trackers {
		tr.mu.Lock()
		rate2s := tr.rate2s
		rate10s := tr.rate10s
		rate40s := tr.rate40s
		tot := tr.totalBytes
		k := tr.key
		proto := tr.protocol
		lastTime := tr.lastPacket
		tr.mu.Unlock()

		if rate2s < 50 && rate10s < 50 && rate40s < 50 && tot == 0 {
			continue
		}

		service := resolveService(k.srcPort, k.dstPort)
		camName := ""
		isCam := false
		if name, ok := camMap[k.srcIP]; ok {
			camName = name
			isCam = true
		} else if name, ok := camMap[k.dstIP]; ok {
			camName = name
			isCam = true
		}

		dir := DirInternal
		if isLocal(k.dstIP) && !isLocal(k.srcIP) {
			dir = DirInbound
		} else if isLocal(k.srcIP) && !isLocal(k.dstIP) {
			dir = DirOutbound
		}

		flows = append(flows, FlowEntry{
			SrcHost:     k.srcIP,
			SrcPort:     k.srcPort,
			DstHost:     k.dstIP,
			DstPort:     k.dstPort,
			Protocol:    proto,
			Service:     service,
			Direction:   dir,
			IsCamera:    isCam,
			CameraName:  camName,
			Rate2sBps:   rate2s,
			Rate10sBps:  rate10s,
			Rate40sBps:  rate40s,
			TotalBytes:  tot,
			LastActive:  lastTime.Format("15:04:05"),
		})
	}

	// Sort flows descending by 2s rate
	sort.Slice(flows, func(i, j int) bool {
		return flows[i].Rate2sBps > flows[j].Rate2sBps
	})

	localIPStr := ""
	if len(localIPs) > 0 {
		localIPStr = strings.Join(localIPs, ", ")
	}

	return Snapshot{
		Timestamp: time.Now().Format(time.RFC3339),
		Interface: iface,
		Stats: InterfaceStats{
			Name:        iface,
			IP:          localIPStr,
			RXRate2s:    m.rxRate2s,
			TXRate2s:    m.txRate2s,
			RXTotal:     m.rxTotal,
			TXTotal:     m.txTotal,
			PeakRate:    m.peakRate,
			ActiveFlows: len(flows),
		},
		Flows: flows,
	}
}

func resolveService(srcPort, dstPort int) string {
	checkPort := func(p int) string {
		switch p {
		case 554, 8554:
			return "RTSP (Video Stream)"
		case 80, 81, 8080, 8000:
			return "HTTP/ISAPI"
		case 1984:
			return "Go2RTC (API/WebRTC)"
		case 37777, 8888:
			return "Dahua DVRIP"
		case 1883, 12369:
			return "MQTT"
		case 2028:
			return "KSPCam Admin"
		case 2023:
			return "Node-RED"
		case 22:
			return "SSH"
		default:
			return ""
		}
	}

	if s := checkPort(dstPort); s != "" {
		return s
	}
	if s := checkPort(srcPort); s != "" {
		return s
	}
	return "TCP/UDP"
}
