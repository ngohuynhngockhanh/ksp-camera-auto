package bidahub

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type persistedState = StoreData

// Store manages the thread-safe, atomically persisted fleet state.
type Store struct {
	mu          sync.RWMutex
	dataPath    string
	clubs       []*Club
	clubByID    map[string]*Club
	clubByToken map[string]*Club
	boxes       []*Box
	boxByID     map[string]*Box
	users       map[string]*UserAccount
	resetOTPs   map[string]*PasswordResetOTP
	lockouts    map[string]*LockoutEntry
	sessions    map[string]*Session
	enrollments map[string]*EnrollmentRequest
	aiEvents    []*AIEvent
}

// NewStore loads state from dataPath if present, seeding from watchlistPath (or built-in catalog) and persisting atomically.
func NewStore(dataPath string, watchlistPath string) (*Store, error) {
	s := &Store{
		dataPath:    dataPath,
		clubByID:    make(map[string]*Club),
		clubByToken: make(map[string]*Club),
		boxByID:     make(map[string]*Box),
		users:       make(map[string]*UserAccount),
		resetOTPs:   make(map[string]*PasswordResetOTP),
		lockouts:    make(map[string]*LockoutEntry),
		sessions:    make(map[string]*Session),
		enrollments: make(map[string]*EnrollmentRequest),
	}

	seedClubs, seedBoxes := LoadOrSeedWatchlist(watchlistPath)

	loaded := false
	if dataPath != "" {
		if raw, err := os.ReadFile(dataPath); err == nil && len(raw) > 0 {
			var st persistedState
			if err := json.Unmarshal(raw, &st); err == nil && len(st.Clubs) > 0 && len(st.Boxes) > 0 {
				s.clubs = st.Clubs
				s.boxes = st.Boxes
				if st.Users != nil {
					s.users = st.Users
				}
				if st.ResetOTPs != nil {
					s.resetOTPs = st.ResetOTPs
				}
				if st.Lockouts != nil {
					s.lockouts = st.Lockouts
				}
				if st.Sessions != nil {
					s.sessions = st.Sessions
				}
				if st.Enrollments != nil {
					s.enrollments = st.Enrollments
				}
				if st.AIEvents != nil {
					s.aiEvents = st.AIEvents
				}
				loaded = true
			}
		}
	}

	if !loaded {
		s.clubs = seedClubs
		s.boxes = seedBoxes
	} else {
		// Merge any newly added watchlist clubs/boxes without overwriting persisted keys/tokens
		existingClubs := make(map[string]*Club, len(s.clubs))
		for _, c := range s.clubs {
			if c.DLHMACKey == "" {
				c.DLHMACKey = randomHex(32, "hmac:"+c.ID)
			}
			if c.PortalToken == "" {
				c.PortalToken = randomHex(16, "portal:"+c.ID)
			}
			c.PortalURL = "/portal/" + c.PortalToken + "/"
			existingClubs[c.ID] = c
		}
		for _, sc := range seedClubs {
			if _, ok := existingClubs[sc.ID]; !ok {
				s.clubs = append(s.clubs, sc)
				existingClubs[sc.ID] = sc
			}
		}

		existingBoxes := make(map[string]*Box, len(s.boxes))
		for _, b := range s.boxes {
			existingBoxes[b.ID] = b
		}
		for _, sb := range seedBoxes {
			if _, ok := existingBoxes[sb.ID]; !ok {
				if c, ok := existingClubs[sb.ClubID]; ok {
					sb.PortalToken = c.PortalToken
				}
				s.boxes = append(s.boxes, sb)
				existingBoxes[sb.ID] = sb
			}
		}
	}

	s.rebuildIndexesLocked()
	if dataPath != "" {
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) rebuildIndexesLocked() {
	s.clubByID = make(map[string]*Club, len(s.clubs))
	s.clubByToken = make(map[string]*Club, len(s.clubs))
	s.boxByID = make(map[string]*Box, len(s.boxes))

	for _, c := range s.clubs {
		if c.DLHMACKey == "" {
			c.DLHMACKey = randomHex(32, "hmac:"+c.ID)
		}
		if c.PortalToken == "" {
			c.PortalToken = randomHex(16, "portal:"+c.ID)
		}
		c.PortalURL = "/portal/" + c.PortalToken + "/"
		c.BoxIDs = nil
		c.BoxesCount = 0
		c.OnlineBoxesCount = 0
		c.CamerasCount = 0
		s.clubByID[c.ID] = c
		s.clubByToken[c.PortalToken] = c
	}

	for _, b := range s.boxes {
		s.boxByID[b.ID] = b
		if b.RAMTotalMB > 0 && b.RAMUsedMB == 0 && b.RAMAvailableMB > 0 && b.RAMTotalMB > b.RAMAvailableMB {
			b.RAMUsedMB = b.RAMTotalMB - b.RAMAvailableMB
		}
		if b.RAMUsedPct == 0 && b.RAMPct > 0 {
			b.RAMUsedPct = b.RAMPct
		} else if b.RAMPct == 0 && b.RAMUsedPct > 0 {
			b.RAMPct = b.RAMUsedPct
		}
		if b.SwapTotalMB == 0 && b.ZRAMTotalMB > 0 {
			b.SwapTotalMB = b.ZRAMTotalMB
		}
		if b.SwapUsedMB == 0 && b.ZRAMUsedMB > 0 {
			b.SwapUsedMB = b.ZRAMUsedMB
		}
		if b.SwapFreeMB == 0 && b.SwapTotalMB > b.SwapUsedMB {
			b.SwapFreeMB = b.SwapTotalMB - b.SwapUsedMB
		}
		if b.ZRAMPct == 0 && b.SwapTotalMB > 0 && b.SwapUsedMB > 0 {
			b.ZRAMPct = (b.SwapUsedMB * 100) / b.SwapTotalMB
		}
		if b.CPUCores <= 0 {
			b.CPUCores = 4
		}
		if b.CamerasCount == 0 && len(b.Cameras) > 0 {
			b.CamerasCount = len(b.Cameras)
		}
		if c, ok := s.clubByID[b.ClubID]; ok {
			b.ClubName = c.Name
			b.PortalToken = c.PortalToken
			if len(c.BoxIDs) == 0 {
				if b.StoreDomain != "" {
					c.StoreURL = "https://" + b.StoreDomain
				}
				if b.CStoreDomain != "" {
					c.CStoreURL = "https://" + b.CStoreDomain
				}
			}
			c.BoxIDs = append(c.BoxIDs, b.ID)
			c.BoxesCount = len(c.BoxIDs)
			if b.Online {
				c.OnlineBoxesCount++
			}
			if len(b.Cameras) > 0 {
				c.CamerasCount += len(b.Cameras)
			} else {
				c.CamerasCount += b.CamerasCount
			}
		}
		if b.HardeningMax == 0 {
			b.HardeningMax = 8
		}
		if b.RedBidaMax == 0 {
			b.RedBidaMax = 17
		}
	}
}

func (s *Store) snapshotClubLocked(c *Club) *Club {
	cp := *c
	var boxIDs []string
	onlineCount := 0
	camsCount := 0
	for _, b := range s.boxes {
		if b.ClubID == c.ID {
			boxIDs = append(boxIDs, b.ID)
			if b.Online {
				onlineCount++
			}
			if len(b.Cameras) > 0 {
				camsCount += len(b.Cameras)
			} else {
				camsCount += b.CamerasCount
			}
		}
	}
	if len(boxIDs) > 0 {
		cp.BoxIDs = boxIDs
		cp.BoxesCount = len(boxIDs)
		cp.OnlineBoxesCount = onlineCount
		cp.CamerasCount = camsCount
	} else {
		cp.BoxIDs = append([]string(nil), c.BoxIDs...)
	}
	return &cp
}

func (s *Store) saveLocked() error {
	if s.dataPath == "" {
		return nil
	}
	dir := filepath.Dir(s.dataPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	st := persistedState{
		Version:     1,
		UpdatedAt:   time.Now().UTC(),
		Clubs:       s.clubs,
		Boxes:       s.boxes,
		Users:       s.users,
		ResetOTPs:   s.resetOTPs,
		Lockouts:    s.lockouts,
		Sessions:    s.sessions,
		Enrollments: s.enrollments,
		AIEvents:    s.aiEvents,
	}
	payload, err := json.MarshalIndent(&st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tmpPath := fmt.Sprintf("%s.tmp.%d", s.dataPath, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tmpPath, s.dataPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}

// Save persists the current state atomically to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rebuildIndexesLocked()
	return s.saveLocked()
}

// Clubs returns a snapshot slice of all clubs with dynamically recomputed online/box/camera counts.
func (s *Store) Clubs() []*Club {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Club, len(s.clubs))
	for i, c := range s.clubs {
		out[i] = s.snapshotClubLocked(c)
	}
	return out
}

// GetClub looks up a club by ID with dynamically recomputed online/box/camera counts.
func (s *Store) GetClub(id string) (*Club, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clubByID[id]
	if !ok {
		return nil, false
	}
	return s.snapshotClubLocked(c), true
}

// GetClubByPortalToken looks up a club strictly by its 32-hex portal_token.
func (s *Store) GetClubByPortalToken(token string) (*Club, bool) {
	token = strings.TrimSpace(token)
	if len(token) < 16 {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clubByToken[token]
	if !ok {
		return nil, false
	}
	return s.snapshotClubLocked(c), true
}

// Boxes returns a deep-enough copy of all boxes.
func (s *Store) Boxes() []*Box {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Box, len(s.boxes))
	for i, b := range s.boxes {
		out[i] = cloneBox(b)
	}
	return out
}

// GetBox returns a cloned Box by ID.
func (s *Store) GetBox(id string) (*Box, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.boxByID[id]
	if !ok {
		return nil, false
	}
	return cloneBox(b), true
}

// GetBoxesByClub returns all boxes belonging to a given clubID.
func (s *Store) GetBoxesByClub(clubID string) []*Box {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Box
	for _, b := range s.boxes {
		if b.ClubID == clubID {
			out = append(out, cloneBox(b))
		}
	}
	return out
}

// UpdateBox mutates a box in-place under write lock and persists state.
func (s *Store) UpdateBox(id string, fn func(b *Box) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.boxByID[id]
	if !ok {
		return fmt.Errorf("box %q not found", id)
	}
	if err := fn(b); err != nil {
		return err
	}
	s.rebuildIndexesLocked()
	return s.saveLocked()
}

// BatchUpdateBoxes mutates multiple boxes in-place under a single write lock, rebuilds indexes, and persists state once.
func (s *Store) BatchUpdateBoxes(fn func(boxByID map[string]*Box) int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated := fn(s.boxByID)
	s.rebuildIndexesLocked()
	if err := s.saveLocked(); err != nil {
		return updated, err
	}
	return updated, nil
}

// Summary returns a consistent snapshot of KPI, Clubs, and Boxes.
func (s *Store) Summary() (FleetKPI, []*Club, []*Box) {
	kpi := s.ComputeKPI()
	clubs := s.Clubs()
	boxes := s.Boxes()
	return kpi, clubs, boxes
}

// AllCameras returns all cameras across all boxes and the total double-streaming count.
func (s *Store) AllCameras() ([]*Camera, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var all []*Camera
	doubleCount := 0
	for _, b := range s.boxes {
		for _, cam := range b.Cameras {
			cp := *cam
			cp.BoxID = b.ID
			cp.BoxName = b.Name
			cp.ClubID = b.ClubID
			cp.ClubName = b.ClubName
			if cp.DoubleStreaming {
				doubleCount++
			}
			all = append(all, &cp)
		}
	}
	return all, doubleCount
}

// RecordEnrollment adds or updates an enrollment request for an unknown box.
func (s *Store) RecordEnrollment(boxID, secret, remoteIP, socModel string) *EnrollmentRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	sum := sha256.Sum256([]byte(secret + ":" + boxID))
	fp := hex.EncodeToString(sum[:])[:12]
	req, exists := s.enrollments[boxID]
	if !exists {
		req = &EnrollmentRequest{
			ID:          boxID,
			BoxID:       boxID,
			Fingerprint: fp,
			RemoteIP:    remoteIP,
			SoCModel:    socModel,
			Status:      "pending",
			FirstSeen:   time.Now().UTC(),
		}
		s.enrollments[boxID] = req
	} else {
		req.RemoteIP = remoteIP
		if socModel != "" {
			req.SoCModel = socModel
		}
	}
	_ = s.saveLocked()
	cp := *req
	return &cp
}

// Enrollments returns all enrollment requests.
func (s *Store) Enrollments() []*EnrollmentRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*EnrollmentRequest, 0, len(s.enrollments))
	for _, e := range s.enrollments {
		cp := *e
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// ApproveEnrollment approves a pending enrollment and creates a Box in the store.
func (s *Store) ApproveEnrollment(id, clubID string) (*Box, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.enrollments[id]
	if !ok {
		return nil, fmt.Errorf("enrollment %q not found", id)
	}
	req.Status = "approved"
	if clubID == "" {
		clubID = s.clubs[0].ID
	}
	club, ok := s.clubByID[clubID]
	if !ok {
		club = s.clubs[0]
	}
	box, exists := s.boxByID[req.BoxID]
	if !exists {
		sp := seedBoxSpec{
			ID:              req.BoxID,
			ClubID:          club.ID,
			ClubName:        club.Name,
			BoxName:         club.Name + " - " + req.BoxID,
			Branding:        club.Branding,
			VPNIP:           req.RemoteIP,
			LANIP:           "192.168.1.10",
			VirtualIP:       "192.168.1.254",
			SoCModel:        req.SoCModel,
			Arch:            "aarch64",
			RAMTotalMB:      1945,
			RAMAvailableMB:  1024,
			ZRAMTotalMB:     1945,
			ZRAMAlgo:        "lz4",
			SDTotalGB:       64,
			SDUsedPct:       40,
			SDFSType:        "f2fs",
			SDMountOpts:     "rw,noatime,errors=continue,users",
			CamerasCount:    6,
			Vendor:          "Dahua",
			Resolution:      "1920x1080",
			Width:           1920,
			Height:          1080,
			FPS:             30,
			BitrateKbps:     1536,
			Codec:           "H.265",
			ShinobiGroupKey: slugify(req.BoxID),
			StoreDomain:     "store-" + slugify(req.BoxID) + ".video.io.vn",
			CStoreDomain:    "c-store-" + slugify(req.BoxID) + ".video.io.vn",
			PM2CameraMB:     180,
			HardeningScore:  8,
		}
		box = materializeBox(sp, club, len(s.boxes), time.Now().UTC())
		s.boxes = append(s.boxes, box)
	}
	s.rebuildIndexesLocked()
	_ = s.saveLocked()
	return cloneBox(box), nil
}

// CheckLockout returns whether ip is currently locked out and the unlock timestamp.
func (s *Store) CheckLockout(ip string, now time.Time) (bool, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.lockouts[ip]
	if !ok {
		return false, time.Time{}
	}
	if !entry.LockedUntil.IsZero() && now.Before(entry.LockedUntil) {
		return true, entry.LockedUntil
	}
	return false, time.Time{}
}

// RecordLoginFailure increments the failed login counter for ip and locks for 30m on >=5 failures within 15m.
func (s *Store) RecordLoginFailure(ip string, now time.Time) (locked bool, count int, lockedUntil time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.lockouts[ip]
	if !ok || (!entry.FirstFailedAt.IsZero() && now.Sub(entry.FirstFailedAt) > 15*time.Minute && now.After(entry.LockedUntil)) {
		entry = &LockoutEntry{
			IP:            ip,
			FailedCount:   0,
			FirstFailedAt: now,
		}
		s.lockouts[ip] = entry
	}
	entry.FailedCount++
	entry.UpdatedAt = now
	if entry.FailedCount >= 5 {
		entry.LockedUntil = now.Add(30 * time.Minute)
		locked = true
	}
	_ = s.saveLocked()
	return locked, entry.FailedCount, entry.LockedUntil
}

// ClearLockout clears failed login attempts for ip on successful login.
func (s *Store) ClearLockout(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.lockouts[ip]; ok {
		delete(s.lockouts, ip)
		_ = s.saveLocked()
	}
}

// CreateSession creates and persists an authenticated browser session.
func (s *Store) CreateSession(username, role, ip string, ttl time.Duration) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	tok := randomHex(24, fmt.Sprintf("sess:%s:%d", username, now.UnixNano()))
	sess := &Session{
		Token:     tok,
		Username:  username,
		Role:      role,
		IP:        ip,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	s.sessions[tok] = sess
	_ = s.saveLocked()
	cp := *sess
	return &cp
}

// GetSession retrieves a valid session by token.
func (s *Store) GetSession(token string, now time.Time) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[token]
	if !ok {
		return nil, false
	}
	if now.After(sess.ExpiresAt) {
		return nil, false
	}
	cp := *sess
	return &cp, true
}

