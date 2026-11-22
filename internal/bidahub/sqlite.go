package bidahub

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// sqliteMigration defines a versioned schema migration for the SQLite WAL ledger.
type sqliteMigration struct {
	Version int64
	Name    string
	Source  string
	Up      func(tx *sql.Tx) error
}

func computeMigrationChecksum(version int64, name, source string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d\n%s\n%s", version, name, source)))
	return hex.EncodeToString(h[:])
}

// addColumnIfMissing checks pragma_table_info before running ALTER TABLE ADD COLUMN (idempotent).
func addColumnIfMissing(tx *sql.Tx, table, column, ddl string) error {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", strings.ReplaceAll(table, "'", "''"))
	if err := tx.QueryRow(query, column).Scan(&count); err != nil {
		return fmt.Errorf("pragma_table_info(%s.%s): %w", table, column, err)
	}
	if count == 0 {
		if _, err := tx.Exec(ddl); err != nil {
			return fmt.Errorf("add column %s.%s: %w", table, column, err)
		}
	}
	return nil
}

var bidahubSQLiteMigrations = []sqliteMigration{
	{
		Version: 202611200001,
		Name:    "baseline_bidahub_tables",
		Source:  "CREATE TABLE clubs, boxes, users, reset_otps, lockouts, sessions, enrollments, ai_events;",
		Up: func(tx *sql.Tx) error {
			stmts := []string{
				`CREATE TABLE IF NOT EXISTS clubs (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL,
					portal_token TEXT NOT NULL,
					branding TEXT NOT NULL DEFAULT '',
					payload_json TEXT NOT NULL,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
				);`,
				`CREATE INDEX IF NOT EXISTS idx_clubs_portal_token ON clubs(portal_token);`,
				`CREATE TABLE IF NOT EXISTS boxes (
					id TEXT PRIMARY KEY,
					club_id TEXT NOT NULL,
					name TEXT NOT NULL,
					vpn_ip TEXT NOT NULL DEFAULT '',
					online INTEGER NOT NULL DEFAULT 0,
					cpu_pct REAL NOT NULL DEFAULT 0,
					ram_used_pct INTEGER NOT NULL DEFAULT 0,
					sd_used_pct INTEGER NOT NULL DEFAULT 0,
					ffmpeg_count INTEGER NOT NULL DEFAULT 0,
					rtt_ms INTEGER NOT NULL DEFAULT 0,
					payload_json TEXT NOT NULL,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
				);`,
				`CREATE INDEX IF NOT EXISTS idx_boxes_club_id ON boxes(club_id);`,
				`CREATE INDEX IF NOT EXISTS idx_boxes_online ON boxes(online);`,
				`CREATE TABLE IF NOT EXISTS users (
					username TEXT PRIMARY KEY,
					email TEXT NOT NULL DEFAULT '',
					role TEXT NOT NULL,
					password_hash TEXT NOT NULL,
					payload_json TEXT NOT NULL,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
				);`,
				`CREATE TABLE IF NOT EXISTS reset_otps (
					username TEXT PRIMARY KEY,
					email TEXT NOT NULL DEFAULT '',
					otp_hash TEXT NOT NULL,
					expires_at DATETIME NOT NULL,
					attempts INTEGER NOT NULL DEFAULT 0,
					payload_json TEXT NOT NULL
				);`,
				`CREATE TABLE IF NOT EXISTS lockouts (
					ip TEXT PRIMARY KEY,
					failed_count INTEGER NOT NULL DEFAULT 0,
					locked_until DATETIME NOT NULL,
					payload_json TEXT NOT NULL
				);`,
				`CREATE TABLE IF NOT EXISTS sessions (
					token TEXT PRIMARY KEY,
					username TEXT NOT NULL,
					role TEXT NOT NULL,
					expires_at DATETIME NOT NULL,
					payload_json TEXT NOT NULL
				);`,
				`CREATE INDEX IF NOT EXISTS idx_sessions_username ON sessions(username);`,
				`CREATE TABLE IF NOT EXISTS enrollments (
					id TEXT PRIMARY KEY,
					box_id TEXT NOT NULL,
					status TEXT NOT NULL,
					payload_json TEXT NOT NULL
				);`,
				`CREATE TABLE IF NOT EXISTS ai_events (
					id TEXT PRIMARY KEY,
					box_id TEXT NOT NULL,
					club_id TEXT NOT NULL DEFAULT '',
					device_id TEXT NOT NULL DEFAULT '',
					event_type TEXT NOT NULL DEFAULT '',
					received_at DATETIME NOT NULL,
					payload_json TEXT NOT NULL
				);`,
				`CREATE INDEX IF NOT EXISTS idx_ai_events_received_at ON ai_events(received_at DESC);`,
				`CREATE INDEX IF NOT EXISTS idx_ai_events_box_id ON ai_events(box_id);`,
			}
			for _, stmt := range stmts {
				if _, err := tx.Exec(stmt); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version: 202611200002,
		Name:    "add_boxes_telemetry_columns_via_pragma",
		Source:  "ALTER TABLE boxes ADD COLUMN telemetry_source, hardening_score;",
		Up: func(tx *sql.Tx) error {
			if err := addColumnIfMissing(tx, "boxes", "telemetry_source",
				"ALTER TABLE boxes ADD COLUMN telemetry_source TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := addColumnIfMissing(tx, "boxes", "hardening_score",
				"ALTER TABLE boxes ADD COLUMN hardening_score INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			return nil
		},
	},
	{
		Version: 202611220001,
		Name:    "add_usage_analytics_tables",
		Source:  "CREATE TABLE usage_box_hourly, usage_box_daily, usage_camera_daily, analytics_ingest_batches;",
		Up: func(tx *sql.Tx) error {
			stmts := []string{
				`CREATE TABLE IF NOT EXISTS usage_box_hourly (
					box_id TEXT NOT NULL,
					club_id TEXT NOT NULL DEFAULT '',
					source TEXT NOT NULL DEFAULT '',
					ai_enabled INTEGER NOT NULL DEFAULT 0,
					source_epoch TEXT NOT NULL DEFAULT '',
					bucket_hour TEXT NOT NULL,
					qr_views INTEGER NOT NULL DEFAULT 0,
					page_views INTEGER NOT NULL DEFAULT 0,
					unique_visitors INTEGER NOT NULL DEFAULT 0,
					live_hls INTEGER NOT NULL DEFAULT 0,
					vod_plays INTEGER NOT NULL DEFAULT 0,
					clip_downloads INTEGER NOT NULL DEFAULT 0,
					timeline_opens INTEGER NOT NULL DEFAULT 0,
					cut_clips INTEGER NOT NULL DEFAULT 0,
					cut_seconds INTEGER NOT NULL DEFAULT 0,
					ai_studio_opens INTEGER NOT NULL DEFAULT 0,
					ai_videos_loads INTEGER NOT NULL DEFAULT 0,
					ai_analyzes INTEGER NOT NULL DEFAULT 0,
					ai_shots INTEGER NOT NULL DEFAULT 0,
					ai_pots INTEGER NOT NULL DEFAULT 0,
					ai_pool_runs INTEGER NOT NULL DEFAULT 0,
					ai_carom_runs INTEGER NOT NULL DEFAULT 0,
					ai_exports_ios INTEGER NOT NULL DEFAULT 0,
					ai_exports_desktop INTEGER NOT NULL DEFAULT 0,
					ai_export_relay INTEGER NOT NULL DEFAULT 0,
					ai_errors INTEGER NOT NULL DEFAULT 0,
					http_errors INTEGER NOT NULL DEFAULT 0,
					bytes_out INTEGER NOT NULL DEFAULT 0,
					os_ios INTEGER NOT NULL DEFAULT 0,
					os_android INTEGER NOT NULL DEFAULT 0,
					os_desktop INTEGER NOT NULL DEFAULT 0,
					os_app INTEGER NOT NULL DEFAULT 0,
					os_other INTEGER NOT NULL DEFAULT 0,
					base_json TEXT NOT NULL DEFAULT '{}',
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (box_id, bucket_hour)
				);`,
				`CREATE INDEX IF NOT EXISTS idx_usage_box_hourly_hour ON usage_box_hourly(bucket_hour);`,
				`CREATE INDEX IF NOT EXISTS idx_usage_box_hourly_club_hour ON usage_box_hourly(club_id, bucket_hour);`,
				`CREATE TABLE IF NOT EXISTS usage_box_daily (
					box_id TEXT NOT NULL,
					club_id TEXT NOT NULL DEFAULT '',
					source TEXT NOT NULL DEFAULT '',
					ai_enabled INTEGER NOT NULL DEFAULT 0,
					source_epoch TEXT NOT NULL DEFAULT '',
					bucket_date TEXT NOT NULL,
					qr_views INTEGER NOT NULL DEFAULT 0,
					page_views INTEGER NOT NULL DEFAULT 0,
					unique_visitors INTEGER NOT NULL DEFAULT 0,
					live_hls INTEGER NOT NULL DEFAULT 0,
					vod_plays INTEGER NOT NULL DEFAULT 0,
					clip_downloads INTEGER NOT NULL DEFAULT 0,
					timeline_opens INTEGER NOT NULL DEFAULT 0,
					cut_clips INTEGER NOT NULL DEFAULT 0,
					cut_seconds INTEGER NOT NULL DEFAULT 0,
					ai_studio_opens INTEGER NOT NULL DEFAULT 0,
					ai_videos_loads INTEGER NOT NULL DEFAULT 0,
					ai_analyzes INTEGER NOT NULL DEFAULT 0,
					ai_shots INTEGER NOT NULL DEFAULT 0,
					ai_pots INTEGER NOT NULL DEFAULT 0,
					ai_pool_runs INTEGER NOT NULL DEFAULT 0,
					ai_carom_runs INTEGER NOT NULL DEFAULT 0,
					ai_exports_ios INTEGER NOT NULL DEFAULT 0,
					ai_exports_desktop INTEGER NOT NULL DEFAULT 0,
					ai_export_relay INTEGER NOT NULL DEFAULT 0,
					ai_errors INTEGER NOT NULL DEFAULT 0,
					http_errors INTEGER NOT NULL DEFAULT 0,
					bytes_out INTEGER NOT NULL DEFAULT 0,
					os_ios INTEGER NOT NULL DEFAULT 0,
					os_android INTEGER NOT NULL DEFAULT 0,
					os_desktop INTEGER NOT NULL DEFAULT 0,
					os_app INTEGER NOT NULL DEFAULT 0,
					os_other INTEGER NOT NULL DEFAULT 0,
					base_json TEXT NOT NULL DEFAULT '{}',
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (box_id, bucket_date)
				);`,
				`CREATE INDEX IF NOT EXISTS idx_usage_box_daily_date ON usage_box_daily(bucket_date);`,
				`CREATE INDEX IF NOT EXISTS idx_usage_box_daily_club_date ON usage_box_daily(club_id, bucket_date);`,
				`CREATE TABLE IF NOT EXISTS usage_camera_daily (
					box_id TEXT NOT NULL,
					club_id TEXT NOT NULL DEFAULT '',
					camera_id TEXT NOT NULL,
					table_name TEXT NOT NULL DEFAULT '',
					source_epoch TEXT NOT NULL DEFAULT '',
					bucket_date TEXT NOT NULL,
					qr_views INTEGER NOT NULL DEFAULT 0,
					unique_visitors INTEGER NOT NULL DEFAULT 0,
					live_hls INTEGER NOT NULL DEFAULT 0,
					vod_plays INTEGER NOT NULL DEFAULT 0,
					clip_downloads INTEGER NOT NULL DEFAULT 0,
					timeline_opens INTEGER NOT NULL DEFAULT 0,
					cut_clips INTEGER NOT NULL DEFAULT 0,
					cut_seconds INTEGER NOT NULL DEFAULT 0,
					ai_studio_opens INTEGER NOT NULL DEFAULT 0,
					ai_analyzes INTEGER NOT NULL DEFAULT 0,
					ai_shots INTEGER NOT NULL DEFAULT 0,
					ai_pots INTEGER NOT NULL DEFAULT 0,
					ai_exports INTEGER NOT NULL DEFAULT 0,
					ai_errors INTEGER NOT NULL DEFAULT 0,
					bytes_out INTEGER NOT NULL DEFAULT 0,
					base_json TEXT NOT NULL DEFAULT '{}',
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (box_id, bucket_date, camera_id)
				);`,
				`CREATE INDEX IF NOT EXISTS idx_usage_cam_daily_date ON usage_camera_daily(bucket_date);`,
				`CREATE INDEX IF NOT EXISTS idx_usage_cam_daily_club_date ON usage_camera_daily(club_id, bucket_date);`,
				`CREATE TABLE IF NOT EXISTS analytics_ingest_batches (
					box_id TEXT NOT NULL,
					batch_id TEXT NOT NULL,
					received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (box_id, batch_id)
				);`,
				`CREATE INDEX IF NOT EXISTS idx_analytics_ingest_batches_received_at ON analytics_ingest_batches(received_at);`,
			}
			for _, stmt := range stmts {
				if _, err := tx.Exec(stmt); err != nil {
					return err
				}
			}
			return nil
		},
	},
}

func resolveSQLiteAndJSONPaths(dataPath string) (sqlitePath, legacyJSONPath, mirrorJSONPath string) {
	dataPath = strings.TrimSpace(dataPath)
	if dataPath == "" {
		return "", "", ""
	}
	if dataPath == ":memory:" {
		return ":memory:", "", ""
	}
	lower := strings.ToLower(dataPath)
	if strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite3") {
		dir := filepath.Dir(dataPath)
		return dataPath, filepath.Join(dir, "hub-state.json"), ""
	}
	if strings.HasSuffix(lower, ".json") {
		base := strings.TrimSuffix(dataPath, filepath.Ext(dataPath))
		return base + ".sqlite", dataPath, dataPath
	}
	return dataPath + ".sqlite", dataPath, ""
}

func openAndMigrateSQLite(sqlitePath string) (*sql.DB, error) {
	if sqlitePath == "" {
		return nil, nil
	}
	if sqlitePath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir sqlite dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", sqlitePath, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pragmas := []string{
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA foreign_keys = ON;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("exec %s: %w", p, err)
		}
	}

	if err := runSQLiteMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func runSQLiteMigrations(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			checksum TEXT NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			execution_ms INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	rows, err := db.Query("SELECT version, checksum FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("query schema_migrations: %w", err)
	}
	applied := make(map[int64]string)
	for rows.Next() {
		var v int64
		var csum string
		if err := rows.Scan(&v, &csum); err != nil {
			_ = rows.Close()
			return err
		}
		applied[v] = csum
	}
	_ = rows.Close()

	migrations := append([]sqliteMigration(nil), bidahubSQLiteMigrations...)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	for _, m := range migrations {
		if _, ok := applied[m.Version]; ok {
			continue
		}
		t0 := time.Now()
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if err := m.Up(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}
		ms := time.Since(t0).Milliseconds()
		csum := computeMigrationChecksum(m.Version, m.Name, m.Source)
		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (version, name, checksum, execution_ms) VALUES (?, ?, ?, ?)",
			m.Version, m.Name, csum, ms,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.Version, err)
		}
	}
	return nil
}

