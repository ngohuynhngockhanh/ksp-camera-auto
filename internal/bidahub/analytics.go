package bidahub

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// analyticsTZ is fixed GMT+7 (Asia/Ho_Chi_Minh) with zero external tzdata dependency.
var analyticsTZ = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)

const (
	analyticsMaxIngestBytes          = 2 << 20 // 2 MiB
	analyticsHourlyKeepDays          = 90
	analyticsDailyKeepDays           = 730 // 2 years
	analyticsBatchKeepDays           = 14
	realtimeEntryTTL                 = 90 * time.Second
	defaultAnalyticsCoalesceWindow   = 2 * time.Second
	defaultAnalyticsCoalesceMaxBatch = 32
	analyticsCoalesceQueueCap        = 512
	analyticsCustomMaxDays           = 366
)

var vietnameseWeekdays = []string{"T2", "T3", "T4", "T5", "T6", "T7", "CN"}

// AnalyticsBoxMetrics holds the 35 box-level usage counters shared by hourly/daily rollups and base_json.
type AnalyticsBoxMetrics struct {
	QRViews           int64 `json:"qr_views"`
	PageViews         int64 `json:"page_views"`
	UniqueVisitors    int64 `json:"unique_visitors"`
	LiveHLS           int64 `json:"live_hls"`
	VODPlays          int64 `json:"vod_plays"`
	ClipDownloads     int64 `json:"clip_downloads"`
	TimelineOpens     int64 `json:"timeline_opens"`
	CutClips          int64 `json:"cut_clips"`
	CutSeconds        int64 `json:"cut_seconds"`
	AIStudioOpens     int64 `json:"ai_studio_opens"`
	AIVideosLoads     int64 `json:"ai_videos_loads"`
	AIAnalyzes        int64 `json:"ai_analyzes"`
	AIShots           int64 `json:"ai_shots"`
	AIPots            int64 `json:"ai_pots"`
	AIPoolRuns        int64 `json:"ai_pool_runs"`
	AICaromRuns       int64 `json:"ai_carom_runs"`
	AIPickleballRuns  int64 `json:"ai_pickleball_runs,omitempty"`
	AIExportsIOS      int64 `json:"ai_exports_ios"`
	AIExportsDesktop  int64 `json:"ai_exports_desktop"`
	AIExportRelay     int64 `json:"ai_export_relay"`
	AIErrors          int64 `json:"ai_errors"`
	HTTPErrors        int64 `json:"http_errors"`
	BytesOut          int64 `json:"bytes_out"`
	OSIOS             int64 `json:"os_ios"`
	OSAndroid         int64 `json:"os_android"`
	OSDesktop         int64 `json:"os_desktop"`
	OSApp             int64 `json:"os_app"`
	OSOther           int64 `json:"os_other"`
	Sessions          int64 `json:"sessions,omitempty"`
	EngagedSessions   int64 `json:"engaged_sessions,omitempty"`
	ReturningVisitors int64 `json:"returning_visitors,omitempty"`
	AcqQR             int64 `json:"acq_qr,omitempty"`
	AcqShare          int64 `json:"acq_share,omitempty"`
	AcqApp            int64 `json:"acq_app,omitempty"`
	AcqDirect         int64 `json:"acq_direct,omitempty"`
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
	m.AIPickleballRuns = clampNonNeg64(m.AIPickleballRuns)
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
	m.Sessions = clampNonNeg64(m.Sessions)
	m.EngagedSessions = clampNonNeg64(m.EngagedSessions)
	m.ReturningVisitors = clampNonNeg64(m.ReturningVisitors)
	m.AcqQR = clampNonNeg64(m.AcqQR)
	m.AcqShare = clampNonNeg64(m.AcqShare)
	m.AcqApp = clampNonNeg64(m.AcqApp)
	m.AcqDirect = clampNonNeg64(m.AcqDirect)
}

func (m AnalyticsBoxMetrics) add(o AnalyticsBoxMetrics) AnalyticsBoxMetrics {
	return AnalyticsBoxMetrics{
		QRViews:           m.QRViews + o.QRViews,
		PageViews:         m.PageViews + o.PageViews,
		UniqueVisitors:    m.UniqueVisitors + o.UniqueVisitors,
		LiveHLS:           m.LiveHLS + o.LiveHLS,
		VODPlays:          m.VODPlays + o.VODPlays,
		ClipDownloads:     m.ClipDownloads + o.ClipDownloads,
		TimelineOpens:     m.TimelineOpens + o.TimelineOpens,
		CutClips:          m.CutClips + o.CutClips,
		CutSeconds:        m.CutSeconds + o.CutSeconds,
		AIStudioOpens:     m.AIStudioOpens + o.AIStudioOpens,
		AIVideosLoads:     m.AIVideosLoads + o.AIVideosLoads,
		AIAnalyzes:        m.AIAnalyzes + o.AIAnalyzes,
		AIShots:           m.AIShots + o.AIShots,
		AIPots:            m.AIPots + o.AIPots,
		AIPoolRuns:        m.AIPoolRuns + o.AIPoolRuns,
		AICaromRuns:       m.AICaromRuns + o.AICaromRuns,
		AIPickleballRuns:  m.AIPickleballRuns + o.AIPickleballRuns,
		AIExportsIOS:      m.AIExportsIOS + o.AIExportsIOS,
		AIExportsDesktop:  m.AIExportsDesktop + o.AIExportsDesktop,
		AIExportRelay:     m.AIExportRelay + o.AIExportRelay,
		AIErrors:          m.AIErrors + o.AIErrors,
		HTTPErrors:        m.HTTPErrors + o.HTTPErrors,
		BytesOut:          m.BytesOut + o.BytesOut,
		OSIOS:             m.OSIOS + o.OSIOS,
		OSAndroid:         m.OSAndroid + o.OSAndroid,
		OSDesktop:         m.OSDesktop + o.OSDesktop,
		OSApp:             m.OSApp + o.OSApp,
		OSOther:           m.OSOther + o.OSOther,
		Sessions:          m.Sessions + o.Sessions,
		EngagedSessions:   m.EngagedSessions + o.EngagedSessions,
		ReturningVisitors: m.ReturningVisitors + o.ReturningVisitors,
		AcqQR:             m.AcqQR + o.AcqQR,
		AcqShare:          m.AcqShare + o.AcqShare,
		AcqApp:            m.AcqApp + o.AcqApp,
		AcqDirect:         m.AcqDirect + o.AcqDirect,
	}
}

func (m AnalyticsBoxMetrics) maxWith(o AnalyticsBoxMetrics) AnalyticsBoxMetrics {
	return AnalyticsBoxMetrics{
		QRViews:           max64(m.QRViews, o.QRViews),
		PageViews:         max64(m.PageViews, o.PageViews),
		UniqueVisitors:    max64(m.UniqueVisitors, o.UniqueVisitors),
		LiveHLS:           max64(m.LiveHLS, o.LiveHLS),
		VODPlays:          max64(m.VODPlays, o.VODPlays),
		ClipDownloads:     max64(m.ClipDownloads, o.ClipDownloads),
		TimelineOpens:     max64(m.TimelineOpens, o.TimelineOpens),
		CutClips:          max64(m.CutClips, o.CutClips),
		CutSeconds:        max64(m.CutSeconds, o.CutSeconds),
		AIStudioOpens:     max64(m.AIStudioOpens, o.AIStudioOpens),
		AIVideosLoads:     max64(m.AIVideosLoads, o.AIVideosLoads),
		AIAnalyzes:        max64(m.AIAnalyzes, o.AIAnalyzes),
		AIShots:           max64(m.AIShots, o.AIShots),
		AIPots:            max64(m.AIPots, o.AIPots),
		AIPoolRuns:        max64(m.AIPoolRuns, o.AIPoolRuns),
		AICaromRuns:       max64(m.AICaromRuns, o.AICaromRuns),
		AIPickleballRuns:  max64(m.AIPickleballRuns, o.AIPickleballRuns),
		AIExportsIOS:      max64(m.AIExportsIOS, o.AIExportsIOS),
		AIExportsDesktop:  max64(m.AIExportsDesktop, o.AIExportsDesktop),
		AIExportRelay:     max64(m.AIExportRelay, o.AIExportRelay),
		AIErrors:          max64(m.AIErrors, o.AIErrors),
		HTTPErrors:        max64(m.HTTPErrors, o.HTTPErrors),
		BytesOut:          max64(m.BytesOut, o.BytesOut),
		OSIOS:             max64(m.OSIOS, o.OSIOS),
		OSAndroid:         max64(m.OSAndroid, o.OSAndroid),
		OSDesktop:         max64(m.OSDesktop, o.OSDesktop),
		OSApp:             max64(m.OSApp, o.OSApp),
		OSOther:           max64(m.OSOther, o.OSOther),
		Sessions:          max64(m.Sessions, o.Sessions),
		EngagedSessions:   max64(m.EngagedSessions, o.EngagedSessions),
		ReturningVisitors: max64(m.ReturningVisitors, o.ReturningVisitors),
		AcqQR:             max64(m.AcqQR, o.AcqQR),
		AcqShare:          max64(m.AcqShare, o.AcqShare),
		AcqApp:            max64(m.AcqApp, o.AcqApp),
		AcqDirect:         max64(m.AcqDirect, o.AcqDirect),
	}
}

func (m AnalyticsBoxMetrics) aiExportsTotal() int64 {
	return max64(m.AIExportsIOS, m.AIExportRelay) + m.AIExportsDesktop
}