// DeleteSession removes a session token.
func (s *Store) DeleteSession(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[token]; ok {
		delete(s.sessions, token)
		_ = s.saveLocked()
	}
}

// DeleteSessionsForUser removes all active sessions for a username and returns the count removed.
func (s *Store) DeleteSessionsForUser(username string) int {
	username = strings.TrimSpace(username)
	if username == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.findUserLocked(username); ok {
		username = u.Username
	}
	removed := 0
	for tok, sess := range s.sessions {
		if sess != nil && strings.EqualFold(sess.Username, username) {
			delete(s.sessions, tok)
			removed++
		}
	}
	if removed > 0 {
		_ = s.saveLocked()
	}
	return removed
}

// ClearAllLockouts clears all brute-force lockout entries (used after verified password reset).
func (s *Store) ClearAllLockouts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.lockouts) > 0 {
		s.lockouts = make(map[string]*LockoutEntry)
		_ = s.saveLocked()
	}
}

var seedBcryptCache sync.Map

func hashSeedPassword(plain string) (string, error) {
	digest := sha256.Sum256([]byte(plain))
	if cached, ok := seedBcryptCache.Load(digest); ok {
		if s, ok := cached.(string); ok && s != "" {
			return s, nil
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	res := string(hash)
	seedBcryptCache.Store(digest, res)
	return res, nil
}

// SeedDefaultUsers seeds the admin and viewer accounts using one-way bcrypt password hashes
// without ever persisting plaintext passwords to disk.
func (s *Store) SeedDefaultUsers(adminUser, adminPass, adminEmail, viewerUser, viewerPass string) {
	adminUser = strings.TrimSpace(adminUser)
	if adminUser == "" {
		adminUser = "admin"
	}
	if adminPass == "" {
		adminPass = "inut2026"
	}
	adminEmail = strings.TrimSpace(adminEmail)
	if adminEmail == "" {
		adminEmail = DefaultAdminEmail
	}
	viewerUser = strings.TrimSpace(viewerUser)
	if viewerUser == "" {
		viewerUser = "viewer"
	}
	if viewerPass == "" {
		viewerPass = "viewer2026"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.users == nil {
		s.users = make(map[string]*UserAccount)
	}
	if s.resetOTPs == nil {
		s.resetOTPs = make(map[string]*PasswordResetOTP)
	}

	now := time.Now().UTC()
	changed := false

	if existing, ok := s.users[adminUser]; !ok || existing == nil || existing.PasswordHash == "" {
		hash, err := hashSeedPassword(adminPass)
		if err == nil {
			s.users[adminUser] = &UserAccount{
				Username:     adminUser,
				Email:        adminEmail,
				Role:         "admin",
				PasswordHash: hash,
				UpdatedAt:    now,
			}
			changed = true
		}
	} else if existing.Email == "" && adminEmail != "" {
		existing.Email = adminEmail
		existing.UpdatedAt = now
		changed = true
	}

	if existing, ok := s.users[viewerUser]; !ok || existing == nil || existing.PasswordHash == "" {
		hash, err := hashSeedPassword(viewerPass)
		if err == nil {
			s.users[viewerUser] = &UserAccount{
				Username:     viewerUser,
				Email:        "",
				Role:         "viewer",
				PasswordHash: hash,
				UpdatedAt:    now,
			}
			changed = true
		}
	}

	if changed {
		_ = s.saveLocked()
	}
}

func (s *Store) findUserLocked(identifier string) (*UserAccount, bool) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, false
	}
	if u, ok := s.users[identifier]; ok && u != nil {
		return u, true
	}
	for _, u := range s.users {
		if u == nil {
			continue
		}
		if strings.EqualFold(u.Username, identifier) || (u.Email != "" && strings.EqualFold(u.Email, identifier)) {
			return u, true
		}
	}
	return nil, false
}

// FindUser looks up a UserAccount by username or email (case-insensitive).
func (s *Store) FindUser(identifier string) (*UserAccount, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.findUserLocked(identifier)
	if !ok {
		return nil, false
	}
	cp := *u
	return &cp, true
}

// AuthenticateUser verifies a username/email and password against the stored bcrypt hash.
func (s *Store) AuthenticateUser(identifier, password string) (*UserAccount, bool) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || password == "" {
		return nil, false
	}
	u, ok := s.FindUser(identifier)
	if !ok || u.PasswordHash == "" {
		return nil, false
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, false
	}
	return u, true
}

