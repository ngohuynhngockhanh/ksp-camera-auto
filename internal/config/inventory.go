package config

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

// Device is a persisted camera inventory entry, managed from the web UI.
type Device struct {
	ID       string `yaml:"id"`             // stable identifier (host:port)
	Name     string `yaml:"name,omitempty"` // human label
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Vendor   Vendor `yaml:"vendor"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// Addr returns host:port for dialling.
func (d Device) Addr() string { return fmt.Sprintf("%s:%d", d.Host, d.Port) }

// Inventory is a concurrency-safe, file-backed set of devices.
type Inventory struct {
	mu      sync.RWMutex
	path    string
	devices map[string]Device
}

// LoadInventory opens (or lazily creates) the inventory file at path.
func LoadInventory(path string) (*Inventory, error) {
	inv := &Inventory{path: path, devices: map[string]Device{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return inv, nil
		}
		return nil, fmt.Errorf("read inventory %s: %w", path, err)
	}
	var list []Device
	if err := yaml.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse inventory %s: %w", path, err)
	}
	for _, d := range list {
		if d.ID == "" {
			d.ID = d.Addr()
		}
		inv.devices[d.ID] = d
	}
	return inv, nil
}

// List returns all devices sorted by ID.
func (i *Inventory) List() []Device {
	i.mu.RLock()
	defer i.mu.RUnlock()
	out := make([]Device, 0, len(i.devices))
	for _, d := range i.devices {
		out = append(out, d)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

// Get returns a device by ID.
func (i *Inventory) Get(id string) (Device, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	d, ok := i.devices[id]
	return d, ok
}

// Upsert adds or replaces a device and persists the inventory.
func (i *Inventory) Upsert(d Device) error {
	if d.Host == "" || d.Port == 0 {
		return fmt.Errorf("device needs host and port")
	}
	if d.ID == "" {
		d.ID = d.Addr()
	}
	i.mu.Lock()
	i.devices[d.ID] = d
	i.mu.Unlock()
	return i.save()
}

// Delete removes a device by ID and persists the inventory.
func (i *Inventory) Delete(id string) error {
	i.mu.Lock()
	delete(i.devices, id)
	i.mu.Unlock()
	return i.save()
}

func (i *Inventory) save() error {
	if i.path == "" {
		return nil
	}
	list := i.List()
	data, err := yaml.Marshal(list)
	if err != nil {
		return fmt.Errorf("marshal inventory: %w", err)
	}
	tmp := i.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write inventory: %w", err)
	}
	return os.Rename(tmp, i.path)
}
