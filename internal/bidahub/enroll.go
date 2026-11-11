package bidahub

import (
	"crypto/subtle"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var reFleetBoxID = regexp.MustCompile(`^inut_(\d{1,3})_(\d{1,3})$`)

// validBoxSecret reports whether secret is the shared fleet key the hub accepts for
// self-enrollment (the same key Ansible installs into every inut-bida-agent).
func (s *Server) validBoxSecret(secret string) bool {
	secret = strings.TrimSpace(secret)
	return secret != "" && s.cfg.APIKey != "" && subtle.ConstantTimeCompare([]byte(secret), []byte(s.cfg.APIKey)) == 1
}

// admitBox returns true when boxID is known, auto-enrolling it first when the box presents
// the fleet key. Unknown boxes with any other secret are queued for manual approval.
func (s *Server) admitBox(boxID, secret, remoteIP string, p HeartbeatPayload) (bool, *EnrollmentRequest) {
	if _, ok := s.store.GetBox(boxID); ok {
		return true, nil
	}
	if s.validBoxSecret(secret) {
		p.BoxID = boxID
		if box, created := s.store.AutoEnrollBox(p, remoteIP); box != nil {
			if created {
				s.broadcastSSE("fleet_update", map[string]any{
					"type":   "box_auto_enrolled",
					"box_id": box.ID,
					"kpi":    s.store.ComputeKPI(),
				})
			}
			return true, nil
		}
	}
	return false, s.store.RecordEnrollment(boxID, secret, remoteIP, p.SoCModel)
}

// AutoEnrollBox creates a Box for p.BoxID from the identity the agent reported (RedBida keys,
// LAN IP, arch). The club is derived from ui_title so boxes of one venue share a club.
// Returns the existing box unchanged (created=false) when it is already enrolled, and nil when
// the payload carries no ui_title (older agents) so no placeholder record is ever created.
func (s *Store) AutoEnrollBox(p HeartbeatPayload, remoteIP string) (*Box, bool) {
	boxID := strings.TrimSpace(p.BoxID)
	if boxID == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.boxByID[boxID]; ok {
		return cloneBox(b), false
	}

	title := strings.TrimSpace(p.RedBidaKeys["ui_title"])
	if title == "" {
		return nil, false
	}
	now := time.Now().UTC()
	branding := deriveBranding(p.RedBidaKeys, "")
	clubID := "club_" + slugify(title)
	club, ok := s.clubByID[clubID]
	if !ok {
		portalToken := randomHex(16, "portal:"+clubID)
		club = &Club{
			ID:          clubID,
			Name:        title,
			Branding:    branding,
			PortalToken: portalToken,
			DLHMACKey:   randomHex(32, "hmac:"+clubID),
			PortalURL:   "/portal/" + portalToken + "/",
			CreatedAt:   now,
		}
		s.clubs = append(s.clubs, club)
	}

	box := &Box{
		ID:           boxID,
		ClubID:       club.ID,
		ClubName:     club.Name,
		Name:         title,
		VPNIP:        fleetVPNIP(boxID),
		SoCModel:     p.SoCModel,
		Branding:     branding,
		PortalToken:  club.PortalToken,
		HardeningMax: 8,
		RedBidaMax:   17,
		Transport:    "tunnel",
		Notes:        fmt.Sprintf("auto-enrolled by inut-bida-agent from %s at %s", remoteIP, now.Format(time.RFC3339)),
	}
	applyHeartbeatToBox(box, p, "live")
	s.boxes = append(s.boxes, box)

	if req, ok := s.enrollments[boxID]; ok {
		req.Status = "approved"
	}
	s.rebuildIndexesLocked()
	_ = s.saveLocked()
	return cloneBox(box), true
}

// applyBoxIdentity overwrites hub-side identity with what the box itself reports, so RedBida
// data in the hub always mirrors /root/ota-mqtt/change_ok instead of seeded placeholders.
// Club membership is left untouched (it carries the owner portal token).
func applyBoxIdentity(b *Box, p HeartbeatPayload) {
	if p.LANIP != "" {
		b.LANIP = p.LANIP
	}
	if p.Arch != "" {
		b.Arch = p.Arch
	}
	keys := p.RedBidaKeys
	if len(keys) == 0 {
		return
	}
	b.RedBidaKeys = make(map[string]string, len(keys))
	for k, v := range keys {
		b.RedBidaKeys[k] = v
	}
	if gk := firstNonEmpty(keys["shinobi_camera_id"], keys["shinobi_group_key"]); gk != "" {
		b.ShinobiGroupKey = gk
		b.RedBidaKeys["shinobi_group_key"] = gk
	}
	if v := strings.TrimSpace(keys["ggcode"]); v != "" {
		b.GGCode = v
	}
	if vip := firstNonEmpty(keys["eth0_virtual_ip"], keys["end0_virtual_ip"]); vip != "" {
		b.VirtualIP = vip
	}
	if n, err := strconv.Atoi(strings.TrimSpace(keys["camera_count"])); err == nil && n >= 0 {
		b.CamerasCount = n
	}
	if frpc, ok := keys["frpc_config"]; ok {
		b.FRPCConfig = frpc
		storeDom, cstoreDom := extractDomainsFromFRPC(frpc)
		if storeDom != "" {
			b.StoreDomain = storeDom
		}
		if cstoreDom != "" {
			b.CStoreDomain = cstoreDom
		}
	}
	b.Branding = deriveBranding(keys, b.Branding)
}

// deriveBranding classifies a venue as pickleball/sport or billiards from its own RedBida
// branding. custom_hashtags is ignored: the billiards standard also carries #highlightsports.
func deriveBranding(keys map[string]string, fallback string) string {
	logo := strings.ToLower(keys["logo_header"] + " " + keys["logo_header_text"])
	title := strings.ToLower(keys["ui_title"])
	switch {
	case strings.Contains(logo, "highlightsports"), strings.Contains(logo, "highlight sport"), strings.Contains(title, "pickleball"):
		return "pickleball"
	case strings.Contains(logo, "bidalive"), strings.Contains(logo, "billiard"), strings.Contains(title, "billiard"), strings.Contains(title, "bida"):
		return "billiards"
	case fallback != "":
		return fallback
	default:
		return "billiards"
	}
}

// fleetVPNIP maps the fleet box naming convention inut_A_B to its tinc address 77.88.A.B.
func fleetVPNIP(boxID string) string {
	m := reFleetBoxID.FindStringSubmatch(boxID)
	if m == nil {
		return ""
	}
	return "77.88." + m[1] + "." + m[2]
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}