func (m AnalyticsBoxMetrics) hasAIActivity() bool {
	return m.AIStudioOpens > 0 || m.AIVideosLoads > 0 || m.AIAnalyzes > 0 || m.AIShots > 0 ||
		m.AIPoolRuns > 0 || m.AICaromRuns > 0 || m.AIPickleballRuns > 0 || m.aiExportsTotal() > 0
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

// AnalyticsBoxDailyIn carries exact box-level daily visitor/session/cohort counts.
type AnalyticsBoxDailyIn struct {
	BucketDate        string `json:"bucket_date"` // "YYYY-MM-DD" GMT+7
	UniqueVisitors    int64  `json:"unique_visitors"`
	Sessions          int64  `json:"sessions,omitempty"`
	EngagedSessions   int64  `json:"engaged_sessions,omitempty"`
	ReturningVisitors int64  `json:"returning_visitors,omitempty"`
}

// AnalyticsCamDailyIn represents one per-camera daily bucket in an ingest payload.
type AnalyticsCamDailyIn struct {
	BucketDate string `json:"bucket_date"` // "YYYY-MM-DD" GMT+7
	CameraID   string `json:"camera_id"`
	TableName  string `json:"table_name,omitempty"`
	AnalyticsCamMetrics
}

// AnalyticsIngestPayload is accepted by POST /fleet/api/analytics/ingest from both
// redbida-go (cumulative snapshot / dirty-bucket delta with source_epoch & rev) and legacy Node-RED (delta with batch_id).
type AnalyticsIngestPayload struct {
	BoxID       string                 `json:"box_id"`
	Secret      string                 `json:"secret,omitempty"`
	Source      string                 `json:"source"`                 // "redbida-go" | "nodered"
	SourceEpoch string                 `json:"source_epoch,omitempty"` // persistent epoch ID for Mode A
	BatchID     string                 `json:"batch_id,omitempty"`     // idempotency key for Mode B
	Rev         uint64                 `json:"rev,omitempty"`          // monotonic edge collector revision
	IsDelta     bool                   `json:"is_delta,omitempty"`     // true for dirty-bucket delta
	GeneratedAt string                 `json:"generated_at,omitempty"`
	AIEnabled   bool                   `json:"ai_enabled"`
	Realtime    *RealtimeBoxMetrics    `json:"realtime,omitempty"`
	BoxHourly   []AnalyticsBoxHourlyIn `json:"box_hourly"`
	BoxDaily    []AnalyticsBoxDailyIn  `json:"box_daily,omitempty"`
	CameraDaily []AnalyticsCamDailyIn  `json:"camera_daily,omitempty"`
}

// AnalyticsIngestResult summarizes a completed or queued analytics ingest request.
type AnalyticsIngestResult struct {
	OK             bool `json:"ok"`
	Deduped        bool `json:"deduped,omitempty"`
	Queued         bool `json:"queued,omitempty"`
	NeedFullSync   bool `json:"need_full_sync,omitempty"`
	HourlyUpserted int  `json:"hourly_upserted,omitempty"`
	DailyUpdated   int  `json:"daily_updated,omitempty"`
	CamerasUpdated int  `json:"cameras_updated,omitempty"`
}

// AnalyticsKPI holds the fleet/club overview KPI totals (GA4-enriched).
type AnalyticsKPI struct {
	QRViews           int64   `json:"qr_views"`
	PageViews         int64   `json:"page_views"`
	UniqueVisitors    int64   `json:"unique_visitors"`
	Sessions          int64   `json:"sessions"`
	EngagedSessions   int64   `json:"engaged_sessions"`
	ReturningVisitors int64   `json:"returning_visitors"`
	EngagementRatePct float64 `json:"engagement_rate_pct"`
	ErrorRatePct      float64 `json:"error_rate_pct"`
	LiveHLS           int64   `json:"live_hls"`
	VODPlays          int64   `json:"vod_plays"`
	ClipDownloads     int64   `json:"clip_downloads"`
	TimelineOpens     int64   `json:"timeline_opens"`
	CutClips          int64   `json:"cut_clips"`
	ClipsTotal        int64   `json:"clips_total"`
	CutSeconds        int64   `json:"cut_seconds"`
	AIStudioOpens     int64   `json:"ai_studio_opens"`
	AIVideosLoads     int64   `json:"ai_videos_loads"`
	AIAnalyzes        int64   `json:"ai_analyzes"`
	AIShots           int64   `json:"ai_shots"`
	AIPots            int64   `json:"ai_pots"`
	AIPoolRuns        int64   `json:"ai_pool_runs"`
	AICaromRuns       int64   `json:"ai_carom_runs"`
	AIPickleballRuns  int64   `json:"ai_pickleball_runs"`
	AIExportsIOS      int64   `json:"ai_exports_ios"`
	AIExportsDesktop  int64   `json:"ai_exports_desktop"`
	AIExportRelay     int64   `json:"ai_export_relay"`
	AIExportsTotal    int64   `json:"ai_exports_total"`
	AIErrors          int64   `json:"ai_errors"`
	HTTPErrors        int64   `json:"http_errors"`
	BytesOut          int64   `json:"bytes_out"`
	ActiveBoxes       int     `json:"active_boxes"`
	ActiveClubs       int     `json:"active_clubs"`
	AIActiveBoxes     int     `json:"ai_active_boxes"`
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
	Sessions       int64  `json:"sessions"`
	DownloadsCuts  int64  `json:"downloads_cuts"`
	AIAnalyzes     int64  `json:"ai_analyzes"`
	AIExports      int64  `json:"ai_exports"`
	BytesOut       int64  `json:"bytes_out"`
}

// AnalyticsComparisonDeltas holds % deltas vs previous equal-length period.
type AnalyticsComparisonDeltas struct {
	UniqueVisitors float64 `json:"unique_visitors"`
	Sessions       float64 `json:"sessions"`
	QRViews        float64 `json:"qr_views"`
	ClipsTotal     float64 `json:"clips_total"`
	AIAnalyzes     float64 `json:"ai_analyzes"`
	BytesOut       float64 `json:"bytes_out"`
	EngagementRate float64 `json:"engagement_rate"`
}

// AnalyticsComparison carries previous-period KPIs, % deltas, and aligned daily series.
type AnalyticsComparison struct {
	From        string                    `json:"from"`
	To          string                    `json:"to"`
	KPIs        AnalyticsKPI              `json:"kpis"`
	DeltasPct   AnalyticsComparisonDeltas `json:"deltas_pct"`
	DailySeries []AnalyticsDailyPoint     `json:"daily_series"`
}

// AnalyticsFunnelStep represents one step of the 4-step visitor conversion funnel.
type AnalyticsFunnelStep struct {
	Step     string  `json:"step"`
	Label    string  `json:"label"`
	Count    int64   `json:"count"`
	PctOfTop float64 `json:"pct_of_top"`
	DropPct  float64 `json:"drop_pct"`
}

// AnalyticsHeatmapPeak identifies the single hottest weekday x hour slot (GMT+7).
type AnalyticsHeatmapPeak struct {
	DayIndex int    `json:"day_index"` // 0=Mon .. 6=Sun
	DayLabel string `json:"day_label"` // "T2".."CN"
	Hour     int    `json:"hour"`      // 0..23
	Value    int64  `json:"value"`
}

// AnalyticsHeatmap7x24 holds the 7x24 Mon..Sun x 0..23h GMT+7 traffic intensity matrix.
type AnalyticsHeatmap7x24 struct {
	Days        []string             `json:"days"`
	Matrix      [][]int64            `json:"matrix"`
	MaxCell     int64                `json:"max_cell"`
	PeakDayHour AnalyticsHeatmapPeak `json:"peak_day_hour"`
}

// AnalyticsAcquisition holds GA4 acquisition channel counts.
type AnalyticsAcquisition struct {
	QR     int64 `json:"qr"`
	Share  int64 `json:"share"`
	App    int64 `json:"app"`
	Direct int64 `json:"direct"`
}

// AnalyticsCohorts holds GA4 new/returning visitor and session engagement metrics.
type AnalyticsCohorts struct {
	NewVisitors       int64   `json:"new_visitors"`
	ReturningVisitors int64   `json:"returning_visitors"`
	ReturningPct      float64 `json:"returning_pct"`
	Sessions          int64   `json:"sessions"`
	EngagedSessions   int64   `json:"engaged_sessions"`
	EngagementRatePct float64 `json:"engagement_rate_pct"`
	ViewsPerSession   float64 `json:"views_per_session"`
}

// AnalyticsTopClub holds aggregated analytics for one club.
type AnalyticsTopClub struct {
	ClubID            string   `json:"club_id"`
	ClubName          string   `json:"club_name"`
	Branding          string   `json:"branding,omitempty"`
	BoxesCount        int      `json:"boxes_count"`
	Sources           []string `json:"sources"`
	AIEnabled         bool     `json:"ai_enabled"`
	QRViews           int64    `json:"qr_views"`
	UniqueVisitors    int64    `json:"unique_visitors"`
	Sessions          int64    `json:"sessions"`
	EngagedSessions   int64    `json:"engaged_sessions"`
	ReturningVisitors int64    `json:"returning_visitors"`
	LiveHLS           int64    `json:"live_hls"`
	VODPlays          int64    `json:"vod_plays"`
	ClipDownloads     int64    `json:"clip_downloads"`
	CutClips          int64    `json:"cut_clips"`
	AIStudioOpens     int64    `json:"ai_studio_opens"`
	AIAnalyzes        int64    `json:"ai_analyzes"`
	AIShots           int64    `json:"ai_shots"`
	AIPickleballRuns  int64    `json:"ai_pickleball_runs"`
	AIExports         int64    `json:"ai_exports"`
	AIErrors          int64    `json:"ai_errors"`
	HTTPErrors        int64    `json:"http_errors"`
	BytesOut          int64    `json:"bytes_out"`
	LastIngestAt      string   `json:"last_ingest_at"`
}

// AnalyticsTopBox holds aggregated analytics for one box.
type AnalyticsTopBox struct {
	BoxID             string `json:"box_id"`
	BoxName           string `json:"box_name"`
	ClubID            string `json:"club_id"`
	ClubName          string `json:"club_name"`
	Branding          string `json:"branding,omitempty"`
	Source            string `json:"source"`
	AIEnabled         bool   `json:"ai_enabled"`
	QRViews           int64  `json:"qr_views"`
	UniqueVisitors    int64  `json:"unique_visitors"`
	Sessions          int64  `json:"sessions"`
	EngagedSessions   int64  `json:"engaged_sessions"`
	ReturningVisitors int64  `json:"returning_visitors"`
	LiveHLS           int64  `json:"live_hls"`
	VODPlays          int64  `json:"vod_plays"`
	ClipDownloads     int64  `json:"clip_downloads"`
	CutClips          int64  `json:"cut_clips"`
	AIStudioOpens     int64  `json:"ai_studio_opens"`
	AIAnalyzes        int64  `json:"ai_analyzes"`
	AIShots           int64  `json:"ai_shots"`
	AIPickleballRuns  int64  `json:"ai_pickleball_runs"`
	AIExports         int64  `json:"ai_exports"`
	AIErrors          int64  `json:"ai_errors"`
	HTTPErrors        int64  `json:"http_errors"`
	BytesOut          int64  `json:"bytes_out"`
	LastIngestAt      string `json:"last_ingest_at"`
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
	Comparison  *AnalyticsComparison  `json:"comparison,omitempty"`
	Funnel      []AnalyticsFunnelStep `json:"funnel"`
	Heatmap7x24 AnalyticsHeatmap7x24  `json:"heatmap_7x24"`
	Acquisition AnalyticsAcquisition  `json:"acquisition"`
	Cohorts     AnalyticsCohorts      `json:"cohorts"`
	TopClubs    []AnalyticsTopClub    `json:"top_clubs"`
	TopBoxes    []AnalyticsTopBox     `json:"top_boxes"`
}

// AnalyticsRealtimeHotClub is one actively viewed club in GA4 Realtime.
type AnalyticsRealtimeHotClub struct {
	ClubID   string `json:"club_id"`
	ClubName string `json:"club_name"`
	Branding string `json:"branding,omitempty"`
	A5       int    `json:"a5"`
	A30      int    `json:"a30"`
	Live     int    `json:"live"`
	VOD      int    `json:"vod"`
	Cut      int    `json:"cut"`
	AI       int    `json:"ai"`
}

// AnalyticsRealtimeHotTable is one actively viewed camera/table in GA4 Realtime.
type AnalyticsRealtimeHotTable struct {
	BoxID     string `json:"box_id"`
	BoxName   string `json:"box_name"`
	ClubID    string `json:"club_id"`
	ClubName  string `json:"club_name"`
	CameraID  string `json:"camera_id"`
	TableName string `json:"table_name"`
	Active    int    `json:"active"`
}

// AnalyticsRealtimeResponse is the JSON contract for GET /fleet/api/analytics/realtime
// and GET /portal/{token}/analytics/realtime.
type AnalyticsRealtimeResponse struct {
	OK           bool                        `json:"ok"`
	ClubID       string                      `json:"club_id,omitempty"`
	ClubName     string                      `json:"club_name,omitempty"`
	Active5m     int                         `json:"active_5m"`
	Active30m    int                         `json:"active_30m"`
	Live         int                         `json:"live"`
	VOD          int                         `json:"vod"`
	Cut          int                         `json:"cut"`
	AI           int                         `json:"ai"`
	MinuteBars30 []int                       `json:"minute_bars_30"`
	ActiveBoxes  int                         `json:"active_boxes"`
	ActiveClubs  int                         `json:"active_clubs"`
	HotClubs     []AnalyticsRealtimeHotClub  `json:"hot_clubs"`
	HotTables    []AnalyticsRealtimeHotTable `json:"hot_tables"`
	UpdatedAt    string                      `json:"updated_at"`
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

// AnalyticsCoalescerStats exposes write coalescer metrics for observability and testing.
type AnalyticsCoalescerStats struct {
	TxCount          int64
	BatchesCommitted int64
}

type analyticsQueryParams struct {
	Range   string
	From    string
	To      string
	ClubID  string
	BoxID   string
	Sport   string // "all" | "billiards" | "pickleball"
	Source  string // "all" | "redbida-go" | "nodered"
	AIOnly  bool
	Compare bool
}

type analyticsCameraQueryParams struct {
	Range  string
	From   string
	To     string
	ClubID string
	BoxID  string
	Sport  string
	Limit  int
}

type analyticsRealtimeFilter struct {
	ClubID string
	BoxID  string
	Sport  string
}

type realtimeBoxEntry struct {
	Metrics   RealtimeBoxMetrics
	UpdatedAt time.Time
}

type analyticsCoalesceItem struct {
	payload AnalyticsIngestPayload
	now     time.Time
}

// analyticsCoalescer batches bursts of incoming analytics ingest payloads within a short
// flush window (default 2s or 32 batches) into a single SQLite WAL transaction.
type analyticsCoalescer struct {
	srv      *Server
	window   time.Duration
	maxBatch int
	inCh     chan analyticsCoalesceItem
	flushCh  chan chan error
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once

	statsMu          sync.Mutex
	txCount          int64
	batchesCommitted int64
}

func newAnalyticsCoalescer(srv *Server, window time.Duration, maxBatch int) *analyticsCoalescer {
	if window <= 0 {
		window = defaultAnalyticsCoalesceWindow
	}
	if maxBatch <= 0 {
		maxBatch = defaultAnalyticsCoalesceMaxBatch
	}
	c := &analyticsCoalescer{
		srv:      srv,
		window:   window,
		maxBatch: maxBatch,
		inCh:     make(chan analyticsCoalesceItem, analyticsCoalesceQueueCap),
		flushCh:  make(chan chan error),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	go c.run()
	return c
}

func (c *analyticsCoalescer) Enqueue(payload AnalyticsIngestPayload, now time.Time) bool {
	select {
	case <-c.stopCh:
		return false
	default:
	}
	select {
	case c.inCh <- analyticsCoalesceItem{payload: payload, now: now}:
		return true
	default:
		return false
	}
}

func (c *analyticsCoalescer) Flush() error {
	ack := make(chan error, 1)
	select {
	case <-c.doneCh:
		return nil
	case c.flushCh <- ack:
	}
	select {
	case err := <-ack:
		return err
	case <-c.doneCh:
		return nil
	}
}

func (c *analyticsCoalescer) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.doneCh
}

func (c *analyticsCoalescer) Stats() AnalyticsCoalescerStats {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return AnalyticsCoalescerStats{
		TxCount:          c.txCount,
		BatchesCommitted: c.batchesCommitted,
	}
}

func (c *analyticsCoalescer) run() {
	defer close(c.doneCh)
	pending := make([]analyticsCoalesceItem, 0, c.maxBatch)
	var timer *time.Timer
	var timerC <-chan time.Time

	stopTimer := func() {
		if timer != nil {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer = nil
			timerC = nil
		}
	}

	commitPending := func() error {
		stopTimer()
		if len(pending) == 0 {
			return nil
		}
		batch := pending
		pending = make([]analyticsCoalesceItem, 0, c.maxBatch)
		err := c.srv.IngestAnalyticsBatch(batch)
		if err != nil {
			log.Printf("[bidahub-analytics] coalescer batch commit failed (%d items): %v", len(batch), err)
			return err
		}
		c.statsMu.Lock()
		c.txCount++
		c.batchesCommitted += int64(len(batch))
		c.statsMu.Unlock()
		return nil
	}

	drainBuffered := func() {
		for {
			select {
			case item := <-c.inCh:
				pending = append(pending, item)
			default:
				return
			}
		}
	}

	for {
		select {
		case item := <-c.inCh:
			pending = append(pending, item)
			if len(pending) >= c.maxBatch {
				_ = commitPending()
			} else if len(pending) == 1 {
				timer = time.NewTimer(c.window)
				timerC = timer.C
			}
		case <-timerC:
			drainBuffered()
			_ = commitPending()
		case ack := <-c.flushCh:
			drainBuffered()
			ack <- commitPending()
		case <-c.stopCh:
			drainBuffered()
			_ = commitPending()
			return
		}
	}
}

// FlushAnalytics forces any buffered analytics ingest batches in the write coalescer
// to commit immediately to SQLite WAL. Safe to call concurrently and in tests.
func (s *Server) FlushAnalytics() error {
	if s.coalescer == nil {
		return nil
	}
	return s.coalescer.Flush()
}

// AnalyticsCoalescerStats returns write coalescer transaction and batch counters.
func (s *Server) AnalyticsCoalescerStats() AnalyticsCoalescerStats {
	if s.coalescer == nil {
		return AnalyticsCoalescerStats{}
	}
	return s.coalescer.Stats()
}

func (s *Server) stopAnalyticsCoalescer() {
	if s.coalescer != nil {
		s.coalescer.Stop()
	}
}

func clampNonNeg64(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

func clampNonNegInt(v int) int {
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

func min64(a, b int64) int64 {
	if a <= b {
		return a
	}
	return b
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func normalizeAnalyticsRange(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "today":
		return "today"
	case "yesterday":
		return "yesterday"
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
	case "today":
		return "today"
	case "yesterday":
		return "yesterday"
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

func normalizeAnalyticsSport(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "billiards", "bida":
		return "billiards"
	case "pickleball":
		return "pickleball"
	default:
		return "all"
	}
}

func normalizeAnalyticsSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "redbida-go", "go":
		return "redbida-go"
	case "nodered", "node-red":
		return "nodered"
	default:
		return "all"
	}
}

func parseAnalyticsCompareParam(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "prev", "previous", "yes":
		return true
	default:
		return false
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

// =============================================================================
// IN-RAM REALTIME VIEWER STORE (0 SQLite writes, 90s auto-expiry)
// =============================================================================

func sanitizeRealtimeBoxMetrics(rt RealtimeBoxMetrics) RealtimeBoxMetrics {
	out := RealtimeBoxMetrics{
		A5:   clampNonNegInt(rt.A5),
		A30:  clampNonNegInt(rt.A30),
		Live: clampNonNegInt(rt.Live),
		VOD:  clampNonNegInt(rt.VOD),
		Cut:  clampNonNegInt(rt.Cut),
		AI:   clampNonNegInt(rt.AI),
	}
	if out.A30 < out.A5 {
		out.A30 = out.A5
	}
	if len(rt.M30) > 0 {
		src := rt.M30
		if len(src) > 30 {
			src = src[len(src)-30:]
		}
		out.M30 = make([]int, len(src))
		for i, v := range src {
			out.M30[i] = clampNonNegInt(v)
		}
	}
	if len(rt.Hot) > 0 {
		hot := make([]RealtimeHotTable, 0, len(rt.Hot))
		for _, h := range rt.Hot {
			cam := strings.TrimSpace(h.Cam)
			if cam == "" || h.N <= 0 {
				continue
			}
			hot = append(hot, RealtimeHotTable{
				Cam:   cam,
				Table: strings.TrimSpace(h.Table),
				N:     h.N,
			})
			if len(hot) >= 5 {
				break
			}
		}
		if len(hot) > 0 {
			out.Hot = hot
		}
	}
	return out
}

func (s *Server) recordBoxRealtime(boxID string, rt *RealtimeBoxMetrics, now time.Time) {
	boxID = strings.TrimSpace(boxID)
	if boxID == "" {
		return
	}
	s.rtMu.Lock()
	defer s.rtMu.Unlock()
	if s.rtByBox == nil {
		s.rtByBox = make(map[string]realtimeBoxEntry)
	}
	if rt == nil {
		delete(s.rtByBox, boxID)
	} else {
		clean := sanitizeRealtimeBoxMetrics(*rt)
		if clean.A30 <= 0 {
			delete(s.rtByBox, boxID)
		} else {
			s.rtByBox[boxID] = realtimeBoxEntry{
				Metrics:   clean,
				UpdatedAt: now,
			}
		}
	}
	// Opportunistic expiry of stale boxes (>90s)
	for id, ent := range s.rtByBox {
		if now.Sub(ent.UpdatedAt) > realtimeEntryTTL {
			delete(s.rtByBox, id)
		}
	}
}

func (s *Server) realtimeSummaryIfChanged(now time.Time) (map[string]int, bool) {
	s.rtMu.Lock()
	defer s.rtMu.Unlock()
	a5, a30 := 0, 0
	for id, ent := range s.rtByBox {
		if now.Sub(ent.UpdatedAt) > realtimeEntryTTL {
			delete(s.rtByBox, id)
			continue
		}
		a5 += ent.Metrics.A5
		a30 += ent.Metrics.A30
	}
	if a5 == s.lastRTSumA5 && a30 == s.lastRTSumA30 {
		return nil, false
	}
	s.lastRTSumA5 = a5
	s.lastRTSumA30 = a30
	return map[string]int{
		"active_5m":  a5,
		"active_30m": a30,
	}, true
}

// QueryAnalyticsRealtime aggregates active viewers across non-expired edge boxes in RAM (0 SQLite queries).
func (s *Server) QueryAnalyticsRealtime(filter analyticsRealtimeFilter, now time.Time) AnalyticsRealtimeResponse {
	meta := s.snapshotAnalyticsMetadata()

	// Snapshot active non-expired box entries under lock.
	s.rtMu.Lock()
	active := make(map[string]RealtimeBoxMetrics, len(s.rtByBox))
	for id, ent := range s.rtByBox {
		if now.Sub(ent.UpdatedAt) > realtimeEntryTTL {
			delete(s.rtByBox, id)
			continue
		}
		if ent.Metrics.A30 > 0 {
			active[id] = ent.Metrics
		}
	}
	s.rtMu.Unlock()

	filterClub := strings.TrimSpace(filter.ClubID)
	filterBox := strings.TrimSpace(filter.BoxID)
	filterSport := normalizeAnalyticsSport(filter.Sport)

	resp := AnalyticsRealtimeResponse{
		OK:           true,
		MinuteBars30: make([]int, 30),
		HotClubs:     make([]AnalyticsRealtimeHotClub, 0),
		HotTables:    make([]AnalyticsRealtimeHotTable, 0),
		UpdatedAt:    now.UTC().Format(time.RFC3339),
	}

	clubAggs := make(map[string]*AnalyticsRealtimeHotClub)
	activeClubsSet := make(map[string]struct{})

	for boxID, rt := range active {
		if filterBox != "" && boxID != filterBox {
			continue
		}
		boxName, clubID, clubName := meta.resolveNames(boxID, "")
		if filterClub != "" && clubID != filterClub {
			continue
		}
		branding := meta.resolveBranding(boxID, clubID)
		if filterSport != "all" && branding != filterSport {
			continue
		}

		resp.Active5m += rt.A5
		resp.Active30m += rt.A30
		resp.Live += rt.Live
		resp.VOD += rt.VOD
		resp.Cut += rt.Cut
		resp.AI += rt.AI
		resp.ActiveBoxes++

		// Right-align M30 bars into the 30-element minute_bars_30 slice [oldest(-29m)...now].
		if len(rt.M30) > 0 {
			src := rt.M30
			if len(src) > 30 {
				src = src[len(src)-30:]
			}
			offset := 30 - len(src)
			for i, v := range src {
				if v > 0 {
					resp.MinuteBars30[offset+i] += v
				}
			}
		}

		clubKey := clubID
		if clubKey == "" {
			clubKey = boxID
		}
		activeClubsSet[clubKey] = struct{}{}

		ca, ok := clubAggs[clubKey]
		if !ok {
			ca = &AnalyticsRealtimeHotClub{
				ClubID:   clubID,
				ClubName: clubName,
				Branding: branding,
			}
			clubAggs[clubKey] = ca
		}
		ca.A5 += rt.A5
		ca.A30 += rt.A30
		ca.Live += rt.Live
		ca.VOD += rt.VOD
		ca.Cut += rt.Cut
		ca.AI += rt.AI

		for _, ht := range rt.Hot {
			if ht.N <= 0 || ht.Cam == "" {
				continue
			}
			tName := meta.resolveTableName(boxID, ht.Cam, ht.Table)
			if tName == "" {
				tName = ht.Cam
			}
			resp.HotTables = append(resp.HotTables, AnalyticsRealtimeHotTable{
				BoxID:     boxID,
				BoxName:   boxName,
				ClubID:    clubID,
				ClubName:  clubName,
				CameraID:  ht.Cam,
				TableName: tName,
				Active:    ht.N,
			})
		}
	}

	resp.ActiveClubs = len(activeClubsSet)

	for _, ca := range clubAggs {
		resp.HotClubs = append(resp.HotClubs, *ca)
	}
	sort.Slice(resp.HotClubs, func(i, j int) bool {
		if resp.HotClubs[i].A5 != resp.HotClubs[j].A5 {
			return resp.HotClubs[i].A5 > resp.HotClubs[j].A5
		}
		if resp.HotClubs[i].A30 != resp.HotClubs[j].A30 {
			return resp.HotClubs[i].A30 > resp.HotClubs[j].A30
		}
		return resp.HotClubs[i].ClubName < resp.HotClubs[j].ClubName
	})
	if len(resp.HotClubs) > 10 {
		resp.HotClubs = resp.HotClubs[:10]
	}

	sort.Slice(resp.HotTables, func(i, j int) bool {
		if resp.HotTables[i].Active != resp.HotTables[j].Active {
			return resp.HotTables[i].Active > resp.HotTables[j].Active
		}
		if resp.HotTables[i].ClubName != resp.HotTables[j].ClubName {
			return resp.HotTables[i].ClubName < resp.HotTables[j].ClubName
		}
		return resp.HotTables[i].CameraID < resp.HotTables[j].CameraID
	})
	if len(resp.HotTables) > 10 {
		resp.HotTables = resp.HotTables[:10]
	}

	return resp
}

// handleAnalyticsRealtime handles GET /fleet/api/analytics/realtime and GET /api/analytics/realtime.
func (s *Server) handleAnalyticsRealtime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	q := r.URL.Query()
	resp := s.QueryAnalyticsRealtime(analyticsRealtimeFilter{
		ClubID: q.Get("club_id"),
		BoxID:  q.Get("box_id"),
		Sport:  q.Get("sport"),
	}, time.Now())
	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
// INGEST ENDPOINT (GZIP + NEED_FULL_SYNC + WRITE COALESCER)
// =============================================================================

func (s *Server) markKnownEpoch(boxID, epoch string) {
	boxID = strings.TrimSpace(boxID)
	epoch = strings.TrimSpace(epoch)
	if boxID == "" || epoch == "" {
		return
	}
	s.analyticsEpochsMu.Lock()
	if s.analyticsKnownEpochs == nil {
		s.analyticsKnownEpochs = make(map[string]string)
	}
	s.analyticsKnownEpochs[boxID] = epoch
	s.analyticsEpochsMu.Unlock()
}

func (s *Server) checkNeedFullSync(boxID string, payload AnalyticsIngestPayload) bool {
	if payload.Source != "redbida-go" {
		return false
	}
	epoch := strings.TrimSpace(payload.SourceEpoch)
	if epoch == "" {
		return false
	}
	if !payload.IsDelta {
		s.markKnownEpoch(boxID, epoch)
		return false
	}

	s.analyticsEpochsMu.RLock()
	known, ok := s.analyticsKnownEpochs[boxID]
	s.analyticsEpochsMu.RUnlock()
	if ok && known == epoch {
		return false
	}

	// Check SQLite (handles Hub restarts without forcing unnecessary full resyncs).
	existsInDB := false
	_ = s.store.WithSQLiteDB(func(db *sql.DB) error {
		var dummy int
		err := db.QueryRow(
			"SELECT 1 FROM usage_box_hourly WHERE box_id = ? AND source_epoch = ? LIMIT 1",
			boxID, epoch,
		).Scan(&dummy)
		if err == nil {
			existsInDB = true
		}
		return nil
	})
	if existsInDB {
		s.markKnownEpoch(boxID, epoch)
		return false
	}
	return true
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

	var bodyReader io.Reader = r.Body
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Content-Encoding")), "gzip") {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_gzip"})
			return
		}
		defer gz.Close()
		bodyReader = gz
	}

	raw, err := io.ReadAll(io.LimitReader(bodyReader, analyticsMaxIngestBytes+1))
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

	if s.store.SQLiteDB() == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "sqlite_unavailable"})
		return
	}

	now := time.Now()
	if payload.Realtime != nil && payload.Realtime.A30 > 0 {
		s.recordBoxRealtime(boxID, payload.Realtime, now)
	}

	needFullSync := s.checkNeedFullSync(boxID, payload)
	syncMode := r.URL.Query().Get("sync") == "1"

	if s.coalescer != nil && !syncMode {
		if !s.coalescer.Enqueue(payload, now) {
			w.Header().Set("Retry-After", "30")
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"ok":    false,
				"error": "ingest_queue_full",
			})
			return
		}
		writeJSON(w, http.StatusOK, AnalyticsIngestResult{
			OK:           true,
			Queued:       true,
			NeedFullSync: needFullSync,
		})
		return
	}

	res, err := s.IngestAnalytics(payload, now)
	if err != nil {
		if errors.Is(err, errNoSQLiteDB) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "sqlite_unavailable"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "ingest_failed", "detail": err.Error()})
		return
	}
	res.NeedFullSync = needFullSync
	writeJSON(w, http.StatusOK, res)
}

