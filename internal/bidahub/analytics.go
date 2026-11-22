package bidahub

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// analyticsTZ is fixed GMT+7 (Asia/Ho_Chi_Minh) with zero external tzdata dependency.
var analyticsTZ = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)

const (
	analyticsMaxIngestBytes = 2 << 20 // 2 MiB
	analyticsHourlyKeepDays = 90
	analyticsDailyKeepDays  = 730 // 2 years
	analyticsBatchKeepDays  = 14
)

// AnalyticsBoxMetrics holds the 27 box-level usage counters shared by hourly/daily rollups and base_json.
type AnalyticsBoxMetrics struct {
	QRViews          int64 `json:"qr_views"`
	PageViews        int64 `json:"page_views"`
	UniqueVisitors   int64 `json:"unique_visitors"`
	LiveHLS          int64 `json:"live_hls"`
	VODPlays         int64 `json:"vod_plays"`
	ClipDownloads    int64 `json:"clip_downloads"`
	TimelineOpens    int64 `json:"timeline_opens"`
	CutClips         int64 `json:"cut_clips"`
	CutSeconds       int64 `json:"cut_seconds"`
	AIStudioOpens    int64 `json:"ai_studio_opens"`
	AIVideosLoads    int64 `json:"ai_videos_loads"`
	AIAnalyzes       int64 `json:"ai_analyzes"`
	AIShots          int64 `json:"ai_shots"`
	AIPots           int64 `json:"ai_pots"`
	AIPoolRuns       int64 `json:"ai_pool_runs"`
	AICaromRuns      int64 `json:"ai_carom_runs"`
	AIExportsIOS     int64 `json:"ai_exports_ios"`
	AIExportsDesktop int64 `json:"ai_exports_desktop"`
	AIExportRelay    int64 `json:"ai_export_relay"`
	AIErrors         int64 `json:"ai_errors"`
	HTTPErrors       int64 `json:"http_errors"`
	BytesOut         int64 `json:"bytes_out"`
	OSIOS            int64 `json:"os_ios"`
	OSAndroid        int64 `json:"os_android"`
	OSDesktop        int64 `json:"os_desktop"`
	OSApp            int64 `json:"os_app"`
	OSOther          int64 `json:"os_other"`
}

func (m *AnalyticsBoxMetrics) sanitize() {
	m.QRViews = clampNonNeg64(m.QRViews)
	m.PageViews = clampNonNeg64(m.PageViews)
	m.UniqueVisitors = clampNonNeg64(m.UniqueVisitors)
	m.LiveHLS = clampNonNeg64(m.LiveHLS)
	m.VODPlays = clampNonNeg64(m.VODPlays)
	m.ClipDownloads = clampNonNeg64(m.ClipDownloads)
	m.TimelineOpens = clampNonNeg64(m.TimelineOpens)
	m.CutClips = clampNonNeg64(m.CutClips)
	m.CutSeconds = clampNonNeg64(m.CutSeconds)
	m.AIStudioOpens = clampNonNeg64(m.AIStudioOpens)
	m.AIVideosLoads = clampNonNeg64(m.AIVideosLoads)
	m.AIAnalyzes = clampNonNeg64(m.AIAnalyzes)
	m.AIShots = clampNonNeg64(m.AIShots)
	m.AIPots = clampNonNeg64(m.AIPots)
	m.AIPoolRuns = clampNonNeg64(m.AIPoolRuns)
	m.AICaromRuns = clampNonNeg64(m.AICaromRuns)
	m.AIExportsIOS = clampNonNeg64(m.AIExportsIOS)
	m.AIExportsDesktop = clampNonNeg64(m.AIExportsDesktop)
	m.AIExportRelay = clampNonNeg64(m.AIExportRelay)
	m.AIErrors = clampNonNeg64(m.AIErrors)
	m.HTTPErrors = clampNonNeg64(m.HTTPErrors)
	m.BytesOut = clampNonNeg64(m.BytesOut)
	m.OSIOS = clampNonNeg64(m.OSIOS)
	m.OSAndroid = clampNonNeg64(m.OSAndroid)
	m.OSDesktop = clampNonNeg64(m.OSDesktop)
	m.OSApp = clampNonNeg64(m.OSApp)
	m.OSOther = clampNonNeg64(m.OSOther)
}

func (m AnalyticsBoxMetrics) add(o AnalyticsBoxMetrics) AnalyticsBoxMetrics {
	return AnalyticsBoxMetrics{
		QRViews:          m.QRViews + o.QRViews,
		PageViews:        m.PageViews + o.PageViews,
		UniqueVisitors:   m.UniqueVisitors + o.UniqueVisitors,
		LiveHLS:          m.LiveHLS + o.LiveHLS,
		VODPlays:         m.VODPlays + o.VODPlays,
		ClipDownloads:    m.ClipDownloads + o.ClipDownloads,
		TimelineOpens:    m.TimelineOpens + o.TimelineOpens,
		CutClips:         m.CutClips + o.CutClips,
		CutSeconds:       m.CutSeconds + o.CutSeconds,
		AIStudioOpens:    m.AIStudioOpens + o.AIStudioOpens,
		AIVideosLoads:    m.AIVideosLoads + o.AIVideosLoads,
		AIAnalyzes:       m.AIAnalyzes + o.AIAnalyzes,
		AIShots:          m.AIShots + o.AIShots,
		AIPots:           m.AIPots + o.AIPots,
		AIPoolRuns:       m.AIPoolRuns + o.AIPoolRuns,
		AICaromRuns:      m.AICaromRuns + o.AICaromRuns,
		AIExportsIOS:     m.AIExportsIOS + o.AIExportsIOS,
		AIExportsDesktop: m.AIExportsDesktop + o.AIExportsDesktop,
		AIExportRelay:    m.AIExportRelay + o.AIExportRelay,
		AIErrors:         m.AIErrors + o.AIErrors,
		HTTPErrors:       m.HTTPErrors + o.HTTPErrors,
		BytesOut:         m.BytesOut + o.BytesOut,
		OSIOS:            m.OSIOS + o.OSIOS,
		OSAndroid:        m.OSAndroid + o.OSAndroid,
		OSDesktop:        m.OSDesktop + o.OSDesktop,
		OSApp:            m.OSApp + o.OSApp,
		OSOther:          m.OSOther + o.OSOther,
	}
}

func (m AnalyticsBoxMetrics) maxWith(o AnalyticsBoxMetrics) AnalyticsBoxMetrics {
	return AnalyticsBoxMetrics{
		QRViews:          max64(m.QRViews, o.QRViews),
		PageViews:        max64(m.PageViews, o.PageViews),
		UniqueVisitors:   max64(m.UniqueVisitors, o.UniqueVisitors),
		LiveHLS:          max64(m.LiveHLS, o.LiveHLS),
		VODPlays:         max64(m.VODPlays, o.VODPlays),
		ClipDownloads:    max64(m.ClipDownloads, o.ClipDownloads),
		TimelineOpens:    max64(m.TimelineOpens, o.TimelineOpens),
		CutClips:         max64(m.CutClips, o.CutClips),
		CutSeconds:       max64(m.CutSeconds, o.CutSeconds),
		AIStudioOpens:    max64(m.AIStudioOpens, o.AIStudioOpens),
		AIVideosLoads:    max64(m.AIVideosLoads, o.AIVideosLoads),
		AIAnalyzes:       max64(m.AIAnalyzes, o.AIAnalyzes),
		AIShots:          max64(m.AIShots, o.AIShots),
		AIPots:           max64(m.AIPots, o.AIPots),
		AIPoolRuns:       max64(m.AIPoolRuns, o.AIPoolRuns),
		AICaromRuns:      max64(m.AICaromRuns, o.AICaromRuns),
		AIExportsIOS:     max64(m.AIExportsIOS, o.AIExportsIOS),
		AIExportsDesktop: max64(m.AIExportsDesktop, o.AIExportsDesktop),
		AIExportRelay:    max64(m.AIExportRelay, o.AIExportRelay),
		AIErrors:         max64(m.AIErrors, o.AIErrors),
		HTTPErrors:       max64(m.HTTPErrors, o.HTTPErrors),
		BytesOut:         max64(m.BytesOut, o.BytesOut),
		OSIOS:            max64(m.OSIOS, o.OSIOS),
		OSAndroid:        max64(m.OSAndroid, o.OSAndroid),
		OSDesktop:        max64(m.OSDesktop, o.OSDesktop),
		OSApp:            max64(m.OSApp, o.OSApp),
		OSOther:          max64(m.OSOther, o.OSOther),
	}
}

func (m AnalyticsBoxMetrics) aiExportsTotal() int64 {
	return max64(m.AIExportsIOS, m.AIExportRelay) + m.AIExportsDesktop
}

func (m AnalyticsBoxMetrics) hasAIActivity() bool {
	return m.AIStudioOpens > 0 || m.AIVideosLoads > 0 || m.AIAnalyzes > 0 || m.AIShots > 0 || m.aiExportsTotal() > 0
}

// AnalyticsCamMetrics holds the 15 per-camera daily usage counters.
type AnalyticsCamMetrics struct {
	QRViews        int64 `json:"qr_views"`
	UniqueVisitors int64 `json:"unique_visitors"`
	LiveHLS        int64 `json:"live_hls"`
	VODPlays       int64 `json:"vod_plays"`
	ClipDownloads  int64 `json:"clip_downloads"`
	TimelineOpens  int64 `json:"timeline_opens"`
	CutClips       int64 `json:"cut_clips"`
	CutSeconds     int64 `json:"cut_seconds"`
	AIStudioOpens  int64 `json:"ai_studio_opens"`
	AIAnalyzes     int64 `json:"ai_analyzes"`
	AIShots        int64 `json:"ai_shots"`
	AIPots         int64 `json:"ai_pots"`
	AIExports      int64 `json:"ai_exports"`
	AIErrors       int64 `json:"ai_errors"`
	BytesOut       int64 `json:"bytes_out"`
}

func (m *AnalyticsCamMetrics) sanitize() {
	m.QRViews = clampNonNeg64(m.QRViews)
	m.UniqueVisitors = clampNonNeg64(m.UniqueVisitors)
	m.LiveHLS = clampNonNeg64(m.LiveHLS)
	m.VODPlays = clampNonNeg64(m.VODPlays)
	m.ClipDownloads = clampNonNeg64(m.ClipDownloads)
	m.TimelineOpens = clampNonNeg64(m.TimelineOpens)
	m.CutClips = clampNonNeg64(m.CutClips)
	m.CutSeconds = clampNonNeg64(m.CutSeconds)
	m.AIStudioOpens = clampNonNeg64(m.AIStudioOpens)
	m.AIAnalyzes = clampNonNeg64(m.AIAnalyzes)
	m.AIShots = clampNonNeg64(m.AIShots)
	m.AIPots = clampNonNeg64(m.AIPots)
	m.AIExports = clampNonNeg64(m.AIExports)
	m.AIErrors = clampNonNeg64(m.AIErrors)
	m.BytesOut = clampNonNeg64(m.BytesOut)
}

