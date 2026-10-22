package bidahub

import "time"

// Club represents a Billiards or Pickleball venue grouping 1..N edge boxes.
type Club struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Branding         string    `json:"branding"` // "billiards" or "pickleball"
	Address          string    `json:"address,omitempty"`
	PortalToken      string    `json:"portal_token"`
	DLHMACKey        string    `json:"dl_hmac_key,omitempty"`
	StoreURL         string    `json:"store_url"`
	CStoreURL        string    `json:"cstore_url"`
	PortalURL        string    `json:"portal_url"`
	BoxIDs           []string  `json:"box_ids"`
	BoxesCount       int       `json:"boxes_count"`
	OnlineBoxesCount int       `json:"online_boxes_count"`
	CamerasCount     int       `json:"cameras_count"`
	CreatedAt        time.Time `json:"created_at"`
}

// Camera represents an IP camera or NVR channel managed on a specific Box.
// Primary key is compound: "<box_id>:<host>:<port>".
type Camera struct {
	ID              string    `json:"id"`
	BoxID           string    `json:"box_id"`
	BoxName         string    `json:"box_name"`
	ClubID          string    `json:"club_id"`
	ClubName        string    `json:"club_name"`
	TableNo         int       `json:"table_no"`
	Name            string    `json:"name"`
	TableName       string    `json:"table_name"`
	Vendor          string    `json:"vendor"`
	Host            string    `json:"host"`
	Port            int       `json:"port"`
	RTSPPort        int       `json:"rtsp_port"`
	NVRHost         string    `json:"nvr_host,omitempty"`
	NVRChannel      int       `json:"nvr_channel,omitempty"`
	Online          bool      `json:"online"`
	Recording       bool      `json:"recording"`
	Codec           string    `json:"codec"`
	SmartCodec      bool      `json:"smart_codec"`
	Resolution      string    `json:"resolution"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	FPS             int       `json:"fps"`
	BitrateKbps     int       `json:"bitrate_kbps"`
	RCMode          string    `json:"rc_mode"`
	GOP             int       `json:"gop"`
	AudioCodec      string    `json:"audio_codec"`
	DoubleStreaming bool      `json:"double_streaming"`
	StreamSource    string    `json:"stream_source"` // "nvr" or "direct"
	LastProbedAt    time.Time `json:"last_probed_at"`
}

// ShinobiMonitor represents a Shinobi NVR monitor record on a Box.
type ShinobiMonitor struct {
	MID                string `json:"mid"`
	BoxID              string `json:"box_id"`
	Name               string `json:"name"`
	Host               string `json:"host"`
	Mode               string `json:"mode"`
	Status             string `json:"status"`
	Codec              string `json:"codec"`
	ACodec             string `json:"acodec"`
	Width              int    `json:"width"`
	Height             int    `json:"height"`
	CustRecord         string `json:"cust_record"`
	InputMapChoicesOK  bool   `json:"input_map_choices_ok"`
	DoSnapshotDisabled bool   `json:"do_snapshot_disabled"`
	FragMP4OK          bool   `json:"frag_mp4_ok"`
}

// Box represents a single edge TV Box / mini-server at a Billiards or Pickleball venue.
type Box struct {
	ID                       string            `json:"id"`
	ClubID                   string            `json:"club_id"`
	ClubName                 string            `json:"club_name"`
	Name                     string            `json:"name"`
	VPNIP                    string            `json:"vpn_ip"`
	LANIP                    string            `json:"lan_ip"`
	VirtualIP                string            `json:"virtual_ip"`
	MACAddr                  string            `json:"mac_addr,omitempty"`
	SoCModel                 string            `json:"soc_model"`
	Arch                     string            `json:"arch"`
	CPUPct                   float64           `json:"cpu_pct"`
	LoadAvg1m                float64           `json:"load_avg_1m"`
	CPUCores                 int               `json:"cpu_cores"`
	TempC                    float64           `json:"temp_c"`
	RAMTotalMB               int               `json:"ram_total_mb"`
	RAMUsedMB                int               `json:"ram_used_mb"`
	RAMAvailableMB           int               `json:"ram_available_mb"`
	RAMUsedPct               int               `json:"ram_used_pct"`
	RAMPct                   int               `json:"ram_pct"`
	HeapAllocMB              float64           `json:"heap_alloc_mb"`
	HeapSysMB                float64           `json:"heap_sys_mb"`
	KspcamRSSMB              float64           `json:"kspcam_rss_mb"`
	CameraRSSMB              float64           `json:"camera_rss_mb"`
	Goroutines               int               `json:"goroutines"`
	SwapTotalMB              int               `json:"swap_total_mb"`
	SwapUsedMB               int               `json:"swap_used_mb"`
	SwapFreeMB               int               `json:"swap_free_mb"`
	ZRAMTotalMB              int               `json:"zram_total_mb"`
	ZRAMUsedMB               int               `json:"zram_used_mb"`
	ZRAMPct                  int               `json:"zram_pct"`
	ZRAMAlgo                 string            `json:"zram_algo"`
	ZRAMMemLimit             int64             `json:"zram_mem_limit"`
	Swappiness               int               `json:"swappiness"`
	SDTotalGB                int               `json:"sd_total_gb"`
	SDUsedGB                 int               `json:"sd_used_gb"`
	SDUsedPct                int               `json:"sd_used_pct"`
	SDFSType                 string            `json:"sd_fs_type"`
	SDMountOpts              string            `json:"sd_mount_opts"`
	SDDevice                 string            `json:"sd_device"`
	CleanFstab               bool              `json:"clean_fstab"`
	UdevAutomount            bool              `json:"udev_automount"`
	PM2CameraLimitMB         int               `json:"pm2_camera_limit_mb"`
	PM2InutLimitMB           int               `json:"pm2_inut_limit_mb"`
	PM2OtaLimitMB            int               `json:"pm2_ota_limit_mb"`
	PM2LogsDevNull           bool              `json:"pm2_logs_dev_null"`
	SysrqEnabled             int               `json:"sysrq_enabled"`
	SysrqHooksRemoved        bool              `json:"sysrq_hooks_removed"`
	CameraWatchdogActive     bool              `json:"camera_watchdog_active"`
	StorageGuardActive       bool              `json:"storage_guard_active"`
	ShinobiDoSnapshot        bool              `json:"shinobi_do_snapshot"`
	ShinobiWorkerFix         bool              `json:"shinobi_worker_fix"`
	ShinobiFragMP4           bool              `json:"shinobi_frag_mp4"`
	ShinobiGroupKey          string            `json:"shinobi_group_key"`
	ShinobiAPIKey            string            `json:"shinobi_api_key,omitempty"`
	ShinobiUserSizeMB        int               `json:"shinobi_user_size_mb"`
	ShinobiMonitorsTotal     int               `json:"shinobi_monitors_total"`
	ShinobiMonitorsRecording int               `json:"shinobi_monitors_recording"`
	CorruptedMP4Count        int               `json:"corrupted_mp4_count"`
	CamerasCount             int               `json:"cameras_count"`
	FFmpegCount              int               `json:"ffmpeg_count"`
	FFmpegExpected           int               `json:"ffmpeg_expected"`
	RXMbps                   float64           `json:"rx_mbps"`
	StoreDomain              string            `json:"store_domain"`
	CStoreDomain             string            `json:"cstore_domain"`
	StoreHTTPCode            int               `json:"store_http_code"`
	CStoreHTTPCode           int               `json:"cstore_http_code"`
	NVRHost                  string            `json:"nvr_host,omitempty"`
	Online                   bool              `json:"online"`
	TunnelActive             bool              `json:"tunnel_active"`
	TunnelAddr               string            `json:"tunnel_addr,omitempty"`
	Transport                string            `json:"transport"` // "tunnel" or "vpn"
	RTTMs                    int               `json:"rtt_ms"`
	RTTHistory               []int             `json:"rtt_history"`
	HardeningScore           int               `json:"hardening_score"`
	HardeningMax             int               `json:"hardening_max"`
	RedBidaScore             int               `json:"redbida_score"`
	RedBidaMax               int               `json:"redbida_max"`
	Branding                 string            `json:"branding"` // "billiards" or "pickleball"
	PortalToken              string            `json:"portal_token"`
	GGCode                   string            `json:"ggcode"`
	FRPCConfig               string            `json:"frpc_config"`
	RedBidaKeys              map[string]string `json:"redbida_keys,omitempty"`
	RedBidaTabs              map[string]string `json:"redbida_tabs,omitempty"`
	Cameras                  []*Camera         `json:"cameras,omitempty"`
	Monitors                 []*ShinobiMonitor `json:"monitors,omitempty"`
	LastSeen                 time.Time         `json:"last_seen"`
	UptimeSec                int64             `json:"uptime_sec"`
	UptimeHuman              string            `json:"uptime_human"`
	TelemetrySource          string            `json:"telemetry_source,omitempty"`
	Notes                    string            `json:"notes,omitempty"`
}

// HardeningPillar represents one of the 8 Golden Hardening SOP steps for a Box.
type HardeningPillar struct {
	ID             string `json:"id"`
	Step           int    `json:"step"`
	Name           string `json:"name"`
	Status         string `json:"status"` // "pass", "warn", "fail"
	Passed         bool   `json:"passed"`
	Detail         string `json:"detail"`
	CommandPreview string `json:"command_preview"`
}

// HardeningReport contains the 8-step Golden Hardening audit for one Box.
type HardeningReport struct {
	BoxID             string            `json:"box_id"`
	BoxName           string            `json:"box_name"`
	ClubID            string            `json:"club_id"`
	ClubName          string            `json:"club_name"`
	VPNIP             string            `json:"vpn_ip"`
	Score             int               `json:"score"`
	MaxScore          int               `json:"max_score"`
	CorruptedMP4Count int               `json:"corrupted_mp4_count"`
	Pillars           []HardeningPillar `json:"pillars"`
	AuditedAt         time.Time         `json:"audited_at"`
}

// RedBidaRule represents one of the 17 Golden Inspector rules for RedBida OTA-MQTT.
type RedBidaRule struct {
	ID       int    `json:"id"`
	Key      string `json:"key"`
	Title    string `json:"title"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Status   string `json:"status"` // "pass", "fail"
	Passed   bool   `json:"passed"`
}

