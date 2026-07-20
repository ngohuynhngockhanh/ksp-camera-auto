// Package config loads and persists the tool's runtime configuration and the
// camera inventory. Configuration is YAML; every field has a safe default so a
// missing or partial file still yields a working setup.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Vendor identifies a supported camera family.
type Vendor string

const (
	VendorHikvision Vendor = "hikvision"
	VendorDahua     Vendor = "dahua"  // covers KBVision (Dahua OEM)
	VendorTiandy    Vendor = "tiandy" // Dahua-lineage RTSP; review-only over RTSP+ONVIF (pure-Go, no NetSDK)
)

// Server holds web UI listener + login settings.
type Server struct {
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// PasswordHash, if set, is a bcrypt hash checked instead of Password
	// (generate with `kspcam --hash-password <pw>`).
	PasswordHash string `yaml:"password_hash"`
	// Viewer is a read-only login that may only use the "Xem lại" review view
	// (list/play/download recordings). Defaults to viewer/inut12345 when unset.
	ViewerUsername string `yaml:"viewer_username"`
	ViewerPassword string `yaml:"viewer_password"`
	// LoginMaxAttempts is how many consecutive failed logins from one IP
	// trigger a lockout.
	LoginMaxAttempts int `yaml:"login_max_attempts"`
	// LoginLockoutMinutes is how long a locked-out IP is blocked for, reset
	// on every further failed attempt while still locked (sliding window).
	LoginLockoutMinutes int `yaml:"login_lockout_minutes"`
}

// Defaults are applied to camera entries that omit a field.
type Defaults struct {
	HikvisionPort int `yaml:"hikvision_port"`
	DahuaPort     int `yaml:"dahua_port"`
	// TiandyPort is the primary/control port stored for a Tiandy device. Tiandy
	// playback rides RTSP (always :554) and IP-config rides ONVIF (:8082); this
	// default (554) is what a bare Tiandy entry gets when a port is omitted.
	TiandyPort int    `yaml:"tiandy_port"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	// TimeoutSeconds bounds one device operation; higher helps slow multi-channel
	// NVRs. The web UI can override it per request.
	TimeoutSeconds int `yaml:"timeout_seconds"`
	// NewPassword is the default when bulk-changing a camera's password.
	NewPassword string `yaml:"new_password"`
	// MaxReviewHours caps the length of a recording range the "Xem lại" view can
	// play/download (guards against absurdly long requests). Default 72.
	MaxReviewHours int `yaml:"max_review_hours"`
}

// Config is the top-level configuration document.
type Config struct {
	Server      Server   `yaml:"server"`
	CamerasFile string   `yaml:"cameras_file"`
	Defaults    Defaults `yaml:"defaults"`
}

// Default returns a Config populated with built-in defaults.
func Default() Config {
	return Config{
		Server: Server{
			Addr:                ":2028",
			Username:            "admin",
			Password:            "smarthome12345",
			LoginMaxAttempts:    5,
			LoginLockoutMinutes: 30,
		},
		CamerasFile: "cameras.yaml",
		Defaults: Defaults{
			HikvisionPort:  8000,
			DahuaPort:      37777,
			TiandyPort:     554,
			Username:       "admin",
			Password:       "smarthome12345",
			TimeoutSeconds: 30,
			NewPassword:    "smarthome12345",
			MaxReviewHours: 72,
		},
	}
}

// Load reads config from path, filling any unset field with its default. A
// missing file is not an error: defaults are returned so the tool still starts.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.applyDefaults()
	return cfg, nil
}

// applyDefaults backfills zero-valued fields after unmarshalling a partial file.
func (c *Config) applyDefaults() {
	d := Default()
	if c.Server.Addr == "" {
		c.Server.Addr = d.Server.Addr
	}
	if c.Server.Username == "" {
		c.Server.Username = d.Server.Username
	}
	if c.Server.Password == "" {
		c.Server.Password = d.Server.Password
	}
	if c.Server.LoginMaxAttempts == 0 {
		c.Server.LoginMaxAttempts = d.Server.LoginMaxAttempts
	}
	if c.Server.LoginLockoutMinutes == 0 {
		c.Server.LoginLockoutMinutes = d.Server.LoginLockoutMinutes
	}
	if c.CamerasFile == "" {
		c.CamerasFile = d.CamerasFile
	}
	if c.Defaults.HikvisionPort == 0 {
		c.Defaults.HikvisionPort = d.Defaults.HikvisionPort
	}
	if c.Defaults.DahuaPort == 0 {
		c.Defaults.DahuaPort = d.Defaults.DahuaPort
	}
	if c.Defaults.TiandyPort == 0 {
		c.Defaults.TiandyPort = d.Defaults.TiandyPort
	}
	if c.Defaults.Username == "" {
		c.Defaults.Username = d.Defaults.Username
	}
	if c.Defaults.TimeoutSeconds == 0 {
		c.Defaults.TimeoutSeconds = d.Defaults.TimeoutSeconds
	}
	if c.Defaults.MaxReviewHours == 0 {
		c.Defaults.MaxReviewHours = d.Defaults.MaxReviewHours
	}
	if c.Defaults.NewPassword == "" {
		c.Defaults.NewPassword = d.Defaults.NewPassword
	}
	if c.Defaults.Password == "" {
		c.Defaults.Password = d.Defaults.Password
	}
}
