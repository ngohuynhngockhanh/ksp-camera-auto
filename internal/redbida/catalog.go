package redbida

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var sensitiveKeyRe = regexp.MustCompile(`(?i)(password|token|secret|username|mqtt_|shinobi_|sapo_url|md5_node_id|node_info|blacklist_keys|hidden_keys|config_no_use|frpc_config|gortc_default_config|apiRecentInput_string|ggcode|api_key|access_key|private_key|credential|cookie|s3-storage)`)
var protectedKeyRe = regexp.MustCompile(`(?i)(^|_)(ip|route|gateway|dns|frpc|broker|port|virtual_ip|static_|wifi_|valid_license|inut_id)`)
var runtimeKeyRe = regexp.MustCompile(`(?i)(^download_count$|^packed_count$|^view_count$|^node_config_|^test_button$)`)
var validKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// These keys were observed on inut_204_63. Directory discovery remains the
// source of truth; this list is only a fallback during temporary outages.
var fallbackKeys = strings.Fields(`
apiRecentInput_string api_count api_model_count app_background app_hide_inut
app_hotline app_website at_least_virtual_ip_retry backup_ip backup_offset
banner_top blacklist_keys button_generate_go2rtc_stream button_reboot
button_restart_shinobi camera_count company_name config_no_use custom_hashtags
db_check_range db_check_rmlm db_check_size_lm default_delay_camera
default_delay_go2rtc default_live_description default_tiso_1_color
default_tiso_2_color default_tiso_3_color default_tiso_4_color default_tiso_type
disable_cut_realtime disable_logo_cat_cam disable_logo_header
disable_reboot_camera_at_4am disable_update_logo_livestream download_count
enable_hardware_reboot_camera_at_4am enable_hardware_reboot_camera_at_6am
enable_reboot_camera_at_6am end0_static_route end0_static_route_gw
end0_virtual_ip eth0_static_route eth0_static_route_gw eth0_virtual_ip
force_google_dns fps_default frpc_config ggcode gortc_default_config help_link
hidden_keys hls_using_go2rtc hls_using_go2rtc_livestream hls_using_go2rtc_tiktok
inut_id lan0_static_route lan0_static_route_gw lan0_virtual_ip language
large_monitor live_static_password livestream_default_bitrate logo_cat_cam
logo_header logo_header_background logo_header_text logo_livestream
max_free_ram_force_reboot max_free_ram_force_restart_camera
max_free_ram_restart_camera max_shared_ram_camera md5_node_id
mode_wifi_same_eth0 mqtt_broker mqtt_password mqtt_port mqtt_username
node_config_tab_index node_config_tab_password node_info packed_count
place_livestream realtime_shop_id s3-storage sapo_url shinobi_camera_id
shinobi_group_key shinobi_monitor_token shinobi_offset shinobi_token shop_id
show_toolbar stop_camera_00h05 stop_camera_00h30 stop_camera_01h00
stop_camera_03h00 stop_camera_03h30 stop_camera_22h30 stop_camera_23h00
stop_camera_23h30 support_live_password support_view_password test_button
toolbar_show_count ui_bg ui_css_custom ui_download_text ui_fb ui_google ui_phone
ui_scoreboard ui_tabs_links ui_tiktok ui_title ui_title_color ui_zalo
url_live_help usb_lan_gateway_ip usb_lan_static_ip usb_lan_subnet valid_license
video_config view_count view_password_help view_static_password
watch_uptime_process wifi_same_eth0_timeout wlan0_static_route
wlan0_static_route_gw wlan0_virtual_ip
`)