// FRPCSubdomainEntry represents a store-* or c-store-* FRPC subdomain entry.
type FRPCSubdomainEntry struct {
	Subdomain string `json:"subdomain"`
	FullURL   string `json:"full_url"`
	Kind      string `json:"kind"` // "store" or "c-store"
	BoxID     string `json:"box_id"`
	BoxName   string `json:"box_name"`
	ClubName  string `json:"club_name"`
	HTTPCode  int    `json:"http_code"`
	Conflict  bool   `json:"conflict"`
}

// FRPCConflict represents a duplicate FRPC subdomain shared by multiple boxes.
type FRPCConflict struct {
	Subdomain string   `json:"subdomain"`
	BoxIDs    []string `json:"box_ids"`
	Message   string   `json:"message"`
}

// RedBidaReport contains the 17-rule inspection, editable keys, and FRPC audit for a Box.
type RedBidaReport struct {
	BoxID           string               `json:"box_id"`
	BoxName         string               `json:"box_name"`
	ClubID          string               `json:"club_id"`
	ClubName        string               `json:"club_name"`
	VPNIP           string               `json:"vpn_ip"`
	Branding        string               `json:"branding"`
	Score           int                  `json:"score"`
	MaxScore        int                  `json:"max_score"`
	StoreDomain     string               `json:"store_domain"`
	CStoreDomain    string               `json:"cstore_domain"`
	StoreHTTPCode   int                  `json:"store_http_code"`
	CStoreHTTPCode  int                  `json:"cstore_http_code"`
	DomainConflict  bool                 `json:"domain_conflict"`
	ConflictDetails []string             `json:"conflict_details,omitempty"`
	ShinobiGroupKey string               `json:"shinobi_group_key"`
	GGCode          string               `json:"ggcode"`
	Eth0VirtualIP   string               `json:"eth0_virtual_ip"`
	End0VirtualIP   string               `json:"end0_virtual_ip"`
	FRPCConfig      string               `json:"frpc_config"`
	Keys            map[string]string    `json:"keys"`
	Tabs            map[string]string    `json:"tabs"`
	Rules           []RedBidaRule        `json:"rules"`
	Subdomains      []FRPCSubdomainEntry `json:"subdomains,omitempty"`
}