func loadStateFromSQLite(db *sql.DB) (persistedState, bool, error) {
	var st persistedState
	if db == nil {
		return st, false, nil
	}

	// 1. Clubs
	cRows, err := db.Query("SELECT payload_json FROM clubs ORDER BY rowid ASC")
	if err != nil {
		return st, false, err
	}
	for cRows.Next() {
		var raw string
		if err := cRows.Scan(&raw); err != nil {
			_ = cRows.Close()
			return st, false, err
		}
		var c Club
		if err := json.Unmarshal([]byte(raw), &c); err == nil && c.ID != "" {
			st.Clubs = append(st.Clubs, &c)
		}
	}
	_ = cRows.Close()

	// 2. Boxes
	bRows, err := db.Query("SELECT payload_json FROM boxes ORDER BY rowid ASC")
	if err != nil {
		return st, false, err
	}
	for bRows.Next() {
		var raw string
		if err := bRows.Scan(&raw); err != nil {
			_ = bRows.Close()
			return st, false, err
		}
		var b Box
		if err := json.Unmarshal([]byte(raw), &b); err == nil && b.ID != "" {
			st.Boxes = append(st.Boxes, &b)
		}
	}
	_ = bRows.Close()

	if len(st.Clubs) == 0 || len(st.Boxes) == 0 {
		return st, false, nil
	}

	// 3. Users
	st.Users = make(map[string]*UserAccount)
	if rows, err := db.Query("SELECT username, payload_json FROM users"); err == nil {
		for rows.Next() {
			var k, raw string
			if rows.Scan(&k, &raw) == nil {
				var u UserAccount
				if json.Unmarshal([]byte(raw), &u) == nil {
					st.Users[k] = &u
				}
			}
		}
		_ = rows.Close()
	}

	// 4. ResetOTPs
	st.ResetOTPs = make(map[string]*PasswordResetOTP)
	if rows, err := db.Query("SELECT username, payload_json FROM reset_otps"); err == nil {
		for rows.Next() {
			var k, raw string
			if rows.Scan(&k, &raw) == nil {
				var o PasswordResetOTP
				if json.Unmarshal([]byte(raw), &o) == nil {
					st.ResetOTPs[k] = &o
				}
			}
		}
		_ = rows.Close()
	}

	// 5. Lockouts
	st.Lockouts = make(map[string]*LockoutEntry)
	if rows, err := db.Query("SELECT ip, payload_json FROM lockouts"); err == nil {
		for rows.Next() {
			var k, raw string
			if rows.Scan(&k, &raw) == nil {
				var l LockoutEntry
				if json.Unmarshal([]byte(raw), &l) == nil {
					st.Lockouts[k] = &l
				}
			}
		}
		_ = rows.Close()
	}

	// 6. Sessions
	st.Sessions = make(map[string]*Session)
	if rows, err := db.Query("SELECT token, payload_json FROM sessions"); err == nil {
		for rows.Next() {
			var k, raw string
			if rows.Scan(&k, &raw) == nil {
				var sess Session
				if json.Unmarshal([]byte(raw), &sess) == nil {
					st.Sessions[k] = &sess
				}
			}
		}
		_ = rows.Close()
	}

	// 7. Enrollments
	st.Enrollments = make(map[string]*EnrollmentRequest)
	if rows, err := db.Query("SELECT id, payload_json FROM enrollments"); err == nil {
		for rows.Next() {
			var k, raw string
			if rows.Scan(&k, &raw) == nil {
				var e EnrollmentRequest
				if json.Unmarshal([]byte(raw), &e) == nil {
					st.Enrollments[k] = &e
				}
			}
		}
		_ = rows.Close()
	}

	// 8. AIEvents
	if rows, err := db.Query("SELECT payload_json FROM ai_events ORDER BY rowid ASC"); err == nil {
		for rows.Next() {
			var raw string
			if rows.Scan(&raw) == nil {
				var ev AIEvent
				if json.Unmarshal([]byte(raw), &ev) == nil {
					st.AIEvents = append(st.AIEvents, &ev)
				}
			}
		}
		_ = rows.Close()
	}

	return st, true, nil
}

