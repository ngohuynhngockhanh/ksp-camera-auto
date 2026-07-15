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
	VendorDahua     Vendor = "dahua" // covers KBVision (Dahua OEM)
)

// Server holds web UI listener + login settings.
type Server struct {
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// PasswordHash, if set, is a bcrypt hash checked instead of Password
	// (generate with `kspcam --hash-password <pw>`).
	PasswordHash string `yaml:"password_hash"`
}

// Defaults are applied to camera entries that omit a field.
type Defaults struct {
	HikvisionPort int    `yaml:"hikvision_port"`
	DahuaPort     int    `yaml:"dahua_port"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
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
			Addr:     ":2028",
			Username: "admin",
			Password: "inut12345",
		},
		CamerasFile: "cameras.yaml",
		Defaults: Defaults{
			HikvisionPort: 8000,
			DahuaPort:     37777,
			Username:      "admin",
			Password:      "inut12345",
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
	if c.CamerasFile == "" {
		c.CamerasFile = d.CamerasFile
	}
	if c.Defaults.HikvisionPort == 0 {
		c.Defaults.HikvisionPort = d.Defaults.HikvisionPort
	}
	if c.Defaults.DahuaPort == 0 {
		c.Defaults.DahuaPort = d.Defaults.DahuaPort
	}
	if c.Defaults.Username == "" {
		c.Defaults.Username = d.Defaults.Username
	}
	if c.Defaults.Password == "" {
		c.Defaults.Password = d.Defaults.Password
	}
}