// HandoverPack contains the 1-click customer handover kit for a Club/Box.
type HandoverPack struct {
	ID           string   `json:"id"`
	ClubID       string   `json:"club_id"`
	ClubName     string   `json:"club_name"`
	Branding     string   `json:"branding"`
	BoxIDs       []string `json:"box_ids"`
	BoxesCount   int      `json:"boxes_count"`
	CamerasCount int      `json:"cameras_count"`
	StoreURL     string   `json:"store_url"`
	CStoreURL    string   `json:"cstore_url"`
	PortalToken  string   `json:"portal_token"`
	PortalURL    string   `json:"portal_url"`
	ClipToken    string   `json:"clip_token"`
	ClipURL      string   `json:"clip_url"`
	ZaloTemplate string   `json:"zalo_template"`
	QRStoreSVG   string   `json:"qr_store_svg"`
	QRCStoreSVG  string   `json:"qr_cstore_svg"`
	QRPortalSVG  string   `json:"qr_portal_svg"`
	QRClipSVG    string   `json:"qr_clip_svg"`
}

// ClipClaims holds verified payload data inside a persistent HMAC-signed clip token.
type ClipClaims struct {
	ClubID    string `json:"club_id"`
	BoxID     string `json:"box_id"`
	CameraID  string `json:"camera_id"`
	ClipPath  string `json:"clip_path"`
	ExpiresAt int64  `json:"exp"`
}