func (m AnalyticsCamMetrics) add(o AnalyticsCamMetrics) AnalyticsCamMetrics {
	return AnalyticsCamMetrics{
		QRViews:        m.QRViews + o.QRViews,
		UniqueVisitors: m.UniqueVisitors + o.UniqueVisitors,
		LiveHLS:        m.LiveHLS + o.LiveHLS,
		VODPlays:       m.VODPlays + o.VODPlays,
		ClipDownloads:  m.ClipDownloads + o.ClipDownloads,
		TimelineOpens:  m.TimelineOpens + o.TimelineOpens,
		CutClips:       m.CutClips + o.CutClips,
		CutSeconds:     m.CutSeconds + o.CutSeconds,
		AIStudioOpens:  m.AIStudioOpens + o.AIStudioOpens,
		AIAnalyzes:     m.AIAnalyzes + o.AIAnalyzes,
		AIShots:        m.AIShots + o.AIShots,
		AIPots:         m.AIPots + o.AIPots,
		AIExports:      m.AIExports + o.AIExports,
		AIErrors:       m.AIErrors + o.AIErrors,
		BytesOut:       m.BytesOut + o.BytesOut,
	}
}

func (m AnalyticsCamMetrics) maxWith(o AnalyticsCamMetrics) AnalyticsCamMetrics {
	return AnalyticsCamMetrics{
		QRViews:        max64(m.QRViews, o.QRViews),
		UniqueVisitors: max64(m.UniqueVisitors, o.UniqueVisitors),
		LiveHLS:        max64(m.LiveHLS, o.LiveHLS),
		VODPlays:       max64(m.VODPlays, o.VODPlays),
		ClipDownloads:  max64(m.ClipDownloads, o.ClipDownloads),
		TimelineOpens:  max64(m.TimelineOpens, o.TimelineOpens),
		CutClips:       max64(m.CutClips, o.CutClips),
		CutSeconds:     max64(m.CutSeconds, o.CutSeconds),
		AIStudioOpens:  max64(m.AIStudioOpens, o.AIStudioOpens),
		AIAnalyzes:     max64(m.AIAnalyzes, o.AIAnalyzes),
		AIShots:        max64(m.AIShots, o.AIShots),
		AIPots:         max64(m.AIPots, o.AIPots),
		AIExports:      max64(m.AIExports, o.AIExports),
		AIErrors:       max64(m.AIErrors, o.AIErrors),
		BytesOut:       max64(m.BytesOut, o.BytesOut),
	}
}

// AnalyticsBoxHourlyIn represents one hourly bucket in an ingest payload.
type AnalyticsBoxHourlyIn struct {
	BucketHour string `json:"bucket_hour"` // "YYYY-MM-DD HH:00" GMT+7
	AnalyticsBoxMetrics
}

// AnalyticsBoxDailyIn carries exact box-level daily unique visitor counts.
type AnalyticsBoxDailyIn struct {
	BucketDate     string `json:"bucket_date"` // "YYYY-MM-DD" GMT+7
	UniqueVisitors int64  `json:"unique_visitors"`
}

// AnalyticsCamDailyIn represents one per-camera daily bucket in an ingest payload.
type AnalyticsCamDailyIn struct {
	BucketDate string `json:"bucket_date"` // "YYYY-MM-DD" GMT+7
	CameraID   string `json:"camera_id"`
	TableName  string `json:"table_name,omitempty"`
	AnalyticsCamMetrics
}

// AnalyticsIngestPayload is accepted by POST /fleet/api/analytics/ingest from both
// redbida-go (cumulative snapshot with source_epoch) and legacy Node-RED (delta with batch_id).
type AnalyticsIngestPayload struct {
	BoxID       string                 `json:"box_id"`
	Secret      string                 `json:"secret,omitempty"`
	Source      string                 `json:"source"`                 // "redbida-go" | "nodered"
	SourceEpoch string                 `json:"source_epoch,omitempty"` // persistent epoch ID for Mode A
	BatchID     string                 `json:"batch_id,omitempty"`     // idempotency key for Mode B
	GeneratedAt string                 `json:"generated_at,omitempty"`
	AIEnabled   bool                   `json:"ai_enabled"`
	BoxHourly   []AnalyticsBoxHourlyIn `json:"box_hourly"`
	BoxDaily    []AnalyticsBoxDailyIn  `json:"box_daily,omitempty"`
	CameraDaily []AnalyticsCamDailyIn  `json:"camera_daily,omitempty"`
}

// AnalyticsIngestResult summarizes a completed analytics ingest transaction.
type AnalyticsIngestResult struct {
	OK             bool `json:"ok"`
	Deduped        bool `json:"deduped,omitempty"`
	HourlyUpserted int  `json:"hourly_upserted,omitempty"`
	DailyUpdated   int  `json:"daily_updated,omitempty"`
	CamerasUpdated int  `json:"cameras_updated,omitempty"`
}

// AnalyticsKPI holds the fleet/club overview KPI totals.
type AnalyticsKPI struct {
	QRViews          int64 `json:"qr_views"`
	PageViews        int64 `json:"page_views"`
	UniqueVisitors   int64 `json:"unique_visitors"`
	LiveHLS          int64 `json:"live_hls"`
	VODPlays         int64 `json:"vod_plays"`
	ClipDownloads    int64 `json:"clip_downloads"`
	TimelineOpens    int64 `json:"timeline_opens"`
	CutClips         int64 `json:"cut_clips"`
	CutSeconds       int64 `json:"cut_seconds"`
	AIStudioOpens    int64 `json:"ai_studio_opens"`
	AIVideosLoads    int64 `json:"ai_videos_loads"`
	AIAnalyzes       int64 `json:"ai_analyzes"`
	AIShots          int64 `json:"ai_shots"`
	AIPots           int64 `json:"ai_pots"`
	AIPoolRuns       int64 `json:"ai_pool_runs"`
	AICaromRuns      int64 `json:"ai_carom_runs"`
	AIExportsIOS     int64 `json:"ai_exports_ios"`
	AIExportsDesktop int64 `json:"ai_exports_desktop"`
	AIExportRelay    int64 `json:"ai_export_relay"`
	AIErrors         int64 `json:"ai_errors"`
	HTTPErrors       int64 `json:"http_errors"`
	BytesOut         int64 `json:"bytes_out"`
	ActiveBoxes      int   `json:"active_boxes"`
	ActiveClubs      int   `json:"active_clubs"`
	AIActiveBoxes    int   `json:"ai_active_boxes"`
}

// AnalyticsOSBreakdown holds aggregated OS/device counts.
type AnalyticsOSBreakdown struct {
	IOS     int64 `json:"ios"`
	Android int64 `json:"android"`
	Desktop int64 `json:"desktop"`
	App     int64 `json:"app"`
	Other   int64 `json:"other"`
}

// AnalyticsPeakHour holds one GMT+7 hour slot (0..23) for peak-hour charts.
type AnalyticsPeakHour struct {
	Hour           int   `json:"hour"`
	QRViews        int64 `json:"qr_views"`
	DownloadsCuts  int64 `json:"downloads_cuts"`
	AIRuns         int64 `json:"ai_runs"`
	UniqueVisitors int64 `json:"unique_visitors"`
	BytesOut       int64 `json:"bytes_out"`
}

// AnalyticsDailyPoint holds one GMT+7 date slot in daily_series.
type AnalyticsDailyPoint struct {
	Date           string `json:"date"`
	QRViews        int64  `json:"qr_views"`
	UniqueVisitors int64  `json:"unique_visitors"`
	DownloadsCuts  int64  `json:"downloads_cuts"`
	AIAnalyzes     int64  `json:"ai_analyzes"`
	AIExports      int64  `json:"ai_exports"`
	BytesOut       int64  `json:"bytes_out"`
}

// AnalyticsTopClub holds aggregated analytics for one club.
type AnalyticsTopClub struct {
	ClubID         string   `json:"club_id"`
	ClubName       string   `json:"club_name"`
	BoxesCount     int      `json:"boxes_count"`
	Sources        []string `json:"sources"`
	AIEnabled      bool     `json:"ai_enabled"`
	QRViews        int64    `json:"qr_views"`
	UniqueVisitors int64    `json:"unique_visitors"`
	LiveHLS        int64    `json:"live_hls"`
	VODPlays       int64    `json:"vod_plays"`
	ClipDownloads  int64    `json:"clip_downloads"`
	CutClips       int64    `json:"cut_clips"`
	AIStudioOpens  int64    `json:"ai_studio_opens"`
	AIAnalyzes     int64    `json:"ai_analyzes"`
	AIShots        int64    `json:"ai_shots"`
	AIExports      int64    `json:"ai_exports"`
	AIErrors       int64    `json:"ai_errors"`
	HTTPErrors     int64    `json:"http_errors"`
	BytesOut       int64    `json:"bytes_out"`
	LastIngestAt   string   `json:"last_ingest_at"`
}

// AnalyticsTopBox holds aggregated analytics for one box.
type AnalyticsTopBox struct {
	BoxID          string `json:"box_id"`
	BoxName        string `json:"box_name"`
	ClubID         string `json:"club_id"`
	ClubName       string `json:"club_name"`
	Source         string `json:"source"`
	AIEnabled      bool   `json:"ai_enabled"`
	QRViews        int64  `json:"qr_views"`
	UniqueVisitors int64  `json:"unique_visitors"`
	LiveHLS        int64  `json:"live_hls"`
	VODPlays       int64  `json:"vod_plays"`
	ClipDownloads  int64  `json:"clip_downloads"`
	CutClips       int64  `json:"cut_clips"`
	AIStudioOpens  int64  `json:"ai_studio_opens"`
	AIAnalyzes     int64  `json:"ai_analyzes"`
	AIShots        int64  `json:"ai_shots"`
	AIExports      int64  `json:"ai_exports"`
	AIErrors       int64  `json:"ai_errors"`
	HTTPErrors     int64  `json:"http_errors"`
	BytesOut       int64  `json:"bytes_out"`
	LastIngestAt   string `json:"last_ingest_at"`
}

// AnalyticsOverviewResponse is the JSON contract for GET /fleet/api/analytics/overview.
type AnalyticsOverviewResponse struct {
	OK          bool                  `json:"ok"`
	Range       string                `json:"range"`
	FromDate    string                `json:"from_date"`
	ToDate      string                `json:"to_date"`
	KPI         AnalyticsKPI          `json:"kpi"`
	OSBreakdown AnalyticsOSBreakdown  `json:"os_breakdown"`
	PeakHours   []AnalyticsPeakHour   `json:"peak_hours"`
	DailySeries []AnalyticsDailyPoint `json:"daily_series"`
	TopClubs    []AnalyticsTopClub    `json:"top_clubs"`
	TopBoxes    []AnalyticsTopBox     `json:"top_boxes"`
}

// AnalyticsCameraRow is one per-camera/table usage row.
type AnalyticsCameraRow struct {
	BoxID          string `json:"box_id"`
	BoxName        string `json:"box_name"`
	ClubID         string `json:"club_id"`
	ClubName       string `json:"club_name"`
	CameraID       string `json:"camera_id"`
	TableName      string `json:"table_name"`
	QRViews        int64  `json:"qr_views"`
	UniqueVisitors int64  `json:"unique_visitors"`
	LiveHLS        int64  `json:"live_hls"`
	VODPlays       int64  `json:"vod_plays"`
	ClipDownloads  int64  `json:"clip_downloads"`
	TimelineOpens  int64  `json:"timeline_opens"`
	CutClips       int64  `json:"cut_clips"`
	CutSeconds     int64  `json:"cut_seconds"`
	AIStudioOpens  int64  `json:"ai_studio_opens"`
	AIAnalyzes     int64  `json:"ai_analyzes"`
	AIShots        int64  `json:"ai_shots"`
	AIPots         int64  `json:"ai_pots"`
	AIExports      int64  `json:"ai_exports"`
	AIErrors       int64  `json:"ai_errors"`
	BytesOut       int64  `json:"bytes_out"`
}

