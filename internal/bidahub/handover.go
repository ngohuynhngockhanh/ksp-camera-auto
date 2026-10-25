package bidahub

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func signClipPayload(club *Club, boxID, cameraID, clipPath string, expiresAt time.Time) (string, error) {
	if club == nil || club.DLHMACKey == "" {
		return "", fmt.Errorf("club missing persistent dl_hmac_key")
	}
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(365 * 24 * time.Hour)
	}
	claims := ClipClaims{
		ClubID:    club.ID,
		BoxID:     boxID,
		CameraID:  cameraID,
		ClipPath:  clipPath,
		ExpiresAt: expiresAt.Unix(),
	}
	rawJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(rawJSON)
	mac := hmac.New(sha256.New, []byte(club.DLHMACKey))
	_, _ = mac.Write([]byte(payloadB64))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payloadB64 + "." + sig, nil
}

// SignClipToken signs a persistent clip token using the club's persisted DLHMACKey.
func (s *Server) SignClipToken(clubID, boxID, cameraID, clipPath string, expiresAt time.Time) (string, error) {
	var club *Club
	if clubID != "" {
		if c, ok := s.store.GetClub(clubID); ok {
			club = c
		}
	}
	if club == nil && boxID != "" {
		if b, ok := s.store.GetBox(boxID); ok {
			if c, ok := s.store.GetClub(b.ClubID); ok {
				club = c
			}
		}
	}
	if club == nil && clubID != "" {
		// Allow passing boxID in the first parameter for convenience
		if b, ok := s.store.GetBox(clubID); ok {
			if bID := b.ID; boxID == "" {
				boxID = bID
			}
			if c, ok := s.store.GetClub(b.ClubID); ok {
				club = c
			}
		}
	}
	if club == nil {
		return "", fmt.Errorf("club not found for clip signing (club=%q, box=%q)", clubID, boxID)
	}
	if boxID == "" && len(club.BoxIDs) > 0 {
		boxID = club.BoxIDs[0]
	}
	if cameraID == "" {
		cameraID = "camera01"
	}
	if clipPath == "" {
		clipPath = fmt.Sprintf("/media/usb1/%s/%s/highlight.mp4", club.ID, cameraID)
	}
	return signClipPayload(club, boxID, cameraID, clipPath, expiresAt)
}

// VerifyClipToken validates a signed clip token against the club's persisted DLHMACKey.
func (s *Server) VerifyClipToken(token string) (*ClipClaims, error) {
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("malformed clip token")
	}
	rawJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid clip token base64: %w", err)
	}
	var claims ClipClaims
	if err := json.Unmarshal(rawJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid clip token claims: %w", err)
	}
	if claims.ClubID == "" {
		return nil, fmt.Errorf("clip token missing club_id")
	}
	if claims.ExpiresAt > 0 && time.Now().UTC().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("clip token expired")
	}

	club, ok := s.store.GetClub(claims.ClubID)
	if !ok || club.DLHMACKey == "" {
		return nil, fmt.Errorf("unknown club %q", claims.ClubID)
	}

	mac := hmac.New(sha256.New, []byte(club.DLHMACKey))
	_, _ = mac.Write([]byte(parts[0]))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(parts[1])), []byte(expectedSig)) != 1 {
		return nil, fmt.Errorf("clip token HMAC signature mismatch")
	}
	return &claims, nil
}

// BuildHandoverPack generates the 1-Click Zalo/Telegram Handover Pack and inline SVG QRs for a Club.
func BuildHandoverPack(club *Club, boxes []*Box, baseURL string) *HandoverPack {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://inut-bida.p2p.inut.io.vn"
	}

	storeURL := ""
	cstoreURL := ""
	boxIDs := make([]string, 0, len(boxes))
	camTotal := 0
	var boxLines []string
	firstBoxID := ""

	for i, b := range boxes {
		if firstBoxID == "" {
			firstBoxID = b.ID
		}
		boxIDs = append(boxIDs, b.ID)
		camTotal += len(b.Cameras)
		if storeURL == "" && b.StoreDomain != "" {
			storeURL = "https://" + b.StoreDomain
		}
		if cstoreURL == "" && b.CStoreDomain != "" {
			cstoreURL = "https://" + b.CStoreDomain
		}
		boxLines = append(boxLines, fmt.Sprintf("  • Box %d (%s - %s): %d camera | Store: https://%s | C-Store: https://%s",
			i+1, b.Name, b.ID, len(b.Cameras), b.StoreDomain, b.CStoreDomain))
	}
	if storeURL == "" {
		storeURL = club.StoreURL
	}
	if cstoreURL == "" {
		cstoreURL = club.CStoreURL
	}

	portalPath := "/portal/" + club.PortalToken + "/"
	portalFullURL := baseURL + portalPath
	clipTok, _ := signClipPayload(club, firstBoxID, "camera01", fmt.Sprintf("/media/usb1/%s/camera01/sample.mp4", club.ID), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	clipPath := "/clip/" + clipTok
	clipFullURL := baseURL + clipPath

	sportLabel := "CLB Bida"
	if strings.EqualFold(club.Branding, "pickleball") {
		sportLabel = "CLB Pickleball"
	}

	zaloTemplate := fmt.Sprintf(
		"🎱 BÀN GIAO HỆ THỐNG CAMERA HIGHLIGHT & TUA LẠI — %s (%s)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🏢 Tên cơ sở: %s\n"+
			"📦 Số lượng thiết bị: %d Box (%d Camera H.265 30fps)\n"+
			"%s\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🔗 1. Trang Khách Xem & Cắt Highlight (Store):\n   %s\n\n"+
			"🎥 2. Trang Quản Lý Luồng Ghi Hình (C-Store):\n   %s\n\n"+
			"📱 3. Cổng Chủ Quán Đa Thuê Bao (Read-Only Portal):\n   %s\n\n"+
			"🎬 4. Link Tải Clip QR Mẫu (Chữ ký bền vững HMAC):\n   %s\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"✅ Đã kích hoạt chuẩn Golden Hardening 8 bước (ZRAM 100%%, PM2 Memory Guard, Watchdog 60s, Storage Guard 90%%, Fragmented MP4 chống lỗi mất điện).",
		club.Name, sportLabel, club.Name, len(boxes), camTotal,
		strings.Join(boxLines, "\n"),
		storeURL, cstoreURL, portalFullURL, clipFullURL,
	)

	return &HandoverPack{
		ID:           club.ID,
		ClubID:       club.ID,
		ClubName:     club.Name,
		Branding:     club.Branding,
		BoxIDs:       boxIDs,
		BoxesCount:   len(boxes),
		CamerasCount: camTotal,
		StoreURL:     storeURL,
		CStoreURL:    cstoreURL,
		PortalToken:  club.PortalToken,
		PortalURL:    portalPath,
		ClipToken:    clipTok,
		ClipURL:      clipPath,
		ZaloTemplate: zaloTemplate,
		QRStoreSVG:   GenerateQRSVG(storeURL),
		QRCStoreSVG:  GenerateQRSVG(cstoreURL),
		QRPortalSVG:  GenerateQRSVG(portalFullURL),
		QRClipSVG:    GenerateQRSVG(clipFullURL),
	}
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "127.0.0.1:2090"
	}
	return scheme + "://" + host
}

