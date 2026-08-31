package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// PasswordBackupItem stores the backed up credentials of one camera.
type PasswordBackupItem struct {
	DeviceID   string        `json:"deviceId"`
	Name       string        `json:"name"`
	Host       string        `json:"host"`
	Port       int           `json:"port"`
	Vendor     config.Vendor `json:"vendor"`
	Username   string        `json:"username"`
	Password   string        `json:"password"`
	NVRChannel int           `json:"nvrChannel,omitempty"`
}

// PasswordBackup holds a timestamped snapshot of camera credentials before a batch change.
type PasswordBackup struct {
	Timestamp string               `json:"timestamp"`
	Items     []PasswordBackupItem `json:"items"`
}

func (s *Server) passwordBackupFilePath() string {
	invPath := s.inv.Path()
	if invPath == "" {
		return "password_backup.json"
	}
	return filepath.Join(filepath.Dir(invPath), "password_backup.json")
}

// savePasswordBackup saves current credentials of the specified devices before changing passwords.
func (s *Server) savePasswordBackup(devices []config.Device) error {
	if len(devices) == 0 {
		return nil
	}
	items := make([]PasswordBackupItem, 0, len(devices))
	for _, d := range devices {
		items = append(items, PasswordBackupItem{
			DeviceID:   d.ID,
			Name:       d.Name,
			Host:       d.Host,
			Port:       d.Port,
			Vendor:     d.Vendor,
			Username:   d.Username,
			Password:   d.Password,
			NVRChannel: d.NVRChannel,
		})
	}
	backup := PasswordBackup{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Items:     items,
	}
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal password backup: %w", err)
	}
	path := s.passwordBackupFilePath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write password backup: %w", err)
	}
	return os.Rename(tmp, path)
}

// loadPasswordBackup reads the latest password backup file.
func (s *Server) loadPasswordBackup() (*PasswordBackup, error) {
	path := s.passwordBackupFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var backup PasswordBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("parse password backup: %w", err)
	}
	return &backup, nil
}
