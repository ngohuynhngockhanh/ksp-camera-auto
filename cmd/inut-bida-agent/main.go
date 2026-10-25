// Command inut-bida-agent runs on each Bida edge box, maintaining a persistent
// outbound WebSocket + Yamux reverse tunnel to inut-bida-hub and pushing real-time
// kernel/process telemetry without SSH or VPN dependencies.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/bidahub"
)

func main() {
	hubURL := flag.String("hub", envOrDefault("INUT_BIDA_HUB_URL", "https://inut-bida.p2p.inut.io.vn"), "Central inut-bida-hub base URL")
	boxID := flag.String("box-id", envOrDefault("INUT_BOX_ID", ""), "Edge box ID (auto-detected from tinc.conf or hostname if empty)")
	secret := flag.String("secret", envOrDefault("INUT_BIDA_HUB_KEY", "inut-bida-hub-key-2026"), "Shared hub authentication key / box secret")
	interval := flag.Duration("interval", 20*time.Second, "Telemetry heartbeat push interval")
	kspcamPort := flag.Int("kspcam-port", 2028, "Local kspcam port")
	shinobiPort := flag.Int("shinobi-port", 8080, "Local Shinobi NVR port")
	noderedPort := flag.Int("nodered-port", 1880, "Local Node-RED port")
	once := flag.Bool("once", false, "Push one heartbeat and exit (for smoke testing)")
	flag.Parse()

	agent := bidahub.NewEdgeAgent(bidahub.AgentConfig{
		HubURL:            *hubURL,
		BoxID:             *boxID,
		Secret:            *secret,
		HeartbeatInterval: *interval,
		KspcamPort:        *kspcamPort,
		ShinobiPort:       *shinobiPort,
		NodeRedPort:       *noderedPort,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if *once {
		if err := agent.PushHeartbeatOnce(ctx); err != nil {
			log.Fatalf("inut-bida-agent heartbeat failed: %v", err)
		}
		log.Printf("inut-bida-agent heartbeat sent successfully")
		return
	}

	log.Printf("inut-bida-agent starting (hub=%s)", *hubURL)
	if err := agent.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("inut-bida-agent stopped: %v", err)
	}
}

func envOrDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