func (s *Server) handleHandoverList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	baseURL := requestBaseURL(r)
	clubs := s.store.Clubs()
	packs := make([]*HandoverPack, 0, len(clubs))
	for _, c := range clubs {
		boxes := s.store.GetBoxesByClub(c.ID)
		packs = append(packs, BuildHandoverPack(c, boxes, baseURL))
	}
	var firstPack *HandoverPack
	if len(packs) > 0 {
		firstPack = packs[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"packs": packs,
		"pack":  firstPack,
	})
}

func (s *Server) handleHandoverSingle(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	baseURL := requestBaseURL(r)

	var club *Club
	if c, ok := s.store.GetClub(id); ok {
		club = c
	} else if b, ok := s.store.GetBox(id); ok {
		if c, ok := s.store.GetClub(b.ClubID); ok {
			club = c
		}
	} else if c, ok := s.store.GetClubByPortalToken(id); ok {
		club = c
	}

	if club == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "handover_target_not_found",
			"id":    id,
		})
		return
	}

	boxes := s.store.GetBoxesByClub(club.ID)
	pack := BuildHandoverPack(club, boxes, baseURL)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"id":            pack.ID,
		"club_id":       pack.ClubID,
		"club_name":     pack.ClubName,
		"branding":      pack.Branding,
		"box_ids":       pack.BoxIDs,
		"boxes_count":   pack.BoxesCount,
		"cameras_count": pack.CamerasCount,
		"store_url":     pack.StoreURL,
		"cstore_url":    pack.CStoreURL,
		"portal_token":  pack.PortalToken,
		"portal_url":    pack.PortalURL,
		"clip_token":    pack.ClipToken,
		"clip_url":      pack.ClipURL,
		"zalo_template": pack.ZaloTemplate,
		"qr_store_svg":  pack.QRStoreSVG,
		"qr_cstore_svg": pack.QRCStoreSVG,
		"qr_portal_svg": pack.QRPortalSVG,
		"qr_clip_svg":   pack.QRClipSVG,
		"pack":          pack,
	})
}

func (s *Server) handleClipSign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	var req struct {
		ClubID   string `json:"club_id"`
		BoxID    string `json:"box_id"`
		CameraID string `json:"camera_id"`
		ClipPath string `json:"clip_path"`
		TTLHours int    `json:"ttl_hours"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	ttl := 720 * time.Hour
	if req.TTLHours > 0 {
		ttl = time.Duration(req.TTLHours) * time.Hour
	}
	tok, err := s.SignClipToken(req.ClubID, req.BoxID, req.CameraID, req.ClipPath, time.Now().UTC().Add(ttl))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	clipURL := "/clip/" + tok
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"clip_token":  tok,
		"clip_url":    clipURL,
		"qr_clip_svg": GenerateQRSVG(requestBaseURL(r) + clipURL),
	})
}

func (s *Server) handleClipVerify(w http.ResponseWriter, r *http.Request, clipToken string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	claims, err := s.VerifyClipToken(clipToken)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"ok":      false,
			"valid":   false,
			"error":   "invalid_clip_token",
			"message": err.Error(),
		})
		return
	}
	club, _ := s.store.GetClub(claims.ClubID)
	clubName := claims.ClubID
	if club != nil {
		clubName = club.Name
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"valid":      true,
		"club_id":    claims.ClubID,
		"club_name":  clubName,
		"box_id":     claims.BoxID,
		"camera_id":  claims.CameraID,
		"clip_path":  claims.ClipPath,
		"expires_at": claims.ExpiresAt,
		"claims":     claims,
	})
}
