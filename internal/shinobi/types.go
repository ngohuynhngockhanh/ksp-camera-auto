// Package shinobi provides a pure-Go REST API client and bi-directional sync
// engine for Shinobi NVR management.
package shinobi

import (
	"encoding/json"
	"strings"
	"time"
)

// MonitorConfig represents the full monitor configuration payload sent to Shinobi.
type MonitorConfig struct {
	Mid      string         `json:"mid"`
	Ke       string         `json:"ke,omitempty"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`     // "h264", "mjpeg", "flv", "mp4"
	Mode     string         `json:"mode"`     // "start", "stop", "record", "idle"
	Host     string         `json:"host"`     // Camera IP / hostname
	Port     string         `json:"port"`     // RTSP port (typically "554")
	Protocol string         `json:"protocol"` // "rtsp", "http", "https"
	Path     string         `json:"path"`     // RTSP path e.g. "/cam/realmonitor?channel=1&subtype=0"
	Ext      string         `json:"ext"`      // "mp4", "webm"
	FPS      string         `json:"fps,omitempty"`
	Width    string         `json:"width,omitempty"`
	Height   string         `json:"height,omitempty"`
	Details  MonitorDetails `json:"details"`
}

// MonitorDetails contains stream parameters, codecs, and credentials for a Shinobi monitor.
type MonitorDetails struct {
	AutoHost           string `json:"auto_host"`                      // e.g. "rtsp://user:pass@host:554/path"
	Muser              string `json:"muser,omitempty"`                // RTSP username
	Mpass              string `json:"mpass,omitempty"`                // RTSP password
	Port               string `json:"port,omitempty"`                 // RTSP port ("554")
	Protocol           string `json:"protocol,omitempty"`             // "rtsp"
	StreamType         string `json:"stream_type,omitempty"`          // "mp4", "h264", "flv"
	StreamFlvType      string `json:"stream_flv_type,omitempty"`      // "ws", "http"
	StreamMjpegClients string `json:"stream_mjpeg_clients,omitempty"` // "" or "1"
	StreamVcodec       string `json:"stream_vcodec,omitempty"`        // "copy"
	StreamAcodec       string `json:"stream_acodec,omitempty"`        // "copy", "no"
	Vcodec             string `json:"vcodec,omitempty"`               // "copy" (0% CPU transcoding)
	Acodec             string `json:"acodec,omitempty"`               // "copy", "aac", "no"
	RecordVcodec       string `json:"record_vcodec,omitempty"`        // "copy"
	RecordAcodec       string `json:"record_acodec,omitempty"`        // "aac", "copy", "no"
	CustInput          string `json:"cust_input"`                     // "" (trống, không dùng custom input flags)
	CustStream         string `json:"cust_stream"`                    // "" (trống, không dùng custom stream flags)
	CustRecord         string `json:"cust_record"`                    // "-tag:v hvc1" cho chuẩn H.265 MP4 recording
	Detector           string `json:"detector,omitempty"`             // "0"
	CustomRTSP         string `json:"custom_rtsp,omitempty"`
}

// FlexibleString handles JSON fields that can be encoded as either a string, a number, or null.
type FlexibleString string

// UnmarshalJSON parses a JSON string, number, or null into FlexibleString.
func (f *FlexibleString) UnmarshalJSON(b []byte) error {
	trimmed := strings.TrimSpace(string(b))
	if len(trimmed) == 0 || trimmed == "null" {
		*f = ""
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal([]byte(trimmed), &s); err != nil {
			return err
		}
		*f = FlexibleString(s)
		return nil
	}
	*f = FlexibleString(trimmed)
	return nil
}

// String returns the string representation of FlexibleString.
func (f FlexibleString) String() string {
	return string(f)
}

// Monitor represents a monitor object returned by the Shinobi REST API.
// In live Shinobi instances, Details is often a JSON-escaped string ("details":"{...}"),
// whereas exported configurations may provide it as a nested JSON object.
type Monitor struct {
	Mid      string          `json:"mid"`
	Ke       string          `json:"ke"`
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Mode     string          `json:"mode"`
	Host     string          `json:"host"`
	Port     FlexibleString  `json:"port"`
	Protocol string          `json:"protocol"`
	Path     string          `json:"path"`
	Ext      string          `json:"ext"`
	FPS      FlexibleString  `json:"fps"`
	Width    FlexibleString  `json:"width"`
	Height   FlexibleString  `json:"height"`
	Details  json.RawMessage `json:"details"`
}

// ParseDetails unmarshals the monitor's Details field whether it was serialized
// as a JSON object or an escaped JSON string.
func (m *Monitor) ParseDetails() MonitorDetails {
	var d MonitorDetails
	raw := m.Details
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return d
	}
	if raw[0] == '"' { // stringified JSON
		var s string
		if json.Unmarshal(raw, &s) == nil {
			_ = json.Unmarshal([]byte(s), &d)
		}
		return d
	}
	_ = json.Unmarshal(raw, &d)
	return d
}

// Video represents a recording clip metadata entry from Shinobi.
type Video struct {
	Mid      string    `json:"mid"`
	Ke       string    `json:"ke"`
	Time     time.Time `json:"time"`
	End      time.Time `json:"end"`
	Ext      string    `json:"ext"`
	Size     int64     `json:"size"`
	Href     string    `json:"href"`
	Filename string    `json:"filename"`
	Status   int       `json:"status"`
}

// SyncReport summarizes the results of a manual sync operation between
// ksp-camera-auto inventory and Shinobi monitors.
type SyncReport struct {
	Direction string   `json:"direction"` // "to_shinobi" or "from_shinobi"
	Created   int      `json:"created"`   // monitors added to Shinobi
	Updated   int      `json:"updated"`   // monitors updated on Shinobi
	Unchanged int      `json:"unchanged"` // monitors already up to date on Shinobi
	Added     int      `json:"added"`     // devices added to inventory
	Skipped   int      `json:"skipped"`   // devices skipped / already present in inventory
	Errors    []string `json:"errors"`    // non-fatal errors encountered
}

// ShinobiStatus reports connection and configuration state for Shinobi.
type ShinobiStatus struct {
	Configured   bool   `json:"configured"`
	Connected    bool   `json:"connected"`
	APIURL       string `json:"apiUrl"`
	GroupKey     string `json:"groupKey"`
	MonitorCount int    `json:"monitorCount"`
	Error        string `json:"error,omitempty"`
}