// AnalyticsCamerasResponse is the JSON contract for GET /fleet/api/analytics/cameras.
type AnalyticsCamerasResponse struct {
	OK      bool                 `json:"ok"`
	Range   string               `json:"range"`
	Cameras []AnalyticsCameraRow `json:"cameras"`
}

// AnalyticsPruneStats reports rows deleted by a retention cleanup run.
type AnalyticsPruneStats struct {
	HourlyDeleted  int64
	DailyDeleted   int64
	CameraDeleted  int64
	BatchesDeleted int64
}

type analyticsQueryParams struct {
	Range  string
	ClubID string
	BoxID  string
	AIOnly bool
}

type analyticsCameraQueryParams struct {
	Range  string
	ClubID string
	BoxID  string
	Limit  int
}

func clampNonNeg64(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

func max64(a, b int64) int64 {
	if a >= b {
		return a
	}
	return b
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func normalizeAnalyticsRange(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "24h":
		return "24h"
	case "7d":
		return "7d"
	case "30d":
		return "30d"
	case "90d":
		return "90d"
	default:
		return "7d"
	}
}

func normalizePortalAnalyticsRange(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "24h":
		return "24h"
	case "7d":
		return "7d"
	case "30d":
		return "30d"
	default:
		return "7d"
	}
}

func parseBucketHour(s string) (string, string, int, bool) {
	s = strings.TrimSpace(s)
	t, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		return "", "", 0, false
	}
	// Normalize to exact top-of-hour string "YYYY-MM-DD HH:00"
	norm := t.Format("2006-01-02 15:00")
	return norm, norm[:10], t.Hour(), true
}

func parseBucketDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return "", false
	}
	return t.Format("2006-01-02"), true
}

func extractIngestSecret(r *http.Request, payloadSecret string) string {
	if s := strings.TrimSpace(payloadSecret); s != "" {
		return s
	}
	if s := strings.TrimSpace(r.Header.Get("X-Box-Secret")); s != "" {
		return s
	}
	if s := strings.TrimSpace(r.Header.Get("X-Hub-Key")); s != "" {
		return s
	}
	if authz := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return ""
}