// UpdateUserPassword validates and updates a user's bcrypt password hash, clearing any active reset OTP.
func (s *Store) UpdateUserPassword(username, newPassword string) error {
	if len(newPassword) < 6 {
		return fmt.Errorf("mật khẩu mới phải có ít nhất 6 ký tự")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.findUserLocked(username)
	if !ok {
		return fmt.Errorf("tài khoản %q không tồn tại", username)
	}
	u.PasswordHash = string(hash)
	u.UpdatedAt = time.Now().UTC()
	delete(s.resetOTPs, u.Username)
	return s.saveLocked()
}

// CreatePasswordResetOTP generates a cryptographic 6-digit OTP (100000-999999) and persists ONLY its SHA-256 hash.
func (s *Store) CreatePasswordResetOTP(username, email string, ttl time.Duration) (string, time.Time, error) {
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate otp: %w", err)
	}
	otpCode := fmt.Sprintf("%06d", n.Int64()+100000)
	sum := sha256.Sum256([]byte(otpCode))
	otpHash := hex.EncodeToString(sum[:])

	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resetOTPs == nil {
		s.resetOTPs = make(map[string]*PasswordResetOTP)
	}
	if u, ok := s.findUserLocked(username); ok {
		username = u.Username
		if email == "" {
			email = u.Email
		}
	} else if u, ok := s.findUserLocked(email); ok {
		if username == "" {
			username = u.Username
		}
	}

	s.resetOTPs[username] = &PasswordResetOTP{
		Username:  username,
		Email:     email,
		OTPHash:   otpHash,
		ExpiresAt: expiresAt,
		Attempts:  0,
		CreatedAt: now,
	}
	if err := s.saveLocked(); err != nil {
		return "", time.Time{}, err
	}
	return otpCode, expiresAt, nil
}

