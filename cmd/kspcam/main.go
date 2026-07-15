// Command kspcam runs the ksp-camera-auto web tool: a single static binary that
// serves a web UI (default :2028) for bulk-configuring Hikvision and
// Dahua/KBVision IP cameras.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/server"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	addr := flag.String("addr", "", "override listen address (e.g. :2028)")
	showVersion := flag.Bool("version", false, "print version and exit")
	hashPassword := flag.String("hash-password", "", "print a bcrypt hash for the given web-login password and exit")
	flag.Parse()

	if *showVersion {
		log.Printf("kspcam %s", version)
		return
	}
	if *hashPassword != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(*hashPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("hash: %v", err)
		}
		fmt.Println(string(h))
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *addr != "" {
		cfg.Server.Addr = *addr
	}

	inv, err := config.LoadInventory(cfg.CamerasFile)
	if err != nil {
		log.Fatalf("inventory: %v", err)
	}

	srv, err := server.New(cfg, inv)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("kspcam %s listening on %s (login: %s)", version, cfg.Server.Addr, cfg.Server.Username)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
