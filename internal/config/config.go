// Package config loads and persists the tool's runtime configuration and the
// camera inventory. Configuration is YAML; every field has a safe default so a
// missing or partial file still yields a working setup.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"

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

// ShinobiConfig holds connection parameters for the Shinobi NVR REST API.
type ShinobiConfig struct {
	Enabled  bool   `yaml:"enabled"`
	APIURL   string `yaml:"api_url"`   // Base URL e.g. "http://127.0.0.1:8080"
	APIKey   string `yaml:"api_key"`   // 30-character API key
	GroupKey string `yaml:"group_key"` // Shinobi Group Key (ke)
}

func (s *ShinobiConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawShinobi ShinobiConfig
	var raw struct {
		rawShinobi `yaml:",inline"`
		URL        string `yaml:"url"`
		Key        string `yaml:"apiKey"`
		Group      string `yaml:"groupKey"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*s = ShinobiConfig(raw.rawShinobi)
	if s.APIURL == "" && raw.URL != "" {
		s.APIURL = raw.URL
	}
	if s.APIKey == "" && raw.Key != "" {
		s.APIKey = raw.Key
	}
	if s.GroupKey == "" && raw.Group != "" {
		s.GroupKey = raw.Group
	}
	return nil
}

// MCPConfig holds configuration for the embedded Model Context Protocol (MCP) server.
type MCPConfig struct {
	Enabled                      bool   `yaml:"enabled"`
	APIKey                       string `yaml:"api_key"`
	AllowUnauthenticatedLoopback bool   `yaml:"allow_unauthenticated_loopback"`
}

// RedbidaConfig describes the local OTA-MQTT bridge used by the RedBida
// settings console. Node-RED remains a read-only survey target; writes travel
// through ota-mqtt's private MQTT topics.
type RedbidaConfig struct {
	Enabled        bool   `yaml:"enabled"`
	BrokerHost     string `yaml:"broker_host"`
	BrokerPort     int    `yaml:"broker_port"`
	ReadTopic      string `yaml:"read_topic"`
	ReadAckTopic   string `yaml:"read_ack_topic"`
	WriteTopic     string `yaml:"write_topic"`
	WriteAckTopic  string `yaml:"write_ack_topic"`
	KeyDir         string `yaml:"key_dir"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	MaxBatchKeys   int    `yaml:"max_batch_keys"`
}

func (r *RedbidaConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawRedbida RedbidaConfig
	var raw struct {
		rawRedbida `yaml:",inline"`
		Broker     string `yaml:"broker"`
		CatalogDir string `yaml:"catalog_dir"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*r = RedbidaConfig(raw.rawRedbida)
	if raw.Broker != "" {
		if host, portStr, err := net.SplitHostPort(raw.Broker); err == nil {
			r.BrokerHost = host
			if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
				r.BrokerPort = p
			}
		} else {
			r.BrokerHost = raw.Broker
		}
	}
	if r.KeyDir == "" && raw.CatalogDir != "" {
		r.KeyDir = raw.CatalogDir
	}
	return nil
}

// Config is the top-level configuration document.
type Config struct {
	Server      Server        `yaml:"server"`
	CamerasFile string        `yaml:"cameras_file"`
	Defaults    Defaults      `yaml:"defaults"`
	Shinobi     ShinobiConfig `yaml:"shinobi"`
	MCP         MCPConfig     `yaml:"mcp"`
	Redbida     RedbidaConfig `yaml:"redbida"`
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
		Shinobi: ShinobiConfig{
			APIURL: "http://127.0.0.1:8080",
		},
		MCP: MCPConfig{
			Enabled:                      true,
			AllowUnauthenticatedLoopback: true,
		},
		Redbida: RedbidaConfig{
			Enabled:        false,
			BrokerHost:     "127.0.0.1",
			BrokerPort:     12369,
			ReadTopic:      "/private/i_gets",
			ReadAckTopic:   "/private/i_gets/ack",
			WriteTopic:     "/private/i_sets",
			WriteAckTopic:  "/private/i_sets/ack",
			KeyDir:         "/root/ota-mqtt/change_ok",
			TimeoutSeconds: 10,
			MaxBatchKeys:   200,
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
	if c.Shinobi.APIURL == "" {
		c.Shinobi.APIURL = d.Shinobi.APIURL
	}
	if c.Redbida.BrokerHost == "" {
		c.Redbida.BrokerHost = d.Redbida.BrokerHost
	}
	if c.Redbida.BrokerPort == 0 {
		c.Redbida.BrokerPort = d.Redbida.BrokerPort
	}
	if c.Redbida.ReadTopic == "" {
		c.Redbida.ReadTopic = d.Redbida.ReadTopic
	}
	if c.Redbida.ReadAckTopic == "" {
		c.Redbida.ReadAckTopic = d.Redbida.ReadAckTopic
	}
	if c.Redbida.WriteTopic == "" {
		c.Redbida.WriteTopic = d.Redbida.WriteTopic
	}
	if c.Redbida.WriteAckTopic == "" {
		c.Redbida.WriteAckTopic = d.Redbida.WriteAckTopic
	}
	if c.Redbida.KeyDir == "" {
		c.Redbida.KeyDir = d.Redbida.KeyDir
	}
	if c.Redbida.TimeoutSeconds == 0 {
		c.Redbida.TimeoutSeconds = d.Redbida.TimeoutSeconds
	}
	if c.Redbida.MaxBatchKeys == 0 {
		c.Redbida.MaxBatchKeys = d.Redbida.MaxBatchKeys
	}
}