func upsertClubTx(tx *sql.Tx, c *Club, now time.Time) error {
	if c == nil {
		return nil
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO clubs (id, name, portal_token, branding, payload_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			portal_token = excluded.portal_token,
			branding = excluded.branding,
			payload_json = excluded.payload_json,
			updated_at = excluded.updated_at
	`, c.ID, c.Name, c.PortalToken, c.Branding, string(raw), now)
	return err
}

func upsertBoxTx(tx *sql.Tx, b *Box, now time.Time) error {
	if b == nil {
		return nil
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	onlineInt := 0
	if b.Online {
		onlineInt = 1
	}
	_, err = tx.Exec(`
		INSERT INTO boxes (
			id, club_id, name, vpn_ip, online, cpu_pct, ram_used_pct, sd_used_pct,
			ffmpeg_count, rtt_ms, telemetry_source, hardening_score, payload_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			club_id = excluded.club_id,
			name = excluded.name,
			vpn_ip = excluded.vpn_ip,
			online = excluded.online,
			cpu_pct = excluded.cpu_pct,
			ram_used_pct = excluded.ram_used_pct,
			sd_used_pct = excluded.sd_used_pct,
			ffmpeg_count = excluded.ffmpeg_count,
			rtt_ms = excluded.rtt_ms,
			telemetry_source = excluded.telemetry_source,
			hardening_score = excluded.hardening_score,
			payload_json = excluded.payload_json,
			updated_at = excluded.updated_at
	`, b.ID, b.ClubID, b.Name, b.VPNIP, onlineInt, b.CPUPct, b.RAMUsedPct, b.SDUsedPct,
		b.FFmpegCount, b.RTTMs, b.TelemetrySource, b.HardeningScore, string(raw), now)
	return err
}

// syncDirtyTelemetryToSQLiteLocked writes ONLY the modified boxes and their parent clubs
// to the SQLite WAL database in a single lightweight transaction.
func (s *Store) syncDirtyTelemetryToSQLiteLocked() error {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.db == nil || (len(s.dirtyBoxes) == 0 && len(s.dirtyClubs) == 0) {
		s.dirtyBoxes = nil
		s.dirtyClubs = nil
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for boxID := range s.dirtyBoxes {
		if b, ok := s.boxByID[boxID]; ok && b != nil {
			if err := upsertBoxTx(tx, b, now); err != nil {
				_ = tx.Rollback()
				return err
			}
			if b.ClubID != "" {
				if s.dirtyClubs == nil {
					s.dirtyClubs = make(map[string]struct{})
				}
				s.dirtyClubs[b.ClubID] = struct{}{}
			}
		}
	}
	for clubID := range s.dirtyClubs {
		if c, ok := s.clubByID[clubID]; ok && c != nil {
			if err := upsertClubTx(tx, c, now); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.dirtyBoxes = nil
	s.dirtyClubs = nil
	return nil
}

// syncAllToSQLiteLocked persists the full Store state into SQLite in a single transaction.
func (s *Store) syncAllToSQLiteLocked() error {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.db == nil {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	now := time.Now().UTC()

	for _, c := range s.clubs {
		if err := upsertClubTx(tx, c, now); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	for _, b := range s.boxes {
		if err := upsertBoxTx(tx, b, now); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM users"); err != nil {
		_ = tx.Rollback()
		return err
	}
	for k, u := range s.users {
		if u == nil {
			continue
		}
		raw, _ := json.Marshal(u)
		if _, err := tx.Exec(
			"INSERT INTO users (username, email, role, password_hash, payload_json, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			k, u.Email, u.Role, u.PasswordHash, string(raw), now,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM reset_otps"); err != nil {
		_ = tx.Rollback()
		return err
	}
	for k, o := range s.resetOTPs {
		if o == nil {
			continue
		}
		raw, _ := json.Marshal(o)
		if _, err := tx.Exec(
			"INSERT INTO reset_otps (username, email, otp_hash, expires_at, attempts, payload_json) VALUES (?, ?, ?, ?, ?, ?)",
			k, o.Email, o.OTPHash, o.ExpiresAt, o.Attempts, string(raw),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM lockouts"); err != nil {
		_ = tx.Rollback()
		return err
	}
	for k, l := range s.lockouts {
		if l == nil {
			continue
		}
		raw, _ := json.Marshal(l)
		if _, err := tx.Exec(
			"INSERT INTO lockouts (ip, failed_count, locked_until, payload_json) VALUES (?, ?, ?, ?)",
			k, l.FailedCount, l.LockedUntil, string(raw),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM sessions"); err != nil {
		_ = tx.Rollback()
		return err
	}
	for k, sess := range s.sessions {
		if sess == nil {
			continue
		}
		raw, _ := json.Marshal(sess)
		if _, err := tx.Exec(
			"INSERT INTO sessions (token, username, role, expires_at, payload_json) VALUES (?, ?, ?, ?, ?)",
			k, sess.Username, sess.Role, sess.ExpiresAt, string(raw),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM enrollments"); err != nil {
		_ = tx.Rollback()
		return err
	}
	for k, e := range s.enrollments {
		if e == nil {
			continue
		}
		raw, _ := json.Marshal(e)
		if _, err := tx.Exec(
			"INSERT INTO enrollments (id, box_id, status, payload_json) VALUES (?, ?, ?, ?)",
			k, e.BoxID, e.Status, string(raw),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM ai_events"); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, ev := range s.aiEvents {
		if ev == nil {
			continue
		}
		raw, _ := json.Marshal(ev)
		if _, err := tx.Exec(
			"INSERT INTO ai_events (id, box_id, club_id, device_id, event_type, received_at, payload_json) VALUES (?, ?, ?, ?, ?, ?, ?)",
			ev.ID, ev.BoxID, ev.ClubID, ev.DeviceID, ev.EventType, ev.ReceivedAt, string(raw),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	s.dirtyBoxes = nil
	s.dirtyClubs = nil
	return nil
}