var fallbackKeySet = keySet(strings.Join(fallbackKeys, " "))
var editableKeySet = keySet(`
api_count api_model_count app_background app_hide_inut app_hotline app_website banner_top camera_count
company_name custom_hashtags db_check_range db_check_rmlm db_check_size_lm
default_delay_camera default_delay_go2rtc default_live_description
default_tiso_1_color default_tiso_2_color default_tiso_3_color
default_tiso_4_color default_tiso_type disable_cut_realtime disable_logo_cat_cam
disable_logo_header fps_default help_link hls_using_go2rtc
hls_using_go2rtc_livestream hls_using_go2rtc_tiktok language large_monitor
livestream_default_bitrate logo_cat_cam logo_header logo_header_background
logo_header_text logo_livestream max_free_ram_force_reboot
max_free_ram_force_restart_camera max_free_ram_restart_camera
max_shared_ram_camera place_livestream show_toolbar toolbar_show_count ui_bg
ui_css_custom ui_download_text ui_fb ui_google ui_phone ui_scoreboard
ui_tabs_links ui_tiktok ui_title ui_title_color ui_zalo url_live_help
video_config
`)
var confirmEditableKeySet = keySet(`
	button_generate_go2rtc_stream button_reboot button_restart_shinobi
	disable_reboot_camera_at_4am disable_update_logo_livestream
	enable_hardware_reboot_camera_at_4am enable_hardware_reboot_camera_at_6am
	enable_reboot_camera_at_6am force_google_dns max_free_ram_force_reboot
	max_free_ram_force_restart_camera max_free_ram_restart_camera
	max_shared_ram_camera stop_camera_00h05
stop_camera_00h30 stop_camera_01h00 stop_camera_03h00 stop_camera_03h30
stop_camera_22h30 stop_camera_23h00 stop_camera_23h30
`)
var booleanKeySet = keySet(`
app_hide_inut button_generate_go2rtc_stream button_reboot button_restart_shinobi
disable_cut_realtime disable_logo_cat_cam disable_logo_header
disable_reboot_camera_at_4am disable_update_logo_livestream
enable_hardware_reboot_camera_at_4am enable_hardware_reboot_camera_at_6am
enable_reboot_camera_at_6am force_google_dns hls_using_go2rtc
hls_using_go2rtc_livestream hls_using_go2rtc_tiktok large_monitor show_toolbar
stop_camera_00h05 stop_camera_00h30 stop_camera_01h00 stop_camera_03h00
stop_camera_03h30 stop_camera_22h30 stop_camera_23h00 stop_camera_23h30
ui_scoreboard
`)
var numberKeySet = keySet(`
api_count api_model_count camera_count db_check_range db_check_rmlm db_check_size_lm default_delay_camera
default_delay_go2rtc default_tiso_type fps_default livestream_default_bitrate
max_free_ram_force_reboot max_free_ram_force_restart_camera
max_free_ram_restart_camera max_shared_ram_camera toolbar_show_count
`)
var jsonKeySet = keySet(``)

func init() {
	numericRules["toolbar_show_count"] = numericRule{min: 0, max: 4096, integer: true}
	numericRules["api_count"] = numericRule{min: 0, max: 1000000, integer: true}
	numericRules["api_model_count"] = numericRule{min: 0, max: 1000000, integer: true}
}

type Catalog struct {
	keyDir    string
	mu        sync.RWMutex
	observed  map[string]KeyMeta
	live      map[string]struct{}
	empty     map[string]struct{}
	sourceErr string
}

func NewCatalog(keyDir string) *Catalog {
	return &Catalog{keyDir: keyDir, observed: map[string]KeyMeta{}, live: map[string]struct{}{}, empty: map[string]struct{}{}}
}

func (c *Catalog) List() []KeyMeta {
	keys := map[string]struct{}{}
	for _, key := range fallbackKeys {
		keys[key] = struct{}{}
	}
	entries, err := os.ReadDir(c.keyDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && validKeyName(entry.Name()) {
				keys[entry.Name()] = struct{}{}
			}
		}
	}

	c.mu.Lock()
	if err != nil {
		c.sourceErr = err.Error()
		c.live = map[string]struct{}{}
		c.empty = map[string]struct{}{}
	} else {
		c.sourceErr = ""
		c.live = make(map[string]struct{}, len(entries))
		c.empty = make(map[string]struct{})
		for _, entry := range entries {
			if !entry.IsDir() && validKeyName(entry.Name()) {
				c.live[entry.Name()] = struct{}{}
				if info, infoErr := entry.Info(); infoErr == nil && info.Size() == 0 {
					c.empty[entry.Name()] = struct{}{}
				}
			}
		}
		for key := range c.observed {
			if _, known := fallbackKeySet[key]; known {
				continue
			}
			if _, present := c.live[key]; !present {
				delete(c.observed, key)
			}
		}
	}
	for key := range c.observed {
		keys[key] = struct{}{}
	}
	observed := make(map[string]KeyMeta, len(c.observed))
	for key, meta := range c.observed {
		observed[key] = meta
	}
	c.mu.Unlock()

	out := make([]KeyMeta, 0, len(keys))
	for key := range keys {
		if meta, ok := observed[key]; ok {
			out = append(out, meta)
		} else {
			out = append(out, metaForKey(key, "", ""))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Group == out[j].Group {
			return out[i].Key < out[j].Key
		}
		return out[i].Group < out[j].Group
	})
	return out
}

