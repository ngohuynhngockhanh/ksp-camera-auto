package bidahub

import (
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// DefaultRedBidaKeyDir is where RedBida / ota-mqtt persists one file per config key on the box.
const DefaultRedBidaKeyDir = "/root/ota-mqtt/change_ok"

// redbidaIdentityKeys is the allow-list of RedBida keys the agent reports to the hub.
// Credentials (shinobi_token, shinobi_monitor_token, passwords) are never read.
var redbidaIdentityKeys = []string{
	"ui_title", "company_name", "hashtag", "custom_hashtags", "table_prefix",
	"shinobi_camera_id", "shinobi_group_key", "ggcode", "camera_count",
	"eth0_virtual_ip", "end0_virtual_ip", "wifi_ip", "qr_ip",
	"logo_header", "logo_header_text", "logo_livestream", "ui_bg",
	"cut_duration_sec", "playback_days", "video_format", "ntp_server",
	"max_free_ram_restart_camera", "auto_clean_disk_pct", "iptables_redirect_80_2023",
	"frpc_config",
}

const maxRedBidaValueBytes = 8 << 10

var reFRPCSecretLine = regexp.MustCompile(`(?im)^(\s*(?:token|meta_token|auth\.token|password|pass|user_pass|privilege_token)\s*=\s*).*$`)

// readRedBidaKeys reads the allow-listed RedBida keys present in dir. Missing keys are omitted;
// frpc_config has its credential lines redacted before leaving the box.
func readRedBidaKeys(dir string) map[string]string {
	out := make(map[string]string, len(redbidaIdentityKeys))
	for _, k := range redbidaIdentityKeys {
		raw, err := os.ReadFile(filepath.Join(dir, k))
		if err != nil {
			continue
		}
		if len(raw) > maxRedBidaValueBytes {
			raw = raw[:maxRedBidaValueBytes]
		}
		v := strings.TrimSpace(string(raw))
		if k == "frpc_config" {
			v = reFRPCSecretLine.ReplaceAllString(v, "${1}<redacted>")
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// sampleLANIP returns the source IPv4 the kernel picks for the default route (no packet is sent).
func sampleLANIP() string {
	conn, err := net.Dial("udp4", "1.1.1.1:53")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP != nil {
		return addr.IP.String()
	}
	return ""
}

// machineArch reports the uname-style architecture of this agent build.
func machineArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "aarch64"
	case "arm":
		return "armv7l"
	case "amd64":
		return "x86_64"
	default:
		return runtime.GOARCH
	}
}
