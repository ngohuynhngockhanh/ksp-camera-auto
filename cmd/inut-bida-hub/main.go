package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/bidahub"
)

var version = "1.0.0"

func main() {
	var (
		port          = flag.Int("port", 2090, "HTTP listen port")
		bindAddr      = flag.String("bind", "0.0.0.0", "HTTP bind address")
		dataPath      = flag.String("data", "/opt/inut-bida-hub/data/hub-state.json", "Path to persistent hub state file")
		dbAlias       = flag.String("db", "", "Alias for -data (systemd unit compatibility)")
		watchlistPath = flag.String("watchlist", "FLEET_WATCHLIST.md", "Path to FLEET_WATCHLIST.md")
		adminUser     = flag.String("admin-user", "", "Admin username (overrides INUT_BIDA_ADMIN_USER)")
		adminPass     = flag.String("admin-pass", "", "Admin seed password (overrides INUT_BIDA_ADMIN_PASS)")
		adminEmail    = flag.String("admin-email", "", "Admin recovery email (overrides INUT_BIDA_ADMIN_EMAIL, default khanhnhn@inut.vn)")
		mailAPIURL    = flag.String("mail-api-url", "", "ksp-pdfsign HTTP Mail API URL (overrides INUT_BIDA_SMTP_API_URL)")
		mailAPIKey    = flag.String("mail-api-key", "", "ksp-pdfsign HTTP Mail API Key (overrides INUT_BIDA_SMTP_API_KEY)")
		apiKey        = flag.String("api-key", "", "Hub API key (overrides INUT_BIDA_API_KEY)")
		mqttBroker    = flag.String("mqtt-broker", "", "MQTT broker for AI/alarm event subscription (e.g. tcp://127.0.0.1:1883)")
		mqttUser      = flag.String("mqtt-user", "", "MQTT username")
		mqttPass      = flag.String("mqtt-pass", "", "MQTT password")
		mqttTopic     = flag.String("mqtt-topic", "kspcam/events/#", "MQTT subscription topic filter")
		showVersion   = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("inut-bida-hub %s\n", version)
		return
	}

	resolvedData := *dataPath
	if *dbAlias != "" {
		resolvedData = *dbAlias
	}

	cfg := bidahub.Config{
		BindAddr:      *bindAddr,
		Port:          *port,
		DataPath:      resolvedData,
		WatchlistPath: *watchlistPath,
		AdminUser:     *adminUser,
		AdminPass:     *adminPass,
		AdminEmail:    *adminEmail,
		MailAPIURL:    *mailAPIURL,
		MailAPIKey:    *mailAPIKey,
		APIKey:        *apiKey,
		MQTTBroker:    *mqttBroker,
		MQTTUser:      *mqttUser,
		MQTTPass:      *mqttPass,
		MQTTTopic:     *mqttTopic,
		PollInterval:  30 * time.Second,
		DialTimeout:   1500 * time.Millisecond,
	}

	srv, err := bidahub.NewServer(cfg)
	if err != nil {
		// Fallback to local ./data/hub-state.json if /opt/inut-bida-hub is not writable by non-root user
		if resolvedData == "/opt/inut-bida-hub/data/hub-state.json" {
			cfg.DataPath = "./data/hub-state.json"
			srv, err = bidahub.NewServer(cfg)
		}
		if err != nil {
			log.Fatalf("inut-bida-hub init failed: %v", err)
		}
	}
	cfg.AdminPass = ""
	*adminPass = ""

	listenAddr := net.JoinHostPort(cfg.BindAddr, strconv.Itoa(cfg.Port))
	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		kpi := srv.Store().ComputeKPI()
		log.Printf("inut-bida-hub %s listening on http://%s (clubs=%d, boxes=%d, state=%s)",
			version, listenAddr, kpi.ClubsTotal, kpi.BoxesTotal, cfg.DataPath)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http listen error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down inut-bida-hub...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutCtx)
	_ = srv.Close()
}