// VerifyAndConsumePasswordResetOTP verifies the 6-digit OTP against the stored SHA-256 hash,
// enforcing expiry and a maximum of 5 failed attempts before invalidation.
func (s *Store) VerifyAndConsumePasswordResetOTP(identifier, otpCode string) (*UserAccount, error) {
	identifier = strings.TrimSpace(identifier)
	otpCode = strings.TrimSpace(otpCode)
	if identifier == "" || otpCode == "" {
		return nil, fmt.Errorf("thiếu thông tin tài khoản hoặc mã OTP")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.findUserLocked(identifier)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy tài khoản phù hợp")
	}
	rec, ok := s.resetOTPs[u.Username]
	if !ok || rec == nil {
		return nil, fmt.Errorf("mã OTP không tồn tại hoặc đã được sử dụng")
	}

	now := time.Now().UTC()
	if now.After(rec.ExpiresAt) {
		delete(s.resetOTPs, u.Username)
		_ = s.saveLocked()
		return nil, fmt.Errorf("mã OTP đã hết hạn")
	}
	if rec.Attempts >= 5 {
		delete(s.resetOTPs, u.Username)
		_ = s.saveLocked()
		return nil, fmt.Errorf("mã OTP đã bị hủy do nhập sai quá 5 lần")
	}

	sum := sha256.Sum256([]byte(otpCode))
	candidateHash := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(rec.OTPHash), []byte(candidateHash)) != 1 {
		rec.Attempts++
		if rec.Attempts >= 5 {
			delete(s.resetOTPs, u.Username)
		}
		_ = s.saveLocked()
		return nil, fmt.Errorf("mã OTP không chính xác")
	}

	delete(s.resetOTPs, u.Username)
	_ = s.saveLocked()
	cp := *u
	return &cp, nil
}