func (c *Catalog) Meta(key string) (KeyMeta, bool) {
	key = strings.TrimSpace(key)
	if !validKeyName(key) {
		return KeyMeta{}, false
	}
	c.mu.RLock()
	meta, ok := c.observed[key]
	c.mu.RUnlock()
	if ok {
		return meta, true
	}
	for _, item := range c.List() {
		if item.Key == key {
			return item, true
		}
	}
	return KeyMeta{}, false
}

func (c *Catalog) Observe(key string, value any) {
	if !validKeyName(key) {
		return
	}
	meta := metaForKey(key, "", "")
	meta.ValueType = valueTypeForKey(key, value)
	c.mu.Lock()
	c.observed[key] = meta
	c.mu.Unlock()
}

func (c *Catalog) Status() (bool, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sourceErr == "", c.sourceErr
}

func (c *Catalog) Present(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.live[key]
	return ok
}

func (c *Catalog) Empty(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.empty[key]
	return ok
}

func validKeyName(key string) bool { return validKeyRe.MatchString(key) }

func metaForKey(key, label, description string) KeyMeta {
	if label == "" {
		label = strings.ReplaceAll(key, "_", " ")
	}
	secret := sensitiveKeyRe.MatchString(key)
	group := "Advanced / Unknown"
	switch {
	case secret:
		group = "Security / Credentials"
	case protectedKeyRe.MatchString(key):
		group = "Network / MQTT"
	case strings.HasPrefix(key, "logo_") || strings.HasPrefix(key, "disable_logo_") || key == "company_name" || key == "banner_top" || key == "custom_hashtags" || strings.HasPrefix(key, "app_"):
		group = "Branding / Logo"
	case key == "camera_count" || key == "toolbar_show_count" || key == "video_config" || key == "button_generate_go2rtc_stream" || strings.Contains(key, "livestream") || strings.HasPrefix(key, "hls_") || key == "place_livestream" || key == "fps_default" || strings.HasPrefix(key, "default_delay_") || key == "disable_cut_realtime":
		group = "Livestream"
	case strings.HasPrefix(key, "ui_") || key == "language" || key == "show_toolbar" || key == "large_monitor" || key == "help_link" || key == "url_live_help" || strings.HasPrefix(key, "default_tiso_") || key == "shop_id" || key == "realtime_shop_id":
		group = "UI / Display"
	case strings.HasPrefix(key, "stop_camera_") || strings.Contains(key, "reboot") || strings.Contains(key, "watch_uptime") || strings.HasPrefix(key, "db_check_") || strings.HasPrefix(key, "max_free_ram_") || strings.HasPrefix(key, "max_shared_ram_") || key == "button_restart_shinobi":
		group = "Schedule / Maintenance"
	}

	risk := RiskUnknown
	editable := false
	switch {
	case secret || protectedKeyRe.MatchString(key) || runtimeKeyRe.MatchString(key):
		risk = RiskProtected
	case confirmEditableKeySet[key]:
		risk = RiskConfirm
		editable = true
	case editableKeySet[key]:
		risk = RiskEditable
		editable = true
	}
	valueType := TypeString
	switch {
	case isLogoKey(key):
		valueType = TypeImage
	case booleanKeySet[key]:
		valueType = TypeBoolean
	case numberKeySet[key]:
		valueType = TypeNumber
	case jsonKeySet[key]:
		valueType = TypeJSON
	}
	return KeyMeta{Key: key, Label: label, Group: group, Description: description, Risk: risk, ValueType: valueType, Editable: editable, Secret: secret}
}

func valueTypeForKey(key string, value any) ValueType {
	meta := metaForKey(key, "", "")
	if meta.ValueType != TypeString {
		return meta.ValueType
	}
	return inferType(value, key)
}

func keySet(raw string) map[string]bool {
	out := make(map[string]bool)
	for _, key := range strings.Fields(raw) {
		out[key] = true
	}
	return out
}