// EnrollmentRequest tracks an unapproved edge box attempting to join the hub.
type EnrollmentRequest struct {
	ID          string    `json:"id"`
	BoxID       string    `json:"box_id"`
	Fingerprint string    `json:"fingerprint"`
	RemoteIP    string    `json:"remote_ip"`
	SoCModel    string    `json:"soc_model"`
	Status      string    `json:"status"` // "pending", "approved"
	FirstSeen   time.Time `json:"first_seen"`
}

// LockoutEntry tracks failed login attempts and brute-force lockouts per client IP.
type LockoutEntry struct {
	IP            string    `json:"ip"`
	FailedCount   int       `json:"failed_count"`
	FirstFailedAt time.Time `json:"first_failed_at"`
	LockedUntil   time.Time `json:"locked_until"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Session represents an authenticated Hub browser session.
type Session struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Role      string    `json:"role"` // "superadmin", "viewer"
	IP        string    `json:"ip"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// FleetKPI holds the top-level 6 SCADA KPI card metrics.
type FleetKPI struct {
	ClubsTotal        int     `json:"clubs_total"`
	MultiBoxClubs     int     `json:"multi_box_clubs"`
	BoxesTotal        int     `json:"boxes_total"`
	BoxesOnline       int     `json:"boxes_online"`
	BoxesOffline      int     `json:"boxes_offline"`
	CamerasTotal      int     `json:"cameras_total"`
	CamerasOnline     int     `json:"cameras_online"`
	FFmpegTotal       int     `json:"ffmpeg_total"`
	FFmpegExpected    int     `json:"ffmpeg_expected"`
	HardeningFull     int     `json:"hardening_full"`
	HardeningAvg      float64 `json:"hardening_avg"`
	DomainsOK         int     `json:"domains_ok"`
	DomainsConflict   int     `json:"domains_conflict"`
	DoubleStreamCount int     `json:"double_stream_count"`
	SDWarningCount    int     `json:"sd_warning_count"`
	AvgRTTMs          int     `json:"avg_rtt_ms"`
}

// UserAccount represents a Hub account stored with a one-way bcrypt password hash.
type UserAccount struct {
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"password_hash"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PasswordResetOTP stores a one-way SHA-256 hashed 6-digit OTP for password recovery.
type PasswordResetOTP struct {
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	OTPHash   string    `json:"otp_hash"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
}

// StoreData represents the atomically persisted JSON payload on disk.
type StoreData struct {
	Version     int                           `json:"version"`
	UpdatedAt   time.Time                     `json:"updated_at"`
	Clubs       []*Club                       `json:"clubs"`
	Boxes       []*Box                        `json:"boxes"`
	Users       map[string]*UserAccount       `json:"users"`
	ResetOTPs   map[string]*PasswordResetOTP  `json:"reset_otps,omitempty"`
	Lockouts    map[string]*LockoutEntry      `json:"lockouts"`
	Sessions    map[string]*Session           `json:"sessions"`
	Enrollments map[string]*EnrollmentRequest `json:"enrollments"`
}

// ServerConfig is a type alias for Config.
type ServerConfig = Config