// ComputeKPI calculates the FleetKPI summary across all clubs, boxes, and cameras.
func (s *Store) ComputeKPI() FleetKPI {
	s.mu.RLock()
	defer s.mu.RUnlock()
	kpi := FleetKPI{
		ClubsTotal: len(s.clubs),
		BoxesTotal: len(s.boxes),
	}
	clubBoxCounts := make(map[string]int, len(s.clubs))
	for _, b := range s.boxes {
		clubBoxCounts[b.ClubID]++
	}
	for _, c := range s.clubs {
		cnt := clubBoxCounts[c.ID]
		if cnt == 0 {
			cnt = len(c.BoxIDs)
		}
		if cnt > 1 {
			kpi.MultiBoxClubs++
		}
	}
	totalHardening := 0
	totalRTT := 0
	rttCount := 0
	domainOwners := make(map[string]int)
	for _, b := range s.boxes {
		if b.StoreDomain != "" {
			domainOwners[strings.ToLower(b.StoreDomain)]++
		}
		if b.CStoreDomain != "" {
			domainOwners[strings.ToLower(b.CStoreDomain)]++
		}
	}
	for _, b := range s.boxes {
		if b.Online {
			kpi.BoxesOnline++
			if b.RTTMs > 0 {
				totalRTT += b.RTTMs
				rttCount++
			}
		} else {
			kpi.BoxesOffline++
		}
		kpi.CamerasTotal += len(b.Cameras)
		for _, cam := range b.Cameras {
			if cam.Online {
				kpi.CamerasOnline++
			}
			if cam.DoubleStreaming {
				kpi.DoubleStreamCount++
			}
		}
		kpi.FFmpegTotal += b.FFmpegCount
		kpi.FFmpegExpected += b.FFmpegExpected
		totalHardening += b.HardeningScore
		if b.HardeningScore >= 8 {
			kpi.HardeningFull++
		}
		if b.SDUsedPct >= 85 {
			kpi.SDWarningCount++
		}
		conflict := domainOwners[strings.ToLower(b.StoreDomain)] > 1 || domainOwners[strings.ToLower(b.CStoreDomain)] > 1
		if conflict {
			kpi.DomainsConflict++
		} else if b.StoreHTTPCode == 200 && b.CStoreHTTPCode == 200 {
			kpi.DomainsOK++
		}
	}
	if len(s.boxes) > 0 {
		kpi.HardeningAvg = float64(totalHardening*10) / float64(len(s.boxes)) / 10.0
	}
	if rttCount > 0 {
		kpi.AvgRTTMs = totalRTT / rttCount
	}
	return kpi
}

func cloneBox(b *Box) *Box {
	cp := *b
	cp.RTTHistory = append([]int(nil), b.RTTHistory...)
	if b.RedBidaKeys != nil {
		cp.RedBidaKeys = make(map[string]string, len(b.RedBidaKeys))
		for k, v := range b.RedBidaKeys {
			cp.RedBidaKeys[k] = v
		}
	}
	if b.RedBidaTabs != nil {
		cp.RedBidaTabs = make(map[string]string, len(b.RedBidaTabs))
		for k, v := range b.RedBidaTabs {
			cp.RedBidaTabs[k] = v
		}
	}
	if b.Cameras != nil {
		cp.Cameras = make([]*Camera, len(b.Cameras))
		for i, cam := range b.Cameras {
			camCopy := *cam
			cp.Cameras[i] = &camCopy
		}
	}
	if b.Monitors != nil {
		cp.Monitors = make([]*ShinobiMonitor, len(b.Monitors))
		for i, mon := range b.Monitors {
			monCopy := *mon
			cp.Monitors[i] = &monCopy
		}
	}
	return &cp
}