// handleAnalyticsIngest handles POST /fleet/api/analytics/ingest and POST /api/analytics/ingest.
func (s *Server) handleAnalyticsIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if r.Body == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, analyticsMaxIngestBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "read_body_failed"})
		return
	}
	if len(raw) > analyticsMaxIngestBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "payload_too_large"})
		return
	}

	var payload AnalyticsIngestPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}

	boxID := strings.TrimSpace(payload.BoxID)
	if boxID == "" {
		boxID = strings.TrimSpace(r.Header.Get("X-Box-ID"))
	}
	if boxID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_box_id"})
		return
	}
	payload.BoxID = boxID

	// Validate box/hub credentials before touching SQLite.
	secret := extractIngestSecret(r, payload.Secret)
	authorized := false
	if s.validBoxSecret(secret) {
		authorized = true
	} else if p, ok := s.authenticateRequest(r); ok {
		if p.Role == "viewer" {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":   "forbidden",
				"message": "Tài khoản viewer chỉ có quyền xem (Read-Only).",
			})
			return
		}
		authorized = true
	} else if s.cfg.APIKey == "" && s.store.HasBox(boxID) {
		authorized = true
	}
	if !authorized {
		if secret != "" {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":   "invalid_secret",
				"message": "X-Box-Secret / X-Hub-Key không hợp lệ.",
			})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error":   "unauthorized",
			"message": "Yêu cầu X-Box-Secret hoặc X-Hub-Key hợp lệ.",
		})
		return
	}

	// Normalize source
	src := strings.ToLower(strings.TrimSpace(payload.Source))
	if src == "" {
		if strings.TrimSpace(payload.BatchID) != "" && strings.TrimSpace(payload.SourceEpoch) == "" {
			src = "nodered"
		} else {
			src = "redbida-go"
		}
	}
	if src != "redbida-go" && src != "nodered" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported_source"})
		return
	}
	payload.Source = src
	if src == "nodered" && strings.TrimSpace(payload.BatchID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_batch_id"})
		return
	}

	res, err := s.IngestAnalytics(payload, time.Now())
	if err != nil {
		if errors.Is(err, errNoSQLiteDB) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "sqlite_unavailable"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "ingest_failed", "detail": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// IngestAnalytics persists an analytics payload (Mode A redbida-go snapshot or Mode B nodered delta)
// atomically inside a single SQLite transaction with strict idempotency and zero double-counting.
func (s *Server) IngestAnalytics(payload AnalyticsIngestPayload, now time.Time) (AnalyticsIngestResult, error) {
	boxID := strings.TrimSpace(payload.BoxID)
	if boxID == "" {
		return AnalyticsIngestResult{}, fmt.Errorf("missing box_id")
	}

	// Snapshot box metadata BEFORE acquiring SQLite lock (enforces s.mu -> s.dbMu ordering).
	var clubID string
	camTableNames := make(map[string]string)
	if box, ok := s.store.GetBox(boxID); ok && box != nil {
		clubID = box.ClubID
		for _, cam := range box.Cameras {
			if cam != nil && cam.Name != "" && cam.TableName != "" {
				camTableNames[strings.ToLower(cam.Name)] = cam.TableName
			}
		}
		for k, v := range box.RedBidaTabs {
			if strings.TrimSpace(v) != "" {
				lk := strings.ToLower(strings.TrimSpace(k))
				if _, exists := camTableNames[lk]; !exists {
					camTableNames[lk] = strings.TrimSpace(v)
				}
			}
		}
	}

	nowStr := now.UTC().Format(time.RFC3339)
	var result AnalyticsIngestResult
	result.OK = true

	err := s.store.WithSQLiteTx(func(tx *sql.Tx) error {
		if payload.Source == "nodered" {
			return ingestNodeRedDeltaTx(tx, boxID, clubID, payload, camTableNames, nowStr, &result)
		}
		return ingestRedBidaSnapshotTx(tx, boxID, clubID, payload, camTableNames, nowStr, &result)
	})
	if err != nil {
		return AnalyticsIngestResult{}, err
	}
	return result, nil
}

func ingestRedBidaSnapshotTx(
	tx *sql.Tx,
	boxID, clubID string,
	payload AnalyticsIngestPayload,
	camTableNames map[string]string,
	nowStr string,
	result *AnalyticsIngestResult,
) error {
	epoch := strings.TrimSpace(payload.SourceEpoch)
	if epoch == "" {
		epoch = "default"
	}

	affectedDates := make(map[string]struct{})
	exactDailyUniques := make(map[string]int64)
	for _, bd := range payload.BoxDaily {
		dateStr, ok := parseBucketDate(bd.BucketDate)
		if !ok {
			continue
		}
		u := clampNonNeg64(bd.UniqueVisitors)
		if cur, exists := exactDailyUniques[dateStr]; !exists || u > cur {
			exactDailyUniques[dateStr] = u
		}
		affectedDates[dateStr] = struct{}{}
	}

	// 1. Upsert usage_box_hourly with epoch-safe base_json freezing.
	for _, item := range payload.BoxHourly {
		hourStr, dateStr, _, ok := parseBucketHour(item.BucketHour)
		if !ok {
			continue
		}
		inc := item.AnalyticsBoxMetrics
		inc.sanitize()
		aiEnabledInt := boolToInt(payload.AIEnabled || inc.hasAIActivity())

		var existingClub, existingEpoch, existingBaseJSON string
		var existingAI int
		var existing AnalyticsBoxMetrics

		err := tx.QueryRow(`
			SELECT club_id, ai_enabled, source_epoch,
				qr_views, page_views, unique_visitors, live_hls, vod_plays, clip_downloads,
				timeline_opens, cut_clips, cut_seconds,
				ai_studio_opens, ai_videos_loads, ai_analyzes, ai_shots, ai_pots,
				ai_pool_runs, ai_carom_runs, ai_exports_ios, ai_exports_desktop, ai_export_relay,
				ai_errors, http_errors, bytes_out,
				os_ios, os_android, os_desktop, os_app, os_other,
				base_json
			FROM usage_box_hourly
			WHERE box_id = ? AND bucket_hour = ?
		`, boxID, hourStr).Scan(
			&existingClub, &existingAI, &existingEpoch,
			&existing.QRViews, &existing.PageViews, &existing.UniqueVisitors, &existing.LiveHLS, &existing.VODPlays, &existing.ClipDownloads,
			&existing.TimelineOpens, &existing.CutClips, &existing.CutSeconds,
			&existing.AIStudioOpens, &existing.AIVideosLoads, &existing.AIAnalyzes, &existing.AIShots, &existing.AIPots,
			&existing.AIPoolRuns, &existing.AICaromRuns, &existing.AIExportsIOS, &existing.AIExportsDesktop, &existing.AIExportRelay,
			&existing.AIErrors, &existing.HTTPErrors, &existing.BytesOut,
			&existing.OSIOS, &existing.OSAndroid, &existing.OSDesktop, &existing.OSApp, &existing.OSOther,
			&existingBaseJSON,
		)

		effClubID := clubID
		if effClubID == "" {
			effClubID = existingClub
		}

		if errors.Is(err, sql.ErrNoRows) {
			if err := upsertBoxHourlyRowTx(tx, boxID, effClubID, "redbida-go", aiEnabledInt, epoch, hourStr, inc, "{}", nowStr); err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("select usage_box_hourly: %w", err)
		} else {
			if existingAI != 0 {
				aiEnabledInt = 1
			}
			var base AnalyticsBoxMetrics
			_ = json.Unmarshal([]byte(existingBaseJSON), &base)

			var merged AnalyticsBoxMetrics
			newBaseJSON := existingBaseJSON
			if newBaseJSON == "" {
				newBaseJSON = "{}"
			}
			if existingEpoch == epoch {
				merged = existing.maxWith(base.add(inc))
			} else {
				// Epoch changed (e.g. checkpoint wiped & restarted, or upgraded from nodered):
				// freeze previous totals into base_json so zero historical counts are ever lost.
				base = existing
				if bRaw, mErr := json.Marshal(base); mErr == nil {
					newBaseJSON = string(bRaw)
				}
				merged = base.add(inc)
			}
			if err := upsertBoxHourlyRowTx(tx, boxID, effClubID, "redbida-go", aiEnabledInt, epoch, hourStr, merged, newBaseJSON, nowStr); err != nil {
				return err
			}
		}

		affectedDates[dateStr] = struct{}{}
		result.HourlyUpserted++
	}

	// 2. Recompute/upsert affected usage_box_daily rows from usage_box_hourly + exact box_daily uniques.
	dates := sortedKeys(affectedDates)
	for _, dateStr := range dates {
		exactUnique, hasExact := exactDailyUniques[dateStr]
		updated, err := rollupBoxDailyTx(tx, boxID, clubID, "redbida-go", boolToInt(payload.AIEnabled), epoch, dateStr, hasExact, exactUnique, nowStr)
		if err != nil {
			return err
		}
		if updated {
			result.DailyUpdated++
		}
	}

	// 3. Epoch-safe upsert for usage_camera_daily.
	for _, cam := range payload.CameraDaily {
		dateStr, ok := parseBucketDate(cam.BucketDate)
		if !ok {
			continue
		}
		camID := strings.TrimSpace(cam.CameraID)
		if camID == "" {
			continue
		}
		inc := cam.AnalyticsCamMetrics
		inc.sanitize()

		tableName := strings.TrimSpace(cam.TableName)

		var existingClub, existingTable, existingEpoch, existingBaseJSON string
		var existing AnalyticsCamMetrics
		err := tx.QueryRow(`
			SELECT club_id, table_name, source_epoch,
				qr_views, unique_visitors, live_hls, vod_plays, clip_downloads,
				timeline_opens, cut_clips, cut_seconds,
				ai_studio_opens, ai_analyzes, ai_shots, ai_pots, ai_exports, ai_errors, bytes_out,
				base_json
			FROM usage_camera_daily
			WHERE box_id = ? AND bucket_date = ? AND camera_id = ?
		`, boxID, dateStr, camID).Scan(
			&existingClub, &existingTable, &existingEpoch,
			&existing.QRViews, &existing.UniqueVisitors, &existing.LiveHLS, &existing.VODPlays, &existing.ClipDownloads,
			&existing.TimelineOpens, &existing.CutClips, &existing.CutSeconds,
			&existing.AIStudioOpens, &existing.AIAnalyzes, &existing.AIShots, &existing.AIPots, &existing.AIExports, &existing.AIErrors, &existing.BytesOut,
			&existingBaseJSON,
		)

		effClubID := clubID
		if effClubID == "" {
			effClubID = existingClub
		}
		if tableName == "" {
			tableName = existingTable
		}
		if tableName == "" {
			tableName = camTableNames[strings.ToLower(camID)]
		}

		if errors.Is(err, sql.ErrNoRows) {
			if err := upsertCameraDailyRowTx(tx, boxID, effClubID, camID, tableName, epoch, dateStr, inc, "{}", nowStr); err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("select usage_camera_daily: %w", err)
		} else {
			var base AnalyticsCamMetrics
			_ = json.Unmarshal([]byte(existingBaseJSON), &base)

			var merged AnalyticsCamMetrics
			newBaseJSON := existingBaseJSON
			if newBaseJSON == "" {
				newBaseJSON = "{}"
			}
			if existingEpoch == epoch {
				merged = existing.maxWith(base.add(inc))
			} else {
				base = existing
				if bRaw, mErr := json.Marshal(base); mErr == nil {
					newBaseJSON = string(bRaw)
				}
				merged = base.add(inc)
			}
			if err := upsertCameraDailyRowTx(tx, boxID, effClubID, camID, tableName, epoch, dateStr, merged, newBaseJSON, nowStr); err != nil {
				return err
			}
		}
		result.CamerasUpdated++
	}

	return nil
}

func ingestNodeRedDeltaTx(
	tx *sql.Tx,
	boxID, clubID string,
	payload AnalyticsIngestPayload,
	camTableNames map[string]string,
	nowStr string,
	result *AnalyticsIngestResult,
) error {
	batchID := strings.TrimSpace(payload.BatchID)
	res, err := tx.Exec(`
		INSERT OR IGNORE INTO analytics_ingest_batches (box_id, batch_id, received_at)
		VALUES (?, ?, ?)
	`, boxID, batchID, nowStr)
	if err != nil {
		return fmt.Errorf("insert analytics_ingest_batches: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		result.Deduped = true
		return nil
	}

	affectedDates := make(map[string]struct{})

	for _, item := range payload.BoxHourly {
		hourStr, dateStr, _, ok := parseBucketHour(item.BucketHour)
		if !ok {
			continue
		}
		delta := item.AnalyticsBoxMetrics
		delta.sanitize()

		var existingClub, existingSource, existingEpoch, existingBaseJSON string
		var existingAI int
		var existing AnalyticsBoxMetrics

		err := tx.QueryRow(`
			SELECT club_id, source, ai_enabled, source_epoch,
				qr_views, page_views, unique_visitors, live_hls, vod_plays, clip_downloads,
				timeline_opens, cut_clips, cut_seconds,
				ai_studio_opens, ai_videos_loads, ai_analyzes, ai_shots, ai_pots,
				ai_pool_runs, ai_carom_runs, ai_exports_ios, ai_exports_desktop, ai_export_relay,
				ai_errors, http_errors, bytes_out,
				os_ios, os_android, os_desktop, os_app, os_other,
				base_json
			FROM usage_box_hourly
			WHERE box_id = ? AND bucket_hour = ?
		`, boxID, hourStr).Scan(
			&existingClub, &existingSource, &existingAI, &existingEpoch,
			&existing.QRViews, &existing.PageViews, &existing.UniqueVisitors, &existing.LiveHLS, &existing.VODPlays, &existing.ClipDownloads,
			&existing.TimelineOpens, &existing.CutClips, &existing.CutSeconds,
			&existing.AIStudioOpens, &existing.AIVideosLoads, &existing.AIAnalyzes, &existing.AIShots, &existing.AIPots,
			&existing.AIPoolRuns, &existing.AICaromRuns, &existing.AIExportsIOS, &existing.AIExportsDesktop, &existing.AIExportRelay,
			&existing.AIErrors, &existing.HTTPErrors, &existing.BytesOut,
			&existing.OSIOS, &existing.OSAndroid, &existing.OSDesktop, &existing.OSApp, &existing.OSOther,
			&existingBaseJSON,
		)

		effClubID := clubID
		if effClubID == "" {
			effClubID = existingClub
		}

		if errors.Is(err, sql.ErrNoRows) {
			// Store delta in base_json too so a future mid-day upgrade to redbida-go preserves Node-RED counts.
			baseRaw, _ := json.Marshal(delta)
			if err := upsertBoxHourlyRowTx(tx, boxID, effClubID, "nodered", 0, "", hourStr, delta, string(baseRaw), nowStr); err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("select usage_box_hourly (nodered): %w", err)
		} else {
			var base AnalyticsBoxMetrics
			_ = json.Unmarshal([]byte(existingBaseJSON), &base)
			newBase := base.add(delta)
			newBaseRaw, _ := json.Marshal(newBase)
			merged := existing.add(delta)
			src := existingSource
			if src == "" {
				src = "nodered"
			}
			if err := upsertBoxHourlyRowTx(tx, boxID, effClubID, src, existingAI, existingEpoch, hourStr, merged, string(newBaseRaw), nowStr); err != nil {
				return err
			}
		}

		affectedDates[dateStr] = struct{}{}
		result.HourlyUpserted++
	}

	// Optional camera_daily deltas if ever sent by Node-RED.
	for _, cam := range payload.CameraDaily {
		dateStr, ok := parseBucketDate(cam.BucketDate)
		if !ok {
			continue
		}
		camID := strings.TrimSpace(cam.CameraID)
		if camID == "" {
			continue
		}
		delta := cam.AnalyticsCamMetrics
		delta.sanitize()
		tableName := strings.TrimSpace(cam.TableName)

		var existingClub, existingTable, existingEpoch, existingBaseJSON string
		var existing AnalyticsCamMetrics
		err := tx.QueryRow(`
			SELECT club_id, table_name, source_epoch,
				qr_views, unique_visitors, live_hls, vod_plays, clip_downloads,
				timeline_opens, cut_clips, cut_seconds,
				ai_studio_opens, ai_analyzes, ai_shots, ai_pots, ai_exports, ai_errors, bytes_out,
				base_json
			FROM usage_camera_daily
			WHERE box_id = ? AND bucket_date = ? AND camera_id = ?
		`, boxID, dateStr, camID).Scan(
			&existingClub, &existingTable, &existingEpoch,
			&existing.QRViews, &existing.UniqueVisitors, &existing.LiveHLS, &existing.VODPlays, &existing.ClipDownloads,
			&existing.TimelineOpens, &existing.CutClips, &existing.CutSeconds,
			&existing.AIStudioOpens, &existing.AIAnalyzes, &existing.AIShots, &existing.AIPots, &existing.AIExports, &existing.AIErrors, &existing.BytesOut,
			&existingBaseJSON,
		)
		effClubID := clubID
		if effClubID == "" {
			effClubID = existingClub
		}
		if tableName == "" {
			tableName = existingTable
		}
		if tableName == "" {
			tableName = camTableNames[strings.ToLower(camID)]
		}
		if errors.Is(err, sql.ErrNoRows) {
			baseRaw, _ := json.Marshal(delta)
			if err := upsertCameraDailyRowTx(tx, boxID, effClubID, camID, tableName, "", dateStr, delta, string(baseRaw), nowStr); err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("select usage_camera_daily (nodered): %w", err)
		} else {
			var base AnalyticsCamMetrics
			_ = json.Unmarshal([]byte(existingBaseJSON), &base)
			newBaseRaw, _ := json.Marshal(base.add(delta))
			if err := upsertCameraDailyRowTx(tx, boxID, effClubID, camID, tableName, existingEpoch, dateStr, existing.add(delta), string(newBaseRaw), nowStr); err != nil {
				return err
			}
		}
		result.CamerasUpdated++
	}

	dates := sortedKeys(affectedDates)
	for _, dateStr := range dates {
		updated, err := rollupBoxDailyTx(tx, boxID, clubID, "nodered", 0, "", dateStr, false, 0, nowStr)
		if err != nil {
			return err
		}
		if updated {
			result.DailyUpdated++
		}
	}
	return nil
}

func rollupBoxDailyTx(
	tx *sql.Tx,
	boxID, clubID, incomingSource string,
	incomingAI int,
	incomingEpoch string,
	dateStr string,
	hasExactUnique bool,
	exactUnique int64,
	nowStr string,
) (bool, error) {
	// Sum all hourly buckets for (boxID, dateStr).
	var hourlyCount int
	var hourlyMaxAI int
	var hourlySum AnalyticsBoxMetrics
	err := tx.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(MAX(ai_enabled), 0),
			COALESCE(SUM(qr_views), 0),
			COALESCE(SUM(page_views), 0),
			COALESCE(SUM(unique_visitors), 0),
			COALESCE(SUM(live_hls), 0),
			COALESCE(SUM(vod_plays), 0),
			COALESCE(SUM(clip_downloads), 0),
			COALESCE(SUM(timeline_opens), 0),
			COALESCE(SUM(cut_clips), 0),
			COALESCE(SUM(cut_seconds), 0),
			COALESCE(SUM(ai_studio_opens), 0),
			COALESCE(SUM(ai_videos_loads), 0),
			COALESCE(SUM(ai_analyzes), 0),
			COALESCE(SUM(ai_shots), 0),
			COALESCE(SUM(ai_pots), 0),
			COALESCE(SUM(ai_pool_runs), 0),
			COALESCE(SUM(ai_carom_runs), 0),
			COALESCE(SUM(ai_exports_ios), 0),
			COALESCE(SUM(ai_exports_desktop), 0),
			COALESCE(SUM(ai_export_relay), 0),
			COALESCE(SUM(ai_errors), 0),
			COALESCE(SUM(http_errors), 0),
			COALESCE(SUM(bytes_out), 0),
			COALESCE(SUM(os_ios), 0),
			COALESCE(SUM(os_android), 0),
			COALESCE(SUM(os_desktop), 0),
			COALESCE(SUM(os_app), 0),
			COALESCE(SUM(os_other), 0)
		FROM usage_box_hourly
		WHERE box_id = ? AND bucket_hour >= ? AND bucket_hour <= ?
	`, boxID, dateStr+" 00:00", dateStr+" 23:59").Scan(
		&hourlyCount, &hourlyMaxAI,
		&hourlySum.QRViews, &hourlySum.PageViews, &hourlySum.UniqueVisitors, &hourlySum.LiveHLS, &hourlySum.VODPlays, &hourlySum.ClipDownloads,
		&hourlySum.TimelineOpens, &hourlySum.CutClips, &hourlySum.CutSeconds,
		&hourlySum.AIStudioOpens, &hourlySum.AIVideosLoads, &hourlySum.AIAnalyzes, &hourlySum.AIShots, &hourlySum.AIPots,
		&hourlySum.AIPoolRuns, &hourlySum.AICaromRuns, &hourlySum.AIExportsIOS, &hourlySum.AIExportsDesktop, &hourlySum.AIExportRelay,
		&hourlySum.AIErrors, &hourlySum.HTTPErrors, &hourlySum.BytesOut,
		&hourlySum.OSIOS, &hourlySum.OSAndroid, &hourlySum.OSDesktop, &hourlySum.OSApp, &hourlySum.OSOther,
	)
	if err != nil {
		return false, fmt.Errorf("rollup hourly sum: %w", err)
	}

	var existingClub, existingSource, existingEpoch, existingBaseJSON string
	var existingAI int
	var existing AnalyticsBoxMetrics
	existingFound := true
	err = tx.QueryRow(`
		SELECT club_id, source, ai_enabled, source_epoch,
			qr_views, page_views, unique_visitors, live_hls, vod_plays, clip_downloads,
			timeline_opens, cut_clips, cut_seconds,
			ai_studio_opens, ai_videos_loads, ai_analyzes, ai_shots, ai_pots,
			ai_pool_runs, ai_carom_runs, ai_exports_ios, ai_exports_desktop, ai_export_relay,
			ai_errors, http_errors, bytes_out,
			os_ios, os_android, os_desktop, os_app, os_other,
			base_json
		FROM usage_box_daily
		WHERE box_id = ? AND bucket_date = ?
	`, boxID, dateStr).Scan(
		&existingClub, &existingSource, &existingAI, &existingEpoch,
		&existing.QRViews, &existing.PageViews, &existing.UniqueVisitors, &existing.LiveHLS, &existing.VODPlays, &existing.ClipDownloads,
		&existing.TimelineOpens, &existing.CutClips, &existing.CutSeconds,
		&existing.AIStudioOpens, &existing.AIVideosLoads, &existing.AIAnalyzes, &existing.AIShots, &existing.AIPots,
		&existing.AIPoolRuns, &existing.AICaromRuns, &existing.AIExportsIOS, &existing.AIExportsDesktop, &existing.AIExportRelay,
		&existing.AIErrors, &existing.HTTPErrors, &existing.BytesOut,
		&existing.OSIOS, &existing.OSAndroid, &existing.OSDesktop, &existing.OSApp, &existing.OSOther,
		&existingBaseJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		existingFound = false
	} else if err != nil {
		return false, fmt.Errorf("select usage_box_daily: %w", err)
	}

	// Avoid inserting empty phantom daily rows for old box_daily entries with zero traffic.
	if hourlyCount == 0 && !existingFound && (!hasExactUnique || exactUnique == 0) {
		return false, nil
	}

	effClubID := clubID
	if effClubID == "" {
		effClubID = existingClub
	}
	effSource := incomingSource
	if existingSource == "redbida-go" && incomingSource == "nodered" {
		effSource = "redbida-go"
	} else if effSource == "" {
		effSource = existingSource
	}
	effAI := existingAI
	if incomingAI != 0 || hourlyMaxAI != 0 || hourlySum.hasAIActivity() {
		effAI = 1
	}

	// Preserve 2-year daily totals even if hourly rows were pruned, by taking max(existing, hourlySum).
	merged := existing.maxWith(hourlySum)
	effEpoch := existingEpoch
	newBaseJSON := existingBaseJSON
	if newBaseJSON == "" {
		newBaseJSON = "{}"
	}

	if hasExactUnique {
		// Exact box-level daily unique visitors with epoch-safe base_json freezing.
		var dailyBase struct {
			UniqueVisitors int64 `json:"unique_visitors"`
		}
		if existingFound {
			_ = json.Unmarshal([]byte(existingBaseJSON), &dailyBase)
			if existingEpoch == incomingEpoch {
				merged.UniqueVisitors = max64(existing.UniqueVisitors, dailyBase.UniqueVisitors+exactUnique)
			} else {
				dailyBase.UniqueVisitors = existing.UniqueVisitors
				if bRaw, mErr := json.Marshal(dailyBase); mErr == nil {
					newBaseJSON = string(bRaw)
				}
				merged.UniqueVisitors = dailyBase.UniqueVisitors + exactUnique
				effEpoch = incomingEpoch
			}
		} else {
			merged.UniqueVisitors = exactUnique
			effEpoch = incomingEpoch
			newBaseJSON = "{}"
		}
	} else {
		if effEpoch == "" {
			effEpoch = incomingEpoch
		}
		merged.UniqueVisitors = max64(existing.UniqueVisitors, hourlySum.UniqueVisitors)
	}

	if err := upsertBoxDailyRowTx(tx, boxID, effClubID, effSource, effAI, effEpoch, dateStr, merged, newBaseJSON, nowStr); err != nil {
		return false, err
	}
	return true, nil
}

func upsertBoxHourlyRowTx(
	tx *sql.Tx,
	boxID, clubID, source string,
	aiEnabled int,
	sourceEpoch, bucketHour string,
	m AnalyticsBoxMetrics,
	baseJSON, updatedAt string,
) error {
	_, err := tx.Exec(`
		INSERT INTO usage_box_hourly (
			box_id, club_id, source, ai_enabled, source_epoch, bucket_hour,
			qr_views, page_views, unique_visitors, live_hls, vod_plays, clip_downloads,
			timeline_opens, cut_clips, cut_seconds,
			ai_studio_opens, ai_videos_loads, ai_analyzes, ai_shots, ai_pots,
			ai_pool_runs, ai_carom_runs, ai_exports_ios, ai_exports_desktop, ai_export_relay,
			ai_errors, http_errors, bytes_out,
			os_ios, os_android, os_desktop, os_app, os_other,
			base_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(box_id, bucket_hour) DO UPDATE SET
			club_id = excluded.club_id,
			source = excluded.source,
			ai_enabled = excluded.ai_enabled,
			source_epoch = excluded.source_epoch,
			qr_views = excluded.qr_views,
			page_views = excluded.page_views,
			unique_visitors = excluded.unique_visitors,
			live_hls = excluded.live_hls,
			vod_plays = excluded.vod_plays,
			clip_downloads = excluded.clip_downloads,
			timeline_opens = excluded.timeline_opens,
			cut_clips = excluded.cut_clips,
			cut_seconds = excluded.cut_seconds,
			ai_studio_opens = excluded.ai_studio_opens,
			ai_videos_loads = excluded.ai_videos_loads,
			ai_analyzes = excluded.ai_analyzes,
			ai_shots = excluded.ai_shots,
			ai_pots = excluded.ai_pots,
			ai_pool_runs = excluded.ai_pool_runs,
			ai_carom_runs = excluded.ai_carom_runs,
			ai_exports_ios = excluded.ai_exports_ios,
			ai_exports_desktop = excluded.ai_exports_desktop,
			ai_export_relay = excluded.ai_export_relay,
			ai_errors = excluded.ai_errors,
			http_errors = excluded.http_errors,
			bytes_out = excluded.bytes_out,
			os_ios = excluded.os_ios,
			os_android = excluded.os_android,
			os_desktop = excluded.os_desktop,
			os_app = excluded.os_app,
			os_other = excluded.os_other,
			base_json = excluded.base_json,
			updated_at = excluded.updated_at
	`, boxID, clubID, source, aiEnabled, sourceEpoch, bucketHour,
		m.QRViews, m.PageViews, m.UniqueVisitors, m.LiveHLS, m.VODPlays, m.ClipDownloads,
		m.TimelineOpens, m.CutClips, m.CutSeconds,
		m.AIStudioOpens, m.AIVideosLoads, m.AIAnalyzes, m.AIShots, m.AIPots,
		m.AIPoolRuns, m.AICaromRuns, m.AIExportsIOS, m.AIExportsDesktop, m.AIExportRelay,
		m.AIErrors, m.HTTPErrors, m.BytesOut,
		m.OSIOS, m.OSAndroid, m.OSDesktop, m.OSApp, m.OSOther,
		baseJSON, updatedAt)
	return err
}

func upsertBoxDailyRowTx(
	tx *sql.Tx,
	boxID, clubID, source string,
	aiEnabled int,
	sourceEpoch, bucketDate string,
	m AnalyticsBoxMetrics,
	baseJSON, updatedAt string,
) error {
	_, err := tx.Exec(`
		INSERT INTO usage_box_daily (
			box_id, club_id, source, ai_enabled, source_epoch, bucket_date,
			qr_views, page_views, unique_visitors, live_hls, vod_plays, clip_downloads,
			timeline_opens, cut_clips, cut_seconds,
			ai_studio_opens, ai_videos_loads, ai_analyzes, ai_shots, ai_pots,
			ai_pool_runs, ai_carom_runs, ai_exports_ios, ai_exports_desktop, ai_export_relay,
			ai_errors, http_errors, bytes_out,
			os_ios, os_android, os_desktop, os_app, os_other,
			base_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(box_id, bucket_date) DO UPDATE SET
			club_id = excluded.club_id,
			source = excluded.source,
			ai_enabled = excluded.ai_enabled,
			source_epoch = excluded.source_epoch,
			qr_views = excluded.qr_views,
			page_views = excluded.page_views,
			unique_visitors = excluded.unique_visitors,
			live_hls = excluded.live_hls,
			vod_plays = excluded.vod_plays,
			clip_downloads = excluded.clip_downloads,
			timeline_opens = excluded.timeline_opens,
			cut_clips = excluded.cut_clips,
			cut_seconds = excluded.cut_seconds,
			ai_studio_opens = excluded.ai_studio_opens,
			ai_videos_loads = excluded.ai_videos_loads,
			ai_analyzes = excluded.ai_analyzes,
			ai_shots = excluded.ai_shots,
			ai_pots = excluded.ai_pots,
			ai_pool_runs = excluded.ai_pool_runs,
			ai_carom_runs = excluded.ai_carom_runs,
			ai_exports_ios = excluded.ai_exports_ios,
			ai_exports_desktop = excluded.ai_exports_desktop,
			ai_export_relay = excluded.ai_export_relay,
			ai_errors = excluded.ai_errors,
			http_errors = excluded.http_errors,
			bytes_out = excluded.bytes_out,
			os_ios = excluded.os_ios,
			os_android = excluded.os_android,
			os_desktop = excluded.os_desktop,
			os_app = excluded.os_app,
			os_other = excluded.os_other,
			base_json = excluded.base_json,
			updated_at = excluded.updated_at
	`, boxID, clubID, source, aiEnabled, sourceEpoch, bucketDate,
		m.QRViews, m.PageViews, m.UniqueVisitors, m.LiveHLS, m.VODPlays, m.ClipDownloads,
		m.TimelineOpens, m.CutClips, m.CutSeconds,
		m.AIStudioOpens, m.AIVideosLoads, m.AIAnalyzes, m.AIShots, m.AIPots,
		m.AIPoolRuns, m.AICaromRuns, m.AIExportsIOS, m.AIExportsDesktop, m.AIExportRelay,
		m.AIErrors, m.HTTPErrors, m.BytesOut,
		m.OSIOS, m.OSAndroid, m.OSDesktop, m.OSApp, m.OSOther,
		baseJSON, updatedAt)
	return err
}

func upsertCameraDailyRowTx(
	tx *sql.Tx,
	boxID, clubID, cameraID, tableName, sourceEpoch, bucketDate string,
	m AnalyticsCamMetrics,
	baseJSON, updatedAt string,
) error {
	_, err := tx.Exec(`
		INSERT INTO usage_camera_daily (
			box_id, club_id, camera_id, table_name, source_epoch, bucket_date,
			qr_views, unique_visitors, live_hls, vod_plays, clip_downloads,
			timeline_opens, cut_clips, cut_seconds,
			ai_studio_opens, ai_analyzes, ai_shots, ai_pots, ai_exports, ai_errors, bytes_out,
			base_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(box_id, bucket_date, camera_id) DO UPDATE SET
			club_id = excluded.club_id,
			table_name = excluded.table_name,
			source_epoch = excluded.source_epoch,
			qr_views = excluded.qr_views,
			unique_visitors = excluded.unique_visitors,
			live_hls = excluded.live_hls,
			vod_plays = excluded.vod_plays,
			clip_downloads = excluded.clip_downloads,
			timeline_opens = excluded.timeline_opens,
			cut_clips = excluded.cut_clips,
			cut_seconds = excluded.cut_seconds,
			ai_studio_opens = excluded.ai_studio_opens,
			ai_analyzes = excluded.ai_analyzes,
			ai_shots = excluded.ai_shots,
			ai_pots = excluded.ai_pots,
			ai_exports = excluded.ai_exports,
			ai_errors = excluded.ai_errors,
			bytes_out = excluded.bytes_out,
			base_json = excluded.base_json,
			updated_at = excluded.updated_at
	`, boxID, clubID, cameraID, tableName, sourceEpoch, bucketDate,
		m.QRViews, m.UniqueVisitors, m.LiveHLS, m.VODPlays, m.ClipDownloads,
		m.TimelineOpens, m.CutClips, m.CutSeconds,
		m.AIStudioOpens, m.AIAnalyzes, m.AIShots, m.AIPots, m.AIExports, m.AIErrors, m.BytesOut,
		baseJSON, updatedAt)
	return err
}

// PruneAnalytics enforces retention windows:
//   - usage_box_hourly: 90 days (GMT+7)
//   - usage_box_daily & usage_camera_daily: 730 days (2 years, GMT+7)
//   - analytics_ingest_batches: 14 days (UTC)
func (s *Store) PruneAnalytics(now time.Time) (AnalyticsPruneStats, error) {
	var stats AnalyticsPruneStats
	nowLocal := now.In(analyticsTZ)
	hourlyCutoff := nowLocal.AddDate(0, 0, -analyticsHourlyKeepDays).Format("2006-01-02 15:00")
	dailyCutoff := nowLocal.AddDate(0, 0, -analyticsDailyKeepDays).Format("2006-01-02")
	batchCutoff := now.UTC().AddDate(0, 0, -analyticsBatchKeepDays).Format(time.RFC3339)

	err := s.WithSQLiteTx(func(tx *sql.Tx) error {
		if res, err := tx.Exec("DELETE FROM usage_box_hourly WHERE bucket_hour < ?", hourlyCutoff); err != nil {
			return err
		} else if n, _ := res.RowsAffected(); n > 0 {
			stats.HourlyDeleted = n
		}
		if res, err := tx.Exec("DELETE FROM usage_box_daily WHERE bucket_date < ?", dailyCutoff); err != nil {
			return err
		} else if n, _ := res.RowsAffected(); n > 0 {
			stats.DailyDeleted = n
		}
		if res, err := tx.Exec("DELETE FROM usage_camera_daily WHERE bucket_date < ?", dailyCutoff); err != nil {
			return err
		} else if n, _ := res.RowsAffected(); n > 0 {
			stats.CameraDeleted = n
		}
		if res, err := tx.Exec("DELETE FROM analytics_ingest_batches WHERE received_at < ?", batchCutoff); err != nil {
			return err
		} else if n, _ := res.RowsAffected(); n > 0 {
			stats.BatchesDeleted = n
		}
		return nil
	})
	return stats, err
}

func (s *Server) analyticsPrunerLoop() {
	initTimer := time.NewTimer(2 * time.Minute)
	defer initTimer.Stop()
	select {
	case <-s.stopCh:
		return
	case <-initTimer.C:
		if stats, err := s.store.PruneAnalytics(time.Now()); err == nil &&
			(stats.HourlyDeleted > 0 || stats.DailyDeleted > 0 || stats.CameraDeleted > 0 || stats.BatchesDeleted > 0) {
			log.Printf("[bidahub-analytics] pruned hourly=%d daily=%d camera=%d batches=%d",
				stats.HourlyDeleted, stats.DailyDeleted, stats.CameraDeleted, stats.BatchesDeleted)
		}
	}

	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if stats, err := s.store.PruneAnalytics(time.Now()); err == nil &&
				(stats.HourlyDeleted > 0 || stats.DailyDeleted > 0 || stats.CameraDeleted > 0 || stats.BatchesDeleted > 0) {
				log.Printf("[bidahub-analytics] pruned hourly=%d daily=%d camera=%d batches=%d",
					stats.HourlyDeleted, stats.DailyDeleted, stats.CameraDeleted, stats.BatchesDeleted)
			}
		}
	}
}

type analyticsMetadataSnapshot struct {
	boxes map[string]*Box
	clubs map[string]*Club
}

func (s *Server) snapshotAnalyticsMetadata() analyticsMetadataSnapshot {
	rawBoxes := s.store.Boxes()
	rawClubs := s.store.Clubs()
	snap := analyticsMetadataSnapshot{
		boxes: make(map[string]*Box, len(rawBoxes)),
		clubs: make(map[string]*Club, len(rawClubs)),
	}
	for _, b := range rawBoxes {
		if b != nil {
			snap.boxes[b.ID] = b
		}
	}
	for _, c := range rawClubs {
		if c != nil {
			snap.clubs[c.ID] = c
		}
	}
	return snap
}

func (m analyticsMetadataSnapshot) resolveNames(boxID, storedClubID string) (boxName, clubID, clubName string) {
	boxName = boxID
	clubID = storedClubID
	if b, ok := m.boxes[boxID]; ok && b != nil {
		if b.Name != "" {
			boxName = b.Name
		}
		if b.ClubID != "" {
			clubID = b.ClubID
		}
		if b.ClubName != "" {
			clubName = b.ClubName
		}
	}
	if clubID != "" {
		if c, ok := m.clubs[clubID]; ok && c != nil && c.Name != "" {
			clubName = c.Name
		}
	}
	if clubName == "" {
		if clubID != "" {
			clubName = clubID
		} else {
			clubName = boxName
		}
	}
	return boxName, clubID, clubName
}

func (m analyticsMetadataSnapshot) resolveTableName(boxID, cameraID, storedTableName string) string {
	if strings.TrimSpace(storedTableName) != "" {
		return strings.TrimSpace(storedTableName)
	}
	b, ok := m.boxes[boxID]
	if !ok || b == nil {
		return ""
	}
	for _, cam := range b.Cameras {
		if cam != nil && strings.EqualFold(cam.Name, cameraID) && cam.TableName != "" {
			return cam.TableName
		}
	}
	for k, v := range b.RedBidaTabs {
		if strings.EqualFold(strings.TrimSpace(k), cameraID) && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

type analyticsRangeWindow struct {
	Range       string
	FromDate    string
	ToDate      string
	FromHour    string
	ToHour      string
	Dates       []string
	UseHourly24 bool
}

func computeAnalyticsWindow(rng string, now time.Time) analyticsRangeWindow {
	rng = normalizeAnalyticsRange(rng)
	nowLocal := now.In(analyticsTZ)

	if rng == "24h" {
		endHour := nowLocal.Truncate(time.Hour)
		startHour := endHour.Add(-23 * time.Hour)
		fromDate := startHour.Format("2006-01-02")
		toDate := endHour.Format("2006-01-02")
		dates := []string{fromDate}
		if toDate != fromDate {
			dates = append(dates, toDate)
		}
		return analyticsRangeWindow{
			Range:       "24h",
			FromDate:    fromDate,
			ToDate:      toDate,
			FromHour:    startHour.Format("2006-01-02 15:00"),
			ToHour:      endHour.Format("2006-01-02 15:00"),
			Dates:       dates,
			UseHourly24: true,
		}
	}

	days := 7
	switch rng {
	case "30d":
		days = 30
	case "90d":
		days = 90
	}
	toDay := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, analyticsTZ)
	fromDay := toDay.AddDate(0, 0, -(days - 1))
	dates := make([]string, 0, days)
	for d := fromDay; !d.After(toDay); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}
	fromDate := fromDay.Format("2006-01-02")
	toDate := toDay.Format("2006-01-02")
	return analyticsRangeWindow{
		Range:       rng,
		FromDate:    fromDate,
		ToDate:      toDate,
		FromHour:    fromDate + " 00:00",
		ToHour:      toDate + " 23:00",
		Dates:       dates,
		UseHourly24: false,
	}
}

type boxUsageRow struct {
	BoxID      string
	ClubID     string
	Source     string
	AIEnabled  bool
	BucketKey  string // bucket_hour or bucket_date
	UpdatedAt  string
	Metrics    AnalyticsBoxMetrics
}

func (s *Server) queryAnalyticsOverview(q analyticsQueryParams, now time.Time) (AnalyticsOverviewResponse, error) {
	win := computeAnalyticsWindow(q.Range, now)
	meta := s.snapshotAnalyticsMetadata()

	peakHours := make([]AnalyticsPeakHour, 24)
	for h := 0; h < 24; h++ {
		peakHours[h] = AnalyticsPeakHour{Hour: h}
	}
	dailySeries := make([]AnalyticsDailyPoint, len(win.Dates))
	dailyIndex := make(map[string]int, len(win.Dates))
	for i, d := range win.Dates {
		dailySeries[i] = AnalyticsDailyPoint{Date: d}
		dailyIndex[d] = i
	}

	resp := AnalyticsOverviewResponse{
		OK:          true,
		Range:       win.Range,
		FromDate:    win.FromDate,
		ToDate:      win.ToDate,
		PeakHours:   peakHours,
		DailySeries: dailySeries,
		TopClubs:    make([]AnalyticsTopClub, 0),
		TopBoxes:    make([]AnalyticsTopBox, 0),
	}

	var mainRows []boxUsageRow
	var hourlyRows []boxUsageRow

	err := s.store.WithSQLiteDB(func(db *sql.DB) error {
		var err error
		if win.UseHourly24 {
			hourlyRows, err = queryBoxHourlyRows(db, win.FromHour, win.ToHour, q.BoxID)
			if err != nil {
				return err
			}
			mainRows = hourlyRows
		} else {
			mainRows, err = queryBoxDailyRows(db, win.FromDate, win.ToDate, q.BoxID)
			if err != nil {
				return err
			}
			hourlyRows, err = queryBoxHourlyRows(db, win.FromHour, win.ToHour, q.BoxID)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errNoSQLiteDB) {
			return resp, nil
		}
		return resp, err
	}

	filterClub := strings.TrimSpace(q.ClubID)
	filterBox := strings.TrimSpace(q.BoxID)

	matchesFilters := func(row boxUsageRow) (boxName, clubID, clubName string, ok bool) {
		if filterBox != "" && row.BoxID != filterBox {
			return "", "", "", false
		}
		boxName, clubID, clubName = meta.resolveNames(row.BoxID, row.ClubID)
		if filterClub != "" && clubID != filterClub {
			return "", "", "", false
		}
		if q.AIOnly && !row.AIEnabled && !row.Metrics.hasAIActivity() {
			return "", "", "", false
		}
		return boxName, clubID, clubName, true
	}

	// Aggregate Peak Hours (always from hourlyRows, GMT+7 0..23).
	for _, row := range hourlyRows {
		if _, _, _, ok := matchesFilters(row); !ok {
			continue
		}
		_, _, hr, valid := parseBucketHour(row.BucketKey)
		if !valid || hr < 0 || hr > 23 {
			continue
		}
		ph := &resp.PeakHours[hr]
		ph.QRViews += row.Metrics.QRViews
		ph.DownloadsCuts += row.Metrics.ClipDownloads + row.Metrics.CutClips
		ph.AIRuns += max64(row.Metrics.AIAnalyzes, row.Metrics.AIPoolRuns+row.Metrics.AICaromRuns)
		ph.UniqueVisitors += row.Metrics.UniqueVisitors
		ph.BytesOut += row.Metrics.BytesOut
	}

	// Aggregate KPI, OS breakdown, Daily Series, Top Clubs, Top Boxes from mainRows.
	activeBoxes := make(map[string]struct{})
	activeClubs := make(map[string]struct{})
	aiActiveBoxes := make(map[string]struct{})

	type boxAgg struct {
		boxID        string
		boxName      string
		clubID       string
		clubName     string
		source       string
		aiEnabled    bool
		metrics      AnalyticsBoxMetrics
		lastIngestAt string
	}
	type clubAgg struct {
		clubID       string
		clubName     string
		boxes        map[string]struct{}
		sources      map[string]struct{}
		aiEnabled    bool
		metrics      AnalyticsBoxMetrics
		lastIngestAt string
	}

	boxAggs := make(map[string]*boxAgg)
	clubAggs := make(map[string]*clubAgg)

	for _, row := range mainRows {
		boxName, clubID, clubName, ok := matchesFilters(row)
		if !ok {
			continue
		}
		m := row.Metrics
		aiFlag := row.AIEnabled || m.hasAIActivity()

		resp.KPI.QRViews += m.QRViews
		resp.KPI.PageViews += m.PageViews
		resp.KPI.UniqueVisitors += m.UniqueVisitors
		resp.KPI.LiveHLS += m.LiveHLS
		resp.KPI.VODPlays += m.VODPlays
		resp.KPI.ClipDownloads += m.ClipDownloads
		resp.KPI.TimelineOpens += m.TimelineOpens
		resp.KPI.CutClips += m.CutClips
		resp.KPI.CutSeconds += m.CutSeconds
		resp.KPI.AIStudioOpens += m.AIStudioOpens
		resp.KPI.AIVideosLoads += m.AIVideosLoads
		resp.KPI.AIAnalyzes += m.AIAnalyzes
		resp.KPI.AIShots += m.AIShots
		resp.KPI.AIPots += m.AIPots
		resp.KPI.AIPoolRuns += m.AIPoolRuns
		resp.KPI.AICaromRuns += m.AICaromRuns
		resp.KPI.AIExportsIOS += m.AIExportsIOS
		resp.KPI.AIExportsDesktop += m.AIExportsDesktop
		resp.KPI.AIExportRelay += m.AIExportRelay
		resp.KPI.AIErrors += m.AIErrors
		resp.KPI.HTTPErrors += m.HTTPErrors
		resp.KPI.BytesOut += m.BytesOut

		resp.OSBreakdown.IOS += m.OSIOS
		resp.OSBreakdown.Android += m.OSAndroid
		resp.OSBreakdown.Desktop += m.OSDesktop
		resp.OSBreakdown.App += m.OSApp
		resp.OSBreakdown.Other += m.OSOther

		activeBoxes[row.BoxID] = struct{}{}
		if aiFlag {
			aiActiveBoxes[row.BoxID] = struct{}{}
		}
		clubKey := clubID
		if clubKey == "" {
			clubKey = row.BoxID
		}
		activeClubs[clubKey] = struct{}{}

		// Daily series accumulation
		dateKey := row.BucketKey
		if len(dateKey) >= 10 {
			dateKey = dateKey[:10]
		}
		if idx, exists := dailyIndex[dateKey]; exists {
			dp := &resp.DailySeries[idx]
			dp.QRViews += m.QRViews
			dp.UniqueVisitors += m.UniqueVisitors
			dp.DownloadsCuts += m.ClipDownloads + m.CutClips
			dp.AIAnalyzes += m.AIAnalyzes
			dp.AIExports += m.aiExportsTotal()
			dp.BytesOut += m.BytesOut
		}

		// Per-box aggregation
		ba, exists := boxAggs[row.BoxID]
		if !exists {
			ba = &boxAgg{
				boxID:    row.BoxID,
				boxName:  boxName,
				clubID:   clubID,
				clubName: clubName,
				source:   row.Source,
			}
			boxAggs[row.BoxID] = ba
		}
		if row.Source == "redbida-go" || ba.source == "" {
			ba.source = row.Source
		}
		if aiFlag {
			ba.aiEnabled = true
		}
		ba.metrics = ba.metrics.add(m)
		if row.UpdatedAt > ba.lastIngestAt {
			ba.lastIngestAt = row.UpdatedAt
		}

		// Per-club aggregation
		ca, exists := clubAggs[clubKey]
		if !exists {
			ca = &clubAgg{
				clubID:   clubID,
				clubName: clubName,
				boxes:    make(map[string]struct{}),
				sources:  make(map[string]struct{}),
			}
			clubAggs[clubKey] = ca
		}
		ca.boxes[row.BoxID] = struct{}{}
		if row.Source != "" {
			ca.sources[row.Source] = struct{}{}
		}
		if aiFlag {
			ca.aiEnabled = true
		}
		ca.metrics = ca.metrics.add(m)
		if row.UpdatedAt > ca.lastIngestAt {
			ca.lastIngestAt = row.UpdatedAt
		}
	}

	resp.KPI.ActiveBoxes = len(activeBoxes)
	resp.KPI.ActiveClubs = len(activeClubs)
	resp.KPI.AIActiveBoxes = len(aiActiveBoxes)

	for _, ca := range clubAggs {
		srcs := sortedKeys(ca.sources)
		resp.TopClubs = append(resp.TopClubs, AnalyticsTopClub{
			ClubID:         ca.clubID,
			ClubName:       ca.clubName,
			BoxesCount:     len(ca.boxes),
			Sources:        srcs,
			AIEnabled:      ca.aiEnabled,
			QRViews:        ca.metrics.QRViews,
			UniqueVisitors: ca.metrics.UniqueVisitors,
			LiveHLS:        ca.metrics.LiveHLS,
			VODPlays:       ca.metrics.VODPlays,
			ClipDownloads:  ca.metrics.ClipDownloads,
			CutClips:       ca.metrics.CutClips,
			AIStudioOpens:  ca.metrics.AIStudioOpens,
			AIAnalyzes:     ca.metrics.AIAnalyzes,
			AIShots:        ca.metrics.AIShots,
			AIExports:      ca.metrics.aiExportsTotal(),
			AIErrors:       ca.metrics.AIErrors,
			HTTPErrors:     ca.metrics.HTTPErrors,
			BytesOut:       ca.metrics.BytesOut,
			LastIngestAt:   ca.lastIngestAt,
		})
	}
	sort.Slice(resp.TopClubs, func(i, j int) bool {
		if resp.TopClubs[i].QRViews != resp.TopClubs[j].QRViews {
			return resp.TopClubs[i].QRViews > resp.TopClubs[j].QRViews
		}
		di := resp.TopClubs[i].ClipDownloads + resp.TopClubs[i].CutClips + resp.TopClubs[i].AIAnalyzes
		dj := resp.TopClubs[j].ClipDownloads + resp.TopClubs[j].CutClips + resp.TopClubs[j].AIAnalyzes
		if di != dj {
			return di > dj
		}
		return resp.TopClubs[i].ClubName < resp.TopClubs[j].ClubName
	})

	for _, ba := range boxAggs {
		resp.TopBoxes = append(resp.TopBoxes, AnalyticsTopBox{
			BoxID:          ba.boxID,
			BoxName:        ba.boxName,
			ClubID:         ba.clubID,
			ClubName:       ba.clubName,
			Source:         ba.source,
			AIEnabled:      ba.aiEnabled,
			QRViews:        ba.metrics.QRViews,
			UniqueVisitors: ba.metrics.UniqueVisitors,
			LiveHLS:        ba.metrics.LiveHLS,
			VODPlays:       ba.metrics.VODPlays,
			ClipDownloads:  ba.metrics.ClipDownloads,
			CutClips:       ba.metrics.CutClips,
			AIStudioOpens:  ba.metrics.AIStudioOpens,
			AIAnalyzes:     ba.metrics.AIAnalyzes,
			AIShots:        ba.metrics.AIShots,
			AIExports:      ba.metrics.aiExportsTotal(),
			AIErrors:       ba.metrics.AIErrors,
			HTTPErrors:     ba.metrics.HTTPErrors,
			BytesOut:       ba.metrics.BytesOut,
			LastIngestAt:   ba.lastIngestAt,
		})
	}
	sort.Slice(resp.TopBoxes, func(i, j int) bool {
		if resp.TopBoxes[i].QRViews != resp.TopBoxes[j].QRViews {
			return resp.TopBoxes[i].QRViews > resp.TopBoxes[j].QRViews
		}
		di := resp.TopBoxes[i].ClipDownloads + resp.TopBoxes[i].CutClips + resp.TopBoxes[i].AIAnalyzes
		dj := resp.TopBoxes[j].ClipDownloads + resp.TopBoxes[j].CutClips + resp.TopBoxes[j].AIAnalyzes
		if di != dj {
			return di > dj
		}
		return resp.TopBoxes[i].BoxID < resp.TopBoxes[j].BoxID
	})

	return resp, nil
}

func queryBoxHourlyRows(db *sql.DB, fromHour, toHour, boxID string) ([]boxUsageRow, error) {
	query := `
		SELECT box_id, club_id, source, ai_enabled, bucket_hour, updated_at,
			qr_views, page_views, unique_visitors, live_hls, vod_plays, clip_downloads,
			timeline_opens, cut_clips, cut_seconds,
			ai_studio_opens, ai_videos_loads, ai_analyzes, ai_shots, ai_pots,
			ai_pool_runs, ai_carom_runs, ai_exports_ios, ai_exports_desktop, ai_export_relay,
			ai_errors, http_errors, bytes_out,
			os_ios, os_android, os_desktop, os_app, os_other
		FROM usage_box_hourly
		WHERE bucket_hour >= ? AND bucket_hour <= ?
	`
	args := []any{fromHour, toHour}
	if strings.TrimSpace(boxID) != "" {
		query += " AND box_id = ?"
		args = append(args, strings.TrimSpace(boxID))
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []boxUsageRow
	for rows.Next() {
		var r boxUsageRow
		var aiInt int
		var updRaw any
		if err := rows.Scan(
			&r.BoxID, &r.ClubID, &r.Source, &aiInt, &r.BucketKey, &updRaw,
			&r.Metrics.QRViews, &r.Metrics.PageViews, &r.Metrics.UniqueVisitors, &r.Metrics.LiveHLS, &r.Metrics.VODPlays, &r.Metrics.ClipDownloads,
			&r.Metrics.TimelineOpens, &r.Metrics.CutClips, &r.Metrics.CutSeconds,
			&r.Metrics.AIStudioOpens, &r.Metrics.AIVideosLoads, &r.Metrics.AIAnalyzes, &r.Metrics.AIShots, &r.Metrics.AIPots,
			&r.Metrics.AIPoolRuns, &r.Metrics.AICaromRuns, &r.Metrics.AIExportsIOS, &r.Metrics.AIExportsDesktop, &r.Metrics.AIExportRelay,
			&r.Metrics.AIErrors, &r.Metrics.HTTPErrors, &r.Metrics.BytesOut,
			&r.Metrics.OSIOS, &r.Metrics.OSAndroid, &r.Metrics.OSDesktop, &r.Metrics.OSApp, &r.Metrics.OSOther,
		); err != nil {
			return nil, err
		}
		r.AIEnabled = aiInt != 0
		r.UpdatedAt = formatSQLTimestamp(updRaw)
		out = append(out, r)
	}
	return out, rows.Err()
}

func queryBoxDailyRows(db *sql.DB, fromDate, toDate, boxID string) ([]boxUsageRow, error) {
	query := `
		SELECT box_id, club_id, source, ai_enabled, bucket_date, updated_at,
			qr_views, page_views, unique_visitors, live_hls, vod_plays, clip_downloads,
			timeline_opens, cut_clips, cut_seconds,
			ai_studio_opens, ai_videos_loads, ai_analyzes, ai_shots, ai_pots,
			ai_pool_runs, ai_carom_runs, ai_exports_ios, ai_exports_desktop, ai_export_relay,
			ai_errors, http_errors, bytes_out,
			os_ios, os_android, os_desktop, os_app, os_other
		FROM usage_box_daily
		WHERE bucket_date >= ? AND bucket_date <= ?
	`
	args := []any{fromDate, toDate}
	if strings.TrimSpace(boxID) != "" {
		query += " AND box_id = ?"
		args = append(args, strings.TrimSpace(boxID))
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []boxUsageRow
	for rows.Next() {
		var r boxUsageRow
		var aiInt int
		var updRaw any
		if err := rows.Scan(
			&r.BoxID, &r.ClubID, &r.Source, &aiInt, &r.BucketKey, &updRaw,
			&r.Metrics.QRViews, &r.Metrics.PageViews, &r.Metrics.UniqueVisitors, &r.Metrics.LiveHLS, &r.Metrics.VODPlays, &r.Metrics.ClipDownloads,
			&r.Metrics.TimelineOpens, &r.Metrics.CutClips, &r.Metrics.CutSeconds,
			&r.Metrics.AIStudioOpens, &r.Metrics.AIVideosLoads, &r.Metrics.AIAnalyzes, &r.Metrics.AIShots, &r.Metrics.AIPots,
			&r.Metrics.AIPoolRuns, &r.Metrics.AICaromRuns, &r.Metrics.AIExportsIOS, &r.Metrics.AIExportsDesktop, &r.Metrics.AIExportRelay,
			&r.Metrics.AIErrors, &r.Metrics.HTTPErrors, &r.Metrics.BytesOut,
			&r.Metrics.OSIOS, &r.Metrics.OSAndroid, &r.Metrics.OSDesktop, &r.Metrics.OSApp, &r.Metrics.OSOther,
		); err != nil {
			return nil, err
		}
		r.AIEnabled = aiInt != 0
		r.UpdatedAt = formatSQLTimestamp(updRaw)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Server) queryAnalyticsCameras(q analyticsCameraQueryParams, now time.Time) ([]AnalyticsCameraRow, error) {
	win := computeAnalyticsWindow(q.Range, now)
	meta := s.snapshotAnalyticsMetadata()

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	type rawCamRow struct {
		boxID     string
		clubID    string
		cameraID  string
		tableName string
		metrics   AnalyticsCamMetrics
	}
	var rawRows []rawCamRow

	err := s.store.WithSQLiteDB(func(db *sql.DB) error {
		query := `
			SELECT box_id, MAX(club_id), camera_id, MAX(table_name),
				COALESCE(SUM(qr_views), 0),
				COALESCE(SUM(unique_visitors), 0),
				COALESCE(SUM(live_hls), 0),
				COALESCE(SUM(vod_plays), 0),
				COALESCE(SUM(clip_downloads), 0),
				COALESCE(SUM(timeline_opens), 0),
				COALESCE(SUM(cut_clips), 0),
				COALESCE(SUM(cut_seconds), 0),
				COALESCE(SUM(ai_studio_opens), 0),
				COALESCE(SUM(ai_analyzes), 0),
				COALESCE(SUM(ai_shots), 0),
				COALESCE(SUM(ai_pots), 0),
				COALESCE(SUM(ai_exports), 0),
				COALESCE(SUM(ai_errors), 0),
				COALESCE(SUM(bytes_out), 0)
			FROM usage_camera_daily
			WHERE bucket_date >= ? AND bucket_date <= ?
		`
		args := []any{win.FromDate, win.ToDate}
		if strings.TrimSpace(q.BoxID) != "" {
			query += " AND box_id = ?"
			args = append(args, strings.TrimSpace(q.BoxID))
		}
		query += " GROUP BY box_id, camera_id"

		rows, err := db.Query(query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var r rawCamRow
			if err := rows.Scan(
				&r.boxID, &r.clubID, &r.cameraID, &r.tableName,
				&r.metrics.QRViews, &r.metrics.UniqueVisitors, &r.metrics.LiveHLS, &r.metrics.VODPlays, &r.metrics.ClipDownloads,
				&r.metrics.TimelineOpens, &r.metrics.CutClips, &r.metrics.CutSeconds,
				&r.metrics.AIStudioOpens, &r.metrics.AIAnalyzes, &r.metrics.AIShots, &r.metrics.AIPots, &r.metrics.AIExports, &r.metrics.AIErrors, &r.metrics.BytesOut,
			); err != nil {
				return err
			}
			rawRows = append(rawRows, r)
		}
		return rows.Err()
	})
	if err != nil {
		if errors.Is(err, errNoSQLiteDB) {
			return make([]AnalyticsCameraRow, 0), nil
		}
		return nil, err
	}

	filterClub := strings.TrimSpace(q.ClubID)
	filterBox := strings.TrimSpace(q.BoxID)

	out := make([]AnalyticsCameraRow, 0, len(rawRows))
	for _, r := range rawRows {
		if filterBox != "" && r.boxID != filterBox {
			continue
		}
		boxName, clubID, clubName := meta.resolveNames(r.boxID, r.clubID)
		if filterClub != "" && clubID != filterClub {
			continue
		}
		tableName := meta.resolveTableName(r.boxID, r.cameraID, r.tableName)
		out = append(out, AnalyticsCameraRow{
			BoxID:          r.boxID,
			BoxName:        boxName,
			ClubID:         clubID,
			ClubName:       clubName,
			CameraID:       r.cameraID,
			TableName:      tableName,
			QRViews:        r.metrics.QRViews,
			UniqueVisitors: r.metrics.UniqueVisitors,
			LiveHLS:        r.metrics.LiveHLS,
			VODPlays:       r.metrics.VODPlays,
			ClipDownloads:  r.metrics.ClipDownloads,
			TimelineOpens:  r.metrics.TimelineOpens,
			CutClips:       r.metrics.CutClips,
			CutSeconds:     r.metrics.CutSeconds,
			AIStudioOpens:  r.metrics.AIStudioOpens,
			AIAnalyzes:     r.metrics.AIAnalyzes,
			AIShots:        r.metrics.AIShots,
			AIPots:         r.metrics.AIPots,
			AIExports:      r.metrics.AIExports,
			AIErrors:       r.metrics.AIErrors,
			BytesOut:       r.metrics.BytesOut,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].QRViews != out[j].QRViews {
			return out[i].QRViews > out[j].QRViews
		}
		ai := out[i].ClipDownloads + out[i].CutClips + out[i].AIAnalyzes
		aj := out[j].ClipDownloads + out[j].CutClips + out[j].AIAnalyzes
		if ai != aj {
			return ai > aj
		}
		if out[i].UniqueVisitors != out[j].UniqueVisitors {
			return out[i].UniqueVisitors > out[j].UniqueVisitors
		}
		if out[i].BoxID != out[j].BoxID {
			return out[i].BoxID < out[j].BoxID
		}
		return out[i].CameraID < out[j].CameraID
	})

	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// handleAnalyticsOverview handles GET /fleet/api/analytics/overview.
func (s *Server) handleAnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	q := r.URL.Query()
	aiOnlyRaw := strings.ToLower(strings.TrimSpace(q.Get("ai_only")))
	aiOnly := aiOnlyRaw == "1" || aiOnlyRaw == "true"

	resp, err := s.queryAnalyticsOverview(analyticsQueryParams{
		Range:  q.Get("range"),
		ClubID: q.Get("club_id"),
		BoxID:  q.Get("box_id"),
		AIOnly: aiOnly,
	}, time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "analytics_query_failed", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleAnalyticsCameras handles GET /fleet/api/analytics/cameras.
func (s *Server) handleAnalyticsCameras(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	q := r.URL.Query()
	rng := normalizeAnalyticsRange(q.Get("range"))
	limit, _ := strconv.Atoi(strings.TrimSpace(q.Get("limit")))
	cams, err := s.queryAnalyticsCameras(analyticsCameraQueryParams{
		Range:  rng,
		ClubID: q.Get("club_id"),
		BoxID:  q.Get("box_id"),
		Limit:  limit,
	}, time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "analytics_cameras_failed", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, AnalyticsCamerasResponse{
		OK:      true,
		Range:   rng,
		Cameras: cams,
	})
}

// handleAnalyticsExportCSV handles GET /fleet/api/analytics/export.csv (UTF-8 BOM Excel-friendly CSV).
func (s *Server) handleAnalyticsExportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	q := r.URL.Query()
	aiOnlyRaw := strings.ToLower(strings.TrimSpace(q.Get("ai_only")))
	aiOnly := aiOnlyRaw == "1" || aiOnlyRaw == "true"

	ov, err := s.queryAnalyticsOverview(analyticsQueryParams{
		Range:  q.Get("range"),
		ClubID: q.Get("club_id"),
		BoxID:  q.Get("box_id"),
		AIOnly: aiOnly,
	}, time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "analytics_export_failed", "detail": err.Error()})
		return
	}

	var buf bytes.Buffer
	// UTF-8 BOM for direct Microsoft Excel compatibility with Vietnamese club/box names.
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	cw := csv.NewWriter(&buf)
	_ = cw.Write([]string{
		"club_id", "club_name", "box_id", "box_name", "source", "ai_enabled",
		"qr_views", "unique_visitors", "live_hls", "vod_plays",
		"clip_downloads", "cut_clips", "ai_studio_opens", "ai_analyzes",
		"ai_shots", "ai_exports", "ai_errors", "http_errors",
		"bytes_out", "last_ingest_at",
	})
	for _, b := range ov.TopBoxes {
		aiStr := "0"
		if b.AIEnabled {
			aiStr = "1"
		}
		_ = cw.Write([]string{
			b.ClubID,
			b.ClubName,
			b.BoxID,
			b.BoxName,
			b.Source,
			aiStr,
			strconv.FormatInt(b.QRViews, 10),
			strconv.FormatInt(b.UniqueVisitors, 10),
			strconv.FormatInt(b.LiveHLS, 10),
			strconv.FormatInt(b.VODPlays, 10),
			strconv.FormatInt(b.ClipDownloads, 10),
			strconv.FormatInt(b.CutClips, 10),
			strconv.FormatInt(b.AIStudioOpens, 10),
			strconv.FormatInt(b.AIAnalyzes, 10),
			strconv.FormatInt(b.AIShots, 10),
			strconv.FormatInt(b.AIExports, 10),
			strconv.FormatInt(b.AIErrors, 10),
			strconv.FormatInt(b.HTTPErrors, 10),
			strconv.FormatInt(b.BytesOut, 10),
			b.LastIngestAt,
		})
	}
	cw.Flush()

	filename := fmt.Sprintf("inut-bida-analytics-%s-%s.csv", ov.Range, ov.ToDate)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func formatSQLTimestamp(v any) string {
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	case string:
		return strings.TrimSpace(t)
	case []byte:
		return strings.TrimSpace(string(t))
	default:
		return ""
	}
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