type boxIngestMeta struct {
	clubID        string
	camTableNames map[string]string
}

func (s *Server) snapshotBoxIngestMeta(boxID string) boxIngestMeta {
	bm := boxIngestMeta{
		camTableNames: make(map[string]string),
	}
	if box, ok := s.store.GetBox(boxID); ok && box != nil {
		bm.clubID = box.ClubID
		for _, cam := range box.Cameras {
			if cam != nil && cam.Name != "" && cam.TableName != "" {
				bm.camTableNames[strings.ToLower(cam.Name)] = cam.TableName
			}
		}
		for k, v := range box.RedBidaTabs {
			if strings.TrimSpace(v) != "" {
				lk := strings.ToLower(strings.TrimSpace(k))
				if _, exists := bm.camTableNames[lk]; !exists {
					bm.camTableNames[lk] = strings.TrimSpace(v)
				}
			}
		}
	}
	return bm
}

// IngestAnalyticsBatch commits multiple analytics payloads inside ONE SQLite WAL transaction.
func (s *Server) IngestAnalyticsBatch(items []analyticsCoalesceItem) error {
	if len(items) == 0 {
		return nil
	}
	// Snapshot all box metadata BEFORE acquiring SQLite lock (enforces s.mu -> s.dbMu ordering).
	metaByBox := make(map[string]boxIngestMeta, len(items))
	for _, it := range items {
		boxID := strings.TrimSpace(it.payload.BoxID)
		if boxID == "" {
			continue
		}
		if _, exists := metaByBox[boxID]; !exists {
			metaByBox[boxID] = s.snapshotBoxIngestMeta(boxID)
		}
	}

	err := s.store.WithSQLiteTx(func(tx *sql.Tx) error {
		for _, it := range items {
			boxID := strings.TrimSpace(it.payload.BoxID)
			if boxID == "" {
				continue
			}
			bm := metaByBox[boxID]
			nowStr := it.now.UTC().Format(time.RFC3339)
			var res AnalyticsIngestResult
			res.OK = true
			if it.payload.Source == "nodered" {
				if err := ingestNodeRedDeltaTx(tx, boxID, bm.clubID, it.payload, bm.camTableNames, nowStr, &res); err != nil {
					return err
				}
			} else {
				if err := ingestRedBidaSnapshotTx(tx, boxID, bm.clubID, it.payload, bm.camTableNames, nowStr, &res); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.payload.Source == "redbida-go" && !it.payload.IsDelta && strings.TrimSpace(it.payload.SourceEpoch) != "" {
			s.markKnownEpoch(it.payload.BoxID, it.payload.SourceEpoch)
		}
	}
	return nil
}

// IngestAnalytics persists an analytics payload (Mode A redbida-go snapshot/delta or Mode B nodered delta)
// atomically inside a single SQLite transaction with strict idempotency and zero double-counting.
func (s *Server) IngestAnalytics(payload AnalyticsIngestPayload, now time.Time) (AnalyticsIngestResult, error) {
	boxID := strings.TrimSpace(payload.BoxID)
	if boxID == "" {
		return AnalyticsIngestResult{}, fmt.Errorf("missing box_id")
	}

	// Snapshot box metadata BEFORE acquiring SQLite lock (enforces s.mu -> s.dbMu ordering).
	bm := s.snapshotBoxIngestMeta(boxID)

	nowStr := now.UTC().Format(time.RFC3339)
	var result AnalyticsIngestResult
	result.OK = true

	err := s.store.WithSQLiteTx(func(tx *sql.Tx) error {
		if payload.Source == "nodered" {
			return ingestNodeRedDeltaTx(tx, boxID, bm.clubID, payload, bm.camTableNames, nowStr, &result)
		}
		return ingestRedBidaSnapshotTx(tx, boxID, bm.clubID, payload, bm.camTableNames, nowStr, &result)
	})
	if err != nil {
		return AnalyticsIngestResult{}, err
	}
	if payload.Source == "redbida-go" && !payload.IsDelta && strings.TrimSpace(payload.SourceEpoch) != "" {
		s.markKnownEpoch(boxID, payload.SourceEpoch)
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
	exactDailyByDate := make(map[string]AnalyticsBoxDailyIn)
	for _, bd := range payload.BoxDaily {
		dateStr, ok := parseBucketDate(bd.BucketDate)
		if !ok {
			continue
		}
		bd.BucketDate = dateStr
		bd.UniqueVisitors = clampNonNeg64(bd.UniqueVisitors)
		bd.Sessions = clampNonNeg64(bd.Sessions)
		bd.EngagedSessions = clampNonNeg64(bd.EngagedSessions)
		bd.ReturningVisitors = clampNonNeg64(bd.ReturningVisitors)
		if cur, exists := exactDailyByDate[dateStr]; exists {
			cur.UniqueVisitors = max64(cur.UniqueVisitors, bd.UniqueVisitors)
			cur.Sessions = max64(cur.Sessions, bd.Sessions)
			cur.EngagedSessions = max64(cur.EngagedSessions, bd.EngagedSessions)
			cur.ReturningVisitors = max64(cur.ReturningVisitors, bd.ReturningVisitors)
			exactDailyByDate[dateStr] = cur
		} else {
			exactDailyByDate[dateStr] = bd
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
				ai_pool_runs, ai_carom_runs, ai_pickleball_runs,
				ai_exports_ios, ai_exports_desktop, ai_export_relay,
				ai_errors, http_errors, bytes_out,
				os_ios, os_android, os_desktop, os_app, os_other,
				sessions, engaged_sessions, returning_visitors,
				acq_qr, acq_share, acq_app, acq_direct,
				base_json
			FROM usage_box_hourly
			WHERE box_id = ? AND bucket_hour = ?
		`, boxID, hourStr).Scan(
			&existingClub, &existingAI, &existingEpoch,
			&existing.QRViews, &existing.PageViews, &existing.UniqueVisitors, &existing.LiveHLS, &existing.VODPlays, &existing.ClipDownloads,
			&existing.TimelineOpens, &existing.CutClips, &existing.CutSeconds,
			&existing.AIStudioOpens, &existing.AIVideosLoads, &existing.AIAnalyzes, &existing.AIShots, &existing.AIPots,
			&existing.AIPoolRuns, &existing.AICaromRuns, &existing.AIPickleballRuns,
			&existing.AIExportsIOS, &existing.AIExportsDesktop, &existing.AIExportRelay,
			&existing.AIErrors, &existing.HTTPErrors, &existing.BytesOut,
			&existing.OSIOS, &existing.OSAndroid, &existing.OSDesktop, &existing.OSApp, &existing.OSOther,
			&existing.Sessions, &existing.EngagedSessions, &existing.ReturningVisitors,
			&existing.AcqQR, &existing.AcqShare, &existing.AcqApp, &existing.AcqDirect,
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

	// 2. Recompute/upsert affected usage_box_daily rows from usage_box_hourly + exact box_daily counts.
	dates := sortedKeys(affectedDates)
	for _, dateStr := range dates {
		exactDaily, hasExact := exactDailyByDate[dateStr]
		updated, err := rollupBoxDailyTx(tx, boxID, clubID, "redbida-go", boolToInt(payload.AIEnabled), epoch, dateStr, hasExact, exactDaily, nowStr)
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
				ai_pool_runs, ai_carom_runs, ai_pickleball_runs,
				ai_exports_ios, ai_exports_desktop, ai_export_relay,
				ai_errors, http_errors, bytes_out,
				os_ios, os_android, os_desktop, os_app, os_other,
				sessions, engaged_sessions, returning_visitors,
				acq_qr, acq_share, acq_app, acq_direct,
				base_json
			FROM usage_box_hourly
			WHERE box_id = ? AND bucket_hour = ?
		`, boxID, hourStr).Scan(
			&existingClub, &existingSource, &existingAI, &existingEpoch,
			&existing.QRViews, &existing.PageViews, &existing.UniqueVisitors, &existing.LiveHLS, &existing.VODPlays, &existing.ClipDownloads,
			&existing.TimelineOpens, &existing.CutClips, &existing.CutSeconds,
			&existing.AIStudioOpens, &existing.AIVideosLoads, &existing.AIAnalyzes, &existing.AIShots, &existing.AIPots,
			&existing.AIPoolRuns, &existing.AICaromRuns, &existing.AIPickleballRuns,
			&existing.AIExportsIOS, &existing.AIExportsDesktop, &existing.AIExportRelay,
			&existing.AIErrors, &existing.HTTPErrors, &existing.BytesOut,
			&existing.OSIOS, &existing.OSAndroid, &existing.OSDesktop, &existing.OSApp, &existing.OSOther,
			&existing.Sessions, &existing.EngagedSessions, &existing.ReturningVisitors,
			&existing.AcqQR, &existing.AcqShare, &existing.AcqApp, &existing.AcqDirect,
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
		updated, err := rollupBoxDailyTx(tx, boxID, clubID, "nodered", 0, "", dateStr, false, AnalyticsBoxDailyIn{}, nowStr)
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
	hasExactDaily bool,
	exactDaily AnalyticsBoxDailyIn,
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
			COALESCE(SUM(ai_pickleball_runs), 0),
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
			COALESCE(SUM(os_other), 0),
			COALESCE(SUM(sessions), 0),
			COALESCE(SUM(engaged_sessions), 0),
			COALESCE(SUM(returning_visitors), 0),
			COALESCE(SUM(acq_qr), 0),
			COALESCE(SUM(acq_share), 0),
			COALESCE(SUM(acq_app), 0),
			COALESCE(SUM(acq_direct), 0)
		FROM usage_box_hourly
		WHERE box_id = ? AND bucket_hour >= ? AND bucket_hour <= ?
	`, boxID, dateStr+" 00:00", dateStr+" 23:59").Scan(
		&hourlyCount, &hourlyMaxAI,
		&hourlySum.QRViews, &hourlySum.PageViews, &hourlySum.UniqueVisitors, &hourlySum.LiveHLS, &hourlySum.VODPlays, &hourlySum.ClipDownloads,
		&hourlySum.TimelineOpens, &hourlySum.CutClips, &hourlySum.CutSeconds,
		&hourlySum.AIStudioOpens, &hourlySum.AIVideosLoads, &hourlySum.AIAnalyzes, &hourlySum.AIShots, &hourlySum.AIPots,
		&hourlySum.AIPoolRuns, &hourlySum.AICaromRuns, &hourlySum.AIPickleballRuns,
		&hourlySum.AIExportsIOS, &hourlySum.AIExportsDesktop, &hourlySum.AIExportRelay,
		&hourlySum.AIErrors, &hourlySum.HTTPErrors, &hourlySum.BytesOut,
		&hourlySum.OSIOS, &hourlySum.OSAndroid, &hourlySum.OSDesktop, &hourlySum.OSApp, &hourlySum.OSOther,
		&hourlySum.Sessions, &hourlySum.EngagedSessions, &hourlySum.ReturningVisitors,
		&hourlySum.AcqQR, &hourlySum.AcqShare, &hourlySum.AcqApp, &hourlySum.AcqDirect,
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
			ai_pool_runs, ai_carom_runs, ai_pickleball_runs,
			ai_exports_ios, ai_exports_desktop, ai_export_relay,
			ai_errors, http_errors, bytes_out,
			os_ios, os_android, os_desktop, os_app, os_other,
			sessions, engaged_sessions, returning_visitors,
			acq_qr, acq_share, acq_app, acq_direct,
			base_json
		FROM usage_box_daily
		WHERE box_id = ? AND bucket_date = ?
	`, boxID, dateStr).Scan(
		&existingClub, &existingSource, &existingAI, &existingEpoch,
		&existing.QRViews, &existing.PageViews, &existing.UniqueVisitors, &existing.LiveHLS, &existing.VODPlays, &existing.ClipDownloads,
		&existing.TimelineOpens, &existing.CutClips, &existing.CutSeconds,
		&existing.AIStudioOpens, &existing.AIVideosLoads, &existing.AIAnalyzes, &existing.AIShots, &existing.AIPots,
		&existing.AIPoolRuns, &existing.AICaromRuns, &existing.AIPickleballRuns,
		&existing.AIExportsIOS, &existing.AIExportsDesktop, &existing.AIExportRelay,
		&existing.AIErrors, &existing.HTTPErrors, &existing.BytesOut,
		&existing.OSIOS, &existing.OSAndroid, &existing.OSDesktop, &existing.OSApp, &existing.OSOther,
		&existing.Sessions, &existing.EngagedSessions, &existing.ReturningVisitors,
		&existing.AcqQR, &existing.AcqShare, &existing.AcqApp, &existing.AcqDirect,
		&existingBaseJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		existingFound = false
	} else if err != nil {
		return false, fmt.Errorf("select usage_box_daily: %w", err)
	}

	// Avoid inserting empty phantom daily rows for old box_daily entries with zero traffic.
	if hourlyCount == 0 && !existingFound &&
		(!hasExactDaily || (exactDaily.UniqueVisitors == 0 && exactDaily.Sessions == 0 && exactDaily.EngagedSessions == 0 && exactDaily.ReturningVisitors == 0)) {
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

	if hasExactDaily {
		// Exact box-level daily visitor/session/cohort totals with epoch-safe base_json freezing.
		var dailyBase struct {
			UniqueVisitors    int64 `json:"unique_visitors"`
			Sessions          int64 `json:"sessions,omitempty"`
			EngagedSessions   int64 `json:"engaged_sessions,omitempty"`
			ReturningVisitors int64 `json:"returning_visitors,omitempty"`
		}
		if existingFound {
			_ = json.Unmarshal([]byte(existingBaseJSON), &dailyBase)
			if existingEpoch == incomingEpoch {
				merged.UniqueVisitors = max64(existing.UniqueVisitors, dailyBase.UniqueVisitors+exactDaily.UniqueVisitors)
				if exactDaily.Sessions > 0 {
					merged.Sessions = max64(existing.Sessions, dailyBase.Sessions+exactDaily.Sessions)
				}
				if exactDaily.EngagedSessions > 0 {
					merged.EngagedSessions = max64(existing.EngagedSessions, dailyBase.EngagedSessions+exactDaily.EngagedSessions)
				}
				if exactDaily.ReturningVisitors > 0 {
					merged.ReturningVisitors = max64(existing.ReturningVisitors, dailyBase.ReturningVisitors+exactDaily.ReturningVisitors)
				}
			} else {
				dailyBase.UniqueVisitors = existing.UniqueVisitors
				dailyBase.Sessions = existing.Sessions
				dailyBase.EngagedSessions = existing.EngagedSessions
				dailyBase.ReturningVisitors = existing.ReturningVisitors
				if bRaw, mErr := json.Marshal(dailyBase); mErr == nil {
					newBaseJSON = string(bRaw)
				}
				merged.UniqueVisitors = dailyBase.UniqueVisitors + exactDaily.UniqueVisitors
				if exactDaily.Sessions > 0 {
					merged.Sessions = dailyBase.Sessions + exactDaily.Sessions
				}
				if exactDaily.EngagedSessions > 0 {
					merged.EngagedSessions = dailyBase.EngagedSessions + exactDaily.EngagedSessions
				}
				if exactDaily.ReturningVisitors > 0 {
					merged.ReturningVisitors = dailyBase.ReturningVisitors + exactDaily.ReturningVisitors
				}
				effEpoch = incomingEpoch
			}
		} else {
			merged.UniqueVisitors = exactDaily.UniqueVisitors
			if exactDaily.Sessions > 0 {
				merged.Sessions = exactDaily.Sessions
			}
			if exactDaily.EngagedSessions > 0 {
				merged.EngagedSessions = exactDaily.EngagedSessions
			}
			if exactDaily.ReturningVisitors > 0 {
				merged.ReturningVisitors = exactDaily.ReturningVisitors
			}
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
			ai_pool_runs, ai_carom_runs, ai_pickleball_runs,
			ai_exports_ios, ai_exports_desktop, ai_export_relay,
			ai_errors, http_errors, bytes_out,
			os_ios, os_android, os_desktop, os_app, os_other,
			sessions, engaged_sessions, returning_visitors,
			acq_qr, acq_share, acq_app, acq_direct,
			base_json, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?,
			?, ?
		)
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
			ai_pickleball_runs = excluded.ai_pickleball_runs,
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
			sessions = excluded.sessions,
			engaged_sessions = excluded.engaged_sessions,
			returning_visitors = excluded.returning_visitors,
			acq_qr = excluded.acq_qr,
			acq_share = excluded.acq_share,
			acq_app = excluded.acq_app,
			acq_direct = excluded.acq_direct,
			base_json = excluded.base_json,
			updated_at = excluded.updated_at
	`, boxID, clubID, source, aiEnabled, sourceEpoch, bucketHour,
		m.QRViews, m.PageViews, m.UniqueVisitors, m.LiveHLS, m.VODPlays, m.ClipDownloads,
		m.TimelineOpens, m.CutClips, m.CutSeconds,
		m.AIStudioOpens, m.AIVideosLoads, m.AIAnalyzes, m.AIShots, m.AIPots,
		m.AIPoolRuns, m.AICaromRuns, m.AIPickleballRuns,
		m.AIExportsIOS, m.AIExportsDesktop, m.AIExportRelay,
		m.AIErrors, m.HTTPErrors, m.BytesOut,
		m.OSIOS, m.OSAndroid, m.OSDesktop, m.OSApp, m.OSOther,
		m.Sessions, m.EngagedSessions, m.ReturningVisitors,
		m.AcqQR, m.AcqShare, m.AcqApp, m.AcqDirect,
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
			ai_pool_runs, ai_carom_runs, ai_pickleball_runs,
			ai_exports_ios, ai_exports_desktop, ai_export_relay,
			ai_errors, http_errors, bytes_out,
			os_ios, os_android, os_desktop, os_app, os_other,
			sessions, engaged_sessions, returning_visitors,
			acq_qr, acq_share, acq_app, acq_direct,
			base_json, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?,
			?, ?
		)
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
			ai_pickleball_runs = excluded.ai_pickleball_runs,
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
			sessions = excluded.sessions,
			engaged_sessions = excluded.engaged_sessions,
			returning_visitors = excluded.returning_visitors,
			acq_qr = excluded.acq_qr,
			acq_share = excluded.acq_share,
			acq_app = excluded.acq_app,
			acq_direct = excluded.acq_direct,
			base_json = excluded.base_json,
			updated_at = excluded.updated_at
	`, boxID, clubID, source, aiEnabled, sourceEpoch, bucketDate,
		m.QRViews, m.PageViews, m.UniqueVisitors, m.LiveHLS, m.VODPlays, m.ClipDownloads,
		m.TimelineOpens, m.CutClips, m.CutSeconds,
		m.AIStudioOpens, m.AIVideosLoads, m.AIAnalyzes, m.AIShots, m.AIPots,
		m.AIPoolRuns, m.AICaromRuns, m.AIPickleballRuns,
		m.AIExportsIOS, m.AIExportsDesktop, m.AIExportRelay,
		m.AIErrors, m.HTTPErrors, m.BytesOut,
		m.OSIOS, m.OSAndroid, m.OSDesktop, m.OSApp, m.OSOther,
		m.Sessions, m.EngagedSessions, m.ReturningVisitors,
		m.AcqQR, m.AcqShare, m.AcqApp, m.AcqDirect,
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

func (m analyticsMetadataSnapshot) resolveBranding(boxID, clubID string) string {
	if b, ok := m.boxes[boxID]; ok && b != nil && strings.TrimSpace(b.Branding) != "" {
		if strings.EqualFold(strings.TrimSpace(b.Branding), "pickleball") {
			return "pickleball"
		}
		return "billiards"
	}
	if clubID != "" {
		if c, ok := m.clubs[clubID]; ok && c != nil && strings.TrimSpace(c.Branding) != "" {
			if strings.EqualFold(strings.TrimSpace(c.Branding), "pickleball") {
				return "pickleball"
			}
			return "billiards"
		}
	}
	return "billiards"
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

// =============================================================================
// GA4 WINDOWING, COMPARISON, FUNNEL, HEATMAP, ACQUISITION & COHORTS
// =============================================================================

type analyticsRangeWindow struct {
	Range       string
	FromDate    string
	ToDate      string
	FromHour    string
	ToHour      string
	Dates       []string
	UseHourly24 bool
}

func buildDateSlice(fromDay, toDay time.Time) []string {
	dates := make([]string, 0, 16)
	for d := fromDay; !d.After(toDay); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}
	return dates
}

func computeAnalyticsWindow(rng, fromParam, toParam string, now time.Time) analyticsRangeWindow {
	nowLocal := now.In(analyticsTZ)
	today := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, analyticsTZ)

	// Custom from/to override when both are valid YYYY-MM-DD dates with from <= to.
	if fStr, fOK := parseBucketDate(fromParam); fOK {
		if tStr, tOK := parseBucketDate(toParam); tOK {
			fDay, _ := time.ParseInLocation("2006-01-02", fStr, analyticsTZ)
			tDay, _ := time.ParseInLocation("2006-01-02", tStr, analyticsTZ)
			if !fDay.After(tDay) {
				if tDay.Sub(fDay) > time.Duration(analyticsCustomMaxDays-1)*24*time.Hour {
					fDay = tDay.AddDate(0, 0, -(analyticsCustomMaxDays - 1))
					fStr = fDay.Format("2006-01-02")
				}
				return analyticsRangeWindow{
					Range:       "custom",
					FromDate:    fStr,
					ToDate:      tStr,
					FromHour:    fStr + " 00:00",
					ToHour:      tStr + " 23:00",
					Dates:       buildDateSlice(fDay, tDay),
					UseHourly24: false,
				}
			}
		}
	}

	rng = normalizeAnalyticsRange(rng)

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

	if rng == "today" {
		d := today.Format("2006-01-02")
		return analyticsRangeWindow{
			Range:       "today",
			FromDate:    d,
			ToDate:      d,
			FromHour:    d + " 00:00",
			ToHour:      d + " 23:00",
			Dates:       []string{d},
			UseHourly24: false,
		}
	}

	if rng == "yesterday" {
		y := today.AddDate(0, 0, -1).Format("2006-01-02")
		return analyticsRangeWindow{
			Range:       "yesterday",
			FromDate:    y,
			ToDate:      y,
			FromHour:    y + " 00:00",
			ToHour:      y + " 23:00",
			Dates:       []string{y},
			UseHourly24: false,
		}
	}

	days := 7
	switch rng {
	case "30d":
		days = 30
	case "90d":
		days = 90
	}
	fromDay := today.AddDate(0, 0, -(days - 1))
	fromDate := fromDay.Format("2006-01-02")
	toDate := today.Format("2006-01-02")
	return analyticsRangeWindow{
		Range:       rng,
		FromDate:    fromDate,
		ToDate:      toDate,
		FromHour:    fromDate + " 00:00",
		ToHour:      toDate + " 23:00",
		Dates:       buildDateSlice(fromDay, today),
		UseHourly24: false,
	}
}

func computePreviousAnalyticsWindow(win analyticsRangeWindow) analyticsRangeWindow {
	if win.UseHourly24 {
		startHour, err := time.ParseInLocation("2006-01-02 15:00", win.FromHour, analyticsTZ)
		if err != nil {
			return win
		}
		prevEnd := startHour.Add(-1 * time.Hour)
		prevStart := prevEnd.Add(-23 * time.Hour)
		fromDate := prevStart.Format("2006-01-02")
		toDate := prevEnd.Format("2006-01-02")
		dates := []string{fromDate}
		if toDate != fromDate {
			dates = append(dates, toDate)
		}
		return analyticsRangeWindow{
			Range:       win.Range,
			FromDate:    fromDate,
			ToDate:      toDate,
			FromHour:    prevStart.Format("2006-01-02 15:00"),
			ToHour:      prevEnd.Format("2006-01-02 15:00"),
			Dates:       dates,
			UseHourly24: true,
		}
	}

	fromDay, err := time.ParseInLocation("2006-01-02", win.FromDate, analyticsTZ)
	if err != nil {
		return win
	}
	span := len(win.Dates)
	if span <= 0 {
		span = 1
	}
	prevTo := fromDay.AddDate(0, 0, -1)
	prevFrom := prevTo.AddDate(0, 0, -(span - 1))
	pFromStr := prevFrom.Format("2006-01-02")
	pToStr := prevTo.Format("2006-01-02")
	return analyticsRangeWindow{
		Range:       win.Range,
		FromDate:    pFromStr,
		ToDate:      pToStr,
		FromHour:    pFromStr + " 00:00",
		ToHour:      pToStr + " 23:00",
		Dates:       buildDateSlice(prevFrom, prevTo),
		UseHourly24: false,
	}
}

func computeDeltaPct(cur, prev float64) float64 {
	if prev > 0 {
		return round1(((cur - prev) / prev) * 100.0)
	}
	if cur > 0 {
		return 100.0
	}
	return 0.0
}

func computeComparisonDeltas(cur, prev AnalyticsKPI) AnalyticsComparisonDeltas {
	return AnalyticsComparisonDeltas{
		UniqueVisitors: computeDeltaPct(float64(cur.UniqueVisitors), float64(prev.UniqueVisitors)),
		Sessions:       computeDeltaPct(float64(cur.Sessions), float64(prev.Sessions)),
		QRViews:        computeDeltaPct(float64(cur.QRViews), float64(prev.QRViews)),
		ClipsTotal:     computeDeltaPct(float64(cur.ClipsTotal), float64(prev.ClipsTotal)),
		AIAnalyzes:     computeDeltaPct(float64(cur.AIAnalyzes), float64(prev.AIAnalyzes)),
		BytesOut:       computeDeltaPct(float64(cur.BytesOut), float64(prev.BytesOut)),
		EngagementRate: computeDeltaPct(cur.EngagementRatePct, prev.EngagementRatePct),
	}
}

func buildAnalyticsFunnel(kpi AnalyticsKPI) []AnalyticsFunnelStep {
	s1 := max64(kpi.QRViews, kpi.PageViews)
	s2 := kpi.LiveHLS + kpi.VODPlays + kpi.TimelineOpens
	s3 := kpi.ClipDownloads + kpi.CutClips
	s4 := max64(kpi.AIStudioOpens, max64(kpi.AIAnalyzes, kpi.AIExportsTotal))

	raw := []struct {
		step  string
		label string
		count int64
	}{
		{step: "qr_views", label: "Quét QR / Mở trang", count: s1},
		{step: "watch_timeline", label: "Xem Live / Timeline", count: s2},
		{step: "download_cut", label: "Tải Clip / Cắt CheckVAR", count: s3},
		{step: "ai_studio", label: "AI Studio / Xuất Video", count: s4},
	}

	steps := make([]AnalyticsFunnelStep, len(raw))
	for i, r := range raw {
		pctTop := 0.0
		dropPct := 0.0
		if s1 > 0 {
			if i == 0 {
				pctTop = 100.0
			} else {
				pct := float64(r.count) * 100.0 / float64(s1)
				if pct > 100.0 {
					pct = 100.0
				}
				pctTop = round1(pct)
			}
		}
		if i > 0 && raw[i-1].count > 0 {
			if r.count < raw[i-1].count {
				dropPct = round1(float64(raw[i-1].count-r.count) * 100.0 / float64(raw[i-1].count))
			}
		}
		steps[i] = AnalyticsFunnelStep{
			Step:     r.step,
			Label:    r.label,
			Count:    r.count,
			PctOfTop: pctTop,
			DropPct:  dropPct,
		}
	}
	return steps
}

func buildAnalyticsCohorts(kpi AnalyticsKPI) AnalyticsCohorts {
	ret := kpi.ReturningVisitors
	if kpi.UniqueVisitors > 0 && ret > kpi.UniqueVisitors {
		ret = kpi.UniqueVisitors
	}
	newVis := max64(0, kpi.UniqueVisitors-ret)
	retPct := 0.0
	if kpi.UniqueVisitors > 0 {
		retPct = round1(float64(ret) * 100.0 / float64(kpi.UniqueVisitors))
	}

	engaged := kpi.EngagedSessions
	if kpi.Sessions > 0 && engaged > kpi.Sessions {
		engaged = kpi.Sessions
	}
	engPct := 0.0
	viewsPerSess := 0.0
	if kpi.Sessions > 0 {
		engPct = round1(float64(engaged) * 100.0 / float64(kpi.Sessions))
		viewsPerSess = round1(float64(max64(kpi.PageViews, kpi.QRViews)) / float64(kpi.Sessions))
	}

	return AnalyticsCohorts{
		NewVisitors:       newVis,
		ReturningVisitors: ret,
		ReturningPct:      retPct,
		Sessions:          kpi.Sessions,
		EngagedSessions:   engaged,
		EngagementRatePct: engPct,
		ViewsPerSession:   viewsPerSess,
	}
}

func newEmptyHeatmap7x24() AnalyticsHeatmap7x24 {
	matrix := make([][]int64, 7)
	for d := 0; d < 7; d++ {
		matrix[d] = make([]int64, 24)
	}
	return AnalyticsHeatmap7x24{
		Days:   append([]string(nil), vietnameseWeekdays...),
		Matrix: matrix,
		PeakDayHour: AnalyticsHeatmapPeak{
			DayIndex: 0,
			DayLabel: vietnameseWeekdays[0],
			Hour:     0,
			Value:    0,
		},
	}
}

type boxUsageRow struct {
	BoxID     string
	ClubID    string
	Source    string
	AIEnabled bool
	BucketKey string // bucket_hour or bucket_date
	UpdatedAt string
	Metrics   AnalyticsBoxMetrics
}

type analyticsAggResult struct {
	KPI         AnalyticsKPI
	OSBreakdown AnalyticsOSBreakdown
	PeakHours   []AnalyticsPeakHour
	DailySeries []AnalyticsDailyPoint
	Funnel      []AnalyticsFunnelStep
	Heatmap7x24 AnalyticsHeatmap7x24
	Acquisition AnalyticsAcquisition
	Cohorts     AnalyticsCohorts
	TopClubs    []AnalyticsTopClub
	TopBoxes    []AnalyticsTopBox
}

func (s *Server) aggregateAnalyticsWindow(win analyticsRangeWindow, q analyticsQueryParams, meta analyticsMetadataSnapshot, includeBreakdowns bool) (analyticsAggResult, error) {
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
	heatmap := newEmptyHeatmap7x24()

	out := analyticsAggResult{
		PeakHours:   peakHours,
		DailySeries: dailySeries,
		Heatmap7x24: heatmap,
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
			if includeBreakdowns {
				hourlyRows, err = queryBoxHourlyRows(db, win.FromHour, win.ToHour, q.BoxID)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errNoSQLiteDB) {
			out.Funnel = buildAnalyticsFunnel(out.KPI)
			out.Cohorts = buildAnalyticsCohorts(out.KPI)
			return out, nil
		}
		return out, err
	}

	filterClub := strings.TrimSpace(q.ClubID)
	filterBox := strings.TrimSpace(q.BoxID)
	filterSport := normalizeAnalyticsSport(q.Sport)
	filterSource := normalizeAnalyticsSource(q.Source)

	matchesFilters := func(row boxUsageRow) (boxName, clubID, clubName, branding string, ok bool) {
		if filterBox != "" && row.BoxID != filterBox {
			return "", "", "", "", false
		}
		boxName, clubID, clubName = meta.resolveNames(row.BoxID, row.ClubID)
		if filterClub != "" && clubID != filterClub {
			return "", "", "", "", false
		}
		branding = meta.resolveBranding(row.BoxID, clubID)
		if filterSport != "all" {
			if filterSport == "pickleball" {
				if branding != "pickleball" && row.Metrics.AIPickleballRuns == 0 {
					return "", "", "", "", false
				}
			} else if branding != filterSport {
				return "", "", "", "", false
			}
		}
		if filterSource != "all" && !strings.EqualFold(strings.TrimSpace(row.Source), filterSource) {
			return "", "", "", "", false
		}
		if q.AIOnly && !row.AIEnabled && !row.Metrics.hasAIActivity() {
			return "", "", "", "", false
		}
		return boxName, clubID, clubName, branding, true
	}

	// Aggregate Peak Hours & 7x24 Heatmap (from hourlyRows, GMT+7).
	if includeBreakdowns {
		for _, row := range hourlyRows {
			if _, _, _, _, ok := matchesFilters(row); !ok {
				continue
			}
			normHour, _, hr, valid := parseBucketHour(row.BucketKey)
			if !valid || hr < 0 || hr > 23 {
				continue
			}
			ph := &out.PeakHours[hr]
			ph.QRViews += row.Metrics.QRViews
			ph.DownloadsCuts += row.Metrics.ClipDownloads + row.Metrics.CutClips
			ph.AIRuns += max64(row.Metrics.AIAnalyzes, row.Metrics.AIPoolRuns+row.Metrics.AICaromRuns+row.Metrics.AIPickleballRuns)
			ph.UniqueVisitors += row.Metrics.UniqueVisitors
			ph.BytesOut += row.Metrics.BytesOut

			if t, err := time.ParseInLocation("2006-01-02 15:00", normHour, analyticsTZ); err == nil {
				dayIdx := (int(t.Weekday()) + 6) % 7 // Mon=0 .. Sun=6
				cellVal := max64(row.Metrics.QRViews, row.Metrics.PageViews) +
					row.Metrics.ClipDownloads + row.Metrics.CutClips + row.Metrics.AIAnalyzes
				out.Heatmap7x24.Matrix[dayIdx][hr] += cellVal
			}
		}
		for d := 0; d < 7; d++ {
			for h := 0; h < 24; h++ {
				v := out.Heatmap7x24.Matrix[d][h]
				if v > out.Heatmap7x24.MaxCell {
					out.Heatmap7x24.MaxCell = v
					out.Heatmap7x24.PeakDayHour = AnalyticsHeatmapPeak{
						DayIndex: d,
						DayLabel: vietnameseWeekdays[d],
						Hour:     h,
						Value:    v,
					}
				}
			}
		}
	}

	// Aggregate KPI, OS breakdown, Acquisition, Daily Series, Top Clubs, Top Boxes from mainRows.
	activeBoxes := make(map[string]struct{})
	activeClubs := make(map[string]struct{})
	aiActiveBoxes := make(map[string]struct{})

	type boxAgg struct {
		boxID        string
		boxName      string
		clubID       string
		clubName     string
		branding     string
		source       string
		aiEnabled    bool
		metrics      AnalyticsBoxMetrics
		lastIngestAt string
	}
	type clubAgg struct {
		clubID       string
		clubName     string
		branding     string
		boxes        map[string]struct{}
		sources      map[string]struct{}
		aiEnabled    bool
		metrics      AnalyticsBoxMetrics
		lastIngestAt string
	}

	boxAggs := make(map[string]*boxAgg)
	clubAggs := make(map[string]*clubAgg)

	for _, row := range mainRows {
		boxName, clubID, clubName, branding, ok := matchesFilters(row)
		if !ok {
			continue
		}
		m := row.Metrics
		aiFlag := row.AIEnabled || m.hasAIActivity()

		out.KPI.QRViews += m.QRViews
		out.KPI.PageViews += m.PageViews
		out.KPI.UniqueVisitors += m.UniqueVisitors
		out.KPI.Sessions += m.Sessions
		out.KPI.EngagedSessions += m.EngagedSessions
		out.KPI.ReturningVisitors += m.ReturningVisitors
		out.KPI.LiveHLS += m.LiveHLS
		out.KPI.VODPlays += m.VODPlays
		out.KPI.ClipDownloads += m.ClipDownloads
		out.KPI.TimelineOpens += m.TimelineOpens
		out.KPI.CutClips += m.CutClips
		out.KPI.CutSeconds += m.CutSeconds
		out.KPI.AIStudioOpens += m.AIStudioOpens
		out.KPI.AIVideosLoads += m.AIVideosLoads
		out.KPI.AIAnalyzes += m.AIAnalyzes
		out.KPI.AIShots += m.AIShots
		out.KPI.AIPots += m.AIPots
		out.KPI.AIPoolRuns += m.AIPoolRuns
		out.KPI.AICaromRuns += m.AICaromRuns
		out.KPI.AIPickleballRuns += m.AIPickleballRuns
		out.KPI.AIExportsIOS += m.AIExportsIOS
		out.KPI.AIExportsDesktop += m.AIExportsDesktop
		out.KPI.AIExportRelay += m.AIExportRelay
		out.KPI.AIExportsTotal += m.aiExportsTotal()
		out.KPI.AIErrors += m.AIErrors
		out.KPI.HTTPErrors += m.HTTPErrors
		out.KPI.BytesOut += m.BytesOut

		out.OSBreakdown.IOS += m.OSIOS
		out.OSBreakdown.Android += m.OSAndroid
		out.OSBreakdown.Desktop += m.OSDesktop
		out.OSBreakdown.App += m.OSApp
		out.OSBreakdown.Other += m.OSOther

		out.Acquisition.QR += m.AcqQR
		out.Acquisition.Share += m.AcqShare
		out.Acquisition.App += m.AcqApp
		out.Acquisition.Direct += m.AcqDirect

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
			dp := &out.DailySeries[idx]
			dp.QRViews += m.QRViews
			dp.UniqueVisitors += m.UniqueVisitors
			dp.Sessions += m.Sessions
			dp.DownloadsCuts += m.ClipDownloads + m.CutClips
			dp.AIAnalyzes += m.AIAnalyzes
			dp.AIExports += m.aiExportsTotal()
			dp.BytesOut += m.BytesOut
		}

		if !includeBreakdowns {
			continue
		}

		// Per-box aggregation
		ba, exists := boxAggs[row.BoxID]
		if !exists {
			ba = &boxAgg{
				boxID:    row.BoxID,
				boxName:  boxName,
				clubID:   clubID,
				clubName: clubName,
				branding: branding,
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
				branding: branding,
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

	out.KPI.ClipsTotal = out.KPI.ClipDownloads + out.KPI.CutClips
	out.KPI.ActiveBoxes = len(activeBoxes)
	out.KPI.ActiveClubs = len(activeClubs)
	out.KPI.AIActiveBoxes = len(aiActiveBoxes)

	out.Cohorts = buildAnalyticsCohorts(out.KPI)
	out.KPI.EngagementRatePct = out.Cohorts.EngagementRatePct

	totalReqs := max64(out.KPI.PageViews, out.KPI.QRViews) + out.KPI.LiveHLS + out.KPI.VODPlays + out.KPI.ClipsTotal + out.KPI.AIAnalyzes
	if totalReqs > 0 {
		errPct := float64(out.KPI.HTTPErrors+out.KPI.AIErrors) * 100.0 / float64(totalReqs)
		if errPct > 100.0 {
			errPct = 100.0
		}
		out.KPI.ErrorRatePct = round1(errPct)
	}

	out.Funnel = buildAnalyticsFunnel(out.KPI)

	if !includeBreakdowns {
		return out, nil
	}

	for _, ca := range clubAggs {
		srcs := sortedKeys(ca.sources)
		out.TopClubs = append(out.TopClubs, AnalyticsTopClub{
			ClubID:            ca.clubID,
			ClubName:          ca.clubName,
			Branding:          ca.branding,
			BoxesCount:        len(ca.boxes),
			Sources:           srcs,
			AIEnabled:         ca.aiEnabled,
			QRViews:           ca.metrics.QRViews,
			UniqueVisitors:    ca.metrics.UniqueVisitors,
			Sessions:          ca.metrics.Sessions,
			EngagedSessions:   ca.metrics.EngagedSessions,
			ReturningVisitors: ca.metrics.ReturningVisitors,
			LiveHLS:           ca.metrics.LiveHLS,
			VODPlays:          ca.metrics.VODPlays,
			ClipDownloads:     ca.metrics.ClipDownloads,
			CutClips:          ca.metrics.CutClips,
			AIStudioOpens:     ca.metrics.AIStudioOpens,
			AIAnalyzes:        ca.metrics.AIAnalyzes,
			AIShots:           ca.metrics.AIShots,
			AIPickleballRuns:  ca.metrics.AIPickleballRuns,
			AIExports:         ca.metrics.aiExportsTotal(),
			AIErrors:          ca.metrics.AIErrors,
			HTTPErrors:        ca.metrics.HTTPErrors,
			BytesOut:          ca.metrics.BytesOut,
			LastIngestAt:      ca.lastIngestAt,
		})
	}
	sort.Slice(out.TopClubs, func(i, j int) bool {
		if out.TopClubs[i].QRViews != out.TopClubs[j].QRViews {
			return out.TopClubs[i].QRViews > out.TopClubs[j].QRViews
		}
		di := out.TopClubs[i].ClipDownloads + out.TopClubs[i].CutClips + out.TopClubs[i].AIAnalyzes
		dj := out.TopClubs[j].ClipDownloads + out.TopClubs[j].CutClips + out.TopClubs[j].AIAnalyzes
		if di != dj {
			return di > dj
		}
		return out.TopClubs[i].ClubName < out.TopClubs[j].ClubName
	})

	for _, ba := range boxAggs {
		out.TopBoxes = append(out.TopBoxes, AnalyticsTopBox{
			BoxID:             ba.boxID,
			BoxName:           ba.boxName,
			ClubID:            ba.clubID,
			ClubName:          ba.clubName,
			Branding:          ba.branding,
			Source:            ba.source,
			AIEnabled:         ba.aiEnabled,
			QRViews:           ba.metrics.QRViews,
			UniqueVisitors:    ba.metrics.UniqueVisitors,
			Sessions:          ba.metrics.Sessions,
			EngagedSessions:   ba.metrics.EngagedSessions,
			ReturningVisitors: ba.metrics.ReturningVisitors,
			LiveHLS:           ba.metrics.LiveHLS,
			VODPlays:          ba.metrics.VODPlays,
			ClipDownloads:     ba.metrics.ClipDownloads,
			CutClips:          ba.metrics.CutClips,
			AIStudioOpens:     ba.metrics.AIStudioOpens,
			AIAnalyzes:        ba.metrics.AIAnalyzes,
			AIShots:           ba.metrics.AIShots,
			AIPickleballRuns:  ba.metrics.AIPickleballRuns,
			AIExports:         ba.metrics.aiExportsTotal(),
			AIErrors:          ba.metrics.AIErrors,
			HTTPErrors:        ba.metrics.HTTPErrors,
			BytesOut:          ba.metrics.BytesOut,
			LastIngestAt:      ba.lastIngestAt,
		})
	}
	sort.Slice(out.TopBoxes, func(i, j int) bool {
		if out.TopBoxes[i].QRViews != out.TopBoxes[j].QRViews {
			return out.TopBoxes[i].QRViews > out.TopBoxes[j].QRViews
		}
		di := out.TopBoxes[i].ClipDownloads + out.TopBoxes[i].CutClips + out.TopBoxes[i].AIAnalyzes
		dj := out.TopBoxes[j].ClipDownloads + out.TopBoxes[j].CutClips + out.TopBoxes[j].AIAnalyzes
		if di != dj {
			return di > dj
		}
		return out.TopBoxes[i].BoxID < out.TopBoxes[j].BoxID
	})

	return out, nil
}

func (s *Server) queryAnalyticsOverview(q analyticsQueryParams, now time.Time) (AnalyticsOverviewResponse, error) {
	// Ensure any coalesced ingests are committed before reading (read-your-own-writes guarantee).
	_ = s.FlushAnalytics()

	win := computeAnalyticsWindow(q.Range, q.From, q.To, now)
	meta := s.snapshotAnalyticsMetadata()

	cur, err := s.aggregateAnalyticsWindow(win, q, meta, true)
	if err != nil {
		return AnalyticsOverviewResponse{}, err
	}

	resp := AnalyticsOverviewResponse{
		OK:          true,
		Range:       win.Range,
		FromDate:    win.FromDate,
		ToDate:      win.ToDate,
		KPI:         cur.KPI,
		OSBreakdown: cur.OSBreakdown,
		PeakHours:   cur.PeakHours,
		DailySeries: cur.DailySeries,
		Funnel:      cur.Funnel,
		Heatmap7x24: cur.Heatmap7x24,
		Acquisition: cur.Acquisition,
		Cohorts:     cur.Cohorts,
		TopClubs:    cur.TopClubs,
		TopBoxes:    cur.TopBoxes,
	}

	if q.Compare {
		prevWin := computePreviousAnalyticsWindow(win)
		prev, err := s.aggregateAnalyticsWindow(prevWin, q, meta, false)
		if err != nil {
			return AnalyticsOverviewResponse{}, err
		}
		resp.Comparison = &AnalyticsComparison{
			From:        prevWin.FromDate,
			To:          prevWin.ToDate,
			KPIs:        prev.KPI,
			DeltasPct:   computeComparisonDeltas(cur.KPI, prev.KPI),
			DailySeries: prev.DailySeries,
		}
	}

	return resp, nil
}

func queryBoxHourlyRows(db *sql.DB, fromHour, toHour, boxID string) ([]boxUsageRow, error) {
	query := `
		SELECT box_id, club_id, source, ai_enabled, bucket_hour, updated_at,
			qr_views, page_views, unique_visitors, live_hls, vod_plays, clip_downloads,
			timeline_opens, cut_clips, cut_seconds,
			ai_studio_opens, ai_videos_loads, ai_analyzes, ai_shots, ai_pots,
			ai_pool_runs, ai_carom_runs, ai_pickleball_runs,
			ai_exports_ios, ai_exports_desktop, ai_export_relay,
			ai_errors, http_errors, bytes_out,
			os_ios, os_android, os_desktop, os_app, os_other,
			sessions, engaged_sessions, returning_visitors,
			acq_qr, acq_share, acq_app, acq_direct
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
			&r.Metrics.AIPoolRuns, &r.Metrics.AICaromRuns, &r.Metrics.AIPickleballRuns,
			&r.Metrics.AIExportsIOS, &r.Metrics.AIExportsDesktop, &r.Metrics.AIExportRelay,
			&r.Metrics.AIErrors, &r.Metrics.HTTPErrors, &r.Metrics.BytesOut,
			&r.Metrics.OSIOS, &r.Metrics.OSAndroid, &r.Metrics.OSDesktop, &r.Metrics.OSApp, &r.Metrics.OSOther,
			&r.Metrics.Sessions, &r.Metrics.EngagedSessions, &r.Metrics.ReturningVisitors,
			&r.Metrics.AcqQR, &r.Metrics.AcqShare, &r.Metrics.AcqApp, &r.Metrics.AcqDirect,
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
			ai_pool_runs, ai_carom_runs, ai_pickleball_runs,
			ai_exports_ios, ai_exports_desktop, ai_export_relay,
			ai_errors, http_errors, bytes_out,
			os_ios, os_android, os_desktop, os_app, os_other,
			sessions, engaged_sessions, returning_visitors,
			acq_qr, acq_share, acq_app, acq_direct
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
			&r.Metrics.AIPoolRuns, &r.Metrics.AICaromRuns, &r.Metrics.AIPickleballRuns,
			&r.Metrics.AIExportsIOS, &r.Metrics.AIExportsDesktop, &r.Metrics.AIExportRelay,
			&r.Metrics.AIErrors, &r.Metrics.HTTPErrors, &r.Metrics.BytesOut,
			&r.Metrics.OSIOS, &r.Metrics.OSAndroid, &r.Metrics.OSDesktop, &r.Metrics.OSApp, &r.Metrics.OSOther,
			&r.Metrics.Sessions, &r.Metrics.EngagedSessions, &r.Metrics.ReturningVisitors,
			&r.Metrics.AcqQR, &r.Metrics.AcqShare, &r.Metrics.AcqApp, &r.Metrics.AcqDirect,
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
	_ = s.FlushAnalytics()

	win := computeAnalyticsWindow(q.Range, q.From, q.To, now)
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
	filterSport := normalizeAnalyticsSport(q.Sport)

	out := make([]AnalyticsCameraRow, 0, len(rawRows))
	for _, r := range rawRows {
		if filterBox != "" && r.boxID != filterBox {
			continue
		}
		boxName, clubID, clubName := meta.resolveNames(r.boxID, r.clubID)
		if filterClub != "" && clubID != filterClub {
			continue
		}
		if filterSport != "all" && meta.resolveBranding(r.boxID, clubID) != filterSport {
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
		Range:   q.Get("range"),
		From:    q.Get("from"),
		To:      q.Get("to"),
		ClubID:  q.Get("club_id"),
		BoxID:   q.Get("box_id"),
		Sport:   q.Get("sport"),
		Source:  q.Get("source"),
		AIOnly:  aiOnly,
		Compare: parseAnalyticsCompareParam(q.Get("compare")),
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
	now := time.Now()
	win := computeAnalyticsWindow(q.Get("range"), q.Get("from"), q.Get("to"), now)
	limit, _ := strconv.Atoi(strings.TrimSpace(q.Get("limit")))
	cams, err := s.queryAnalyticsCameras(analyticsCameraQueryParams{
		Range:  q.Get("range"),
		From:   q.Get("from"),
		To:     q.Get("to"),
		ClubID: q.Get("club_id"),
		BoxID:  q.Get("box_id"),
		Sport:  q.Get("sport"),
		Limit:  limit,
	}, now)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "analytics_cameras_failed", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, AnalyticsCamerasResponse{
		OK:      true,
		Range:   win.Range,
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
		From:   q.Get("from"),
		To:     q.Get("to"),
		ClubID: q.Get("club_id"),
		BoxID:  q.Get("box_id"),
		Sport:  q.Get("sport"),
		Source: q.Get("source"),
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
		"qr_views", "unique_visitors", "sessions", "engaged_sessions", "returning_visitors",
		"live_hls", "vod_plays",
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
			strconv.FormatInt(b.Sessions, 10),
			strconv.FormatInt(b.EngagedSessions, 10),
			strconv.FormatInt(b.ReturningVisitors, 10),
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
