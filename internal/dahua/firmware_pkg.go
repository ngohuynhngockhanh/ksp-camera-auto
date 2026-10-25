package dahua

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	// ErrInvalidFirmwareMagic is returned when the firmware package starts with neither "DH\x03\x04" nor "PK\x03\x04".
	ErrInvalidFirmwareMagic = errors.New("dahua firmware: invalid package header magic (expected DH or PK zip archive)")
	// ErrCorruptFirmwarePackage is returned when zip archive structure or member CRC32 verification fails.
	ErrCorruptFirmwarePackage = errors.New("dahua firmware: corrupt package archive or CRC mismatch")
	// ErrHWIDMismatch is returned when a target camera's hardware ID (upgrader.getUpdateSerial) is not listed in the firmware package.
	ErrHWIDMismatch = errors.New("dahua firmware: target hardware ID does not match package HWID list")
)

// FirmwareMemberInfo summarizes one file inside the firmware package.
type FirmwareMemberInfo struct {
	Name  string `json:"name"`
	Size  uint64 `json:"size"`
	CRC32 string `json:"crc32"`
}

// FirmwarePackageInfo contains the parsed metadata and safety flags of a Dahua/Imou/KBVision firmware package.
type FirmwarePackageInfo struct {
	SHA256                  string               `json:"sha256"`
	SizeBytes               int                  `json:"sizeBytes"`
	MagicHeader             string               `json:"magicHeader"` // "DH" or "PK"
	HWIDs                   []string             `json:"hwids"`
	Models                  []string             `json:"models"`
	Vendor                  string               `json:"vendor,omitempty"`
	ProductVersion          string               `json:"productVersion,omitempty"`
	Build                   string               `json:"build,omitempty"`
	BuildDateTime           string               `json:"buildDateTime,omitempty"`
	SecurityBaselineVersion string               `json:"securityBaselineVersion,omitempty"`
	InstallCommands         []string             `json:"installCommands,omitempty"`
	TouchesBootloader       bool                 `json:"touchesBootloader"`
	RestoresDefault         bool                 `json:"restoresDefault"`
	HasSignature            bool                 `json:"hasSignature"`
	SignatureBytes          uint64               `json:"signatureBytes"`
	Members                 []FirmwareMemberInfo `json:"members"`
}

// SupportsHWID checks whether a camera's hardware ID string (from upgrader.getUpdateSerial)
// matches any HWID entry in the package. If the package carries an explicit HWID list,
// an exact match or model prefix match before the first colon is checked.
func (info *FirmwarePackageInfo) SupportsHWID(deviceHWID string) bool {
	deviceHWID = strings.TrimSpace(deviceHWID)
	if deviceHWID == "" || len(info.HWIDs) == 0 {
		return false
	}
	for _, h := range info.HWIDs {
		if strings.TrimSpace(h) == deviceHWID {
			return true
		}
	}
	return false
}

// InspectFirmwarePackage parses a raw Dahua/KBVision/Imou firmware package (.bin / .pkg),
// normalizes the "DH\x03\x04" header to "PK\x03\x04" in memory, verifies the CRC32 of every
// archive member, and extracts HWID compatibility and install-script risk flags.
func InspectFirmwarePackage(data []byte) (*FirmwarePackageInfo, error) {
	if len(data) < 4 {
		return nil, ErrInvalidFirmwareMagic
	}
	var magic string
	switch {
	case data[0] == 'D' && data[1] == 'H' && data[2] == 0x03 && data[3] == 0x04:
		magic = "DH"
	case data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04:
		magic = "PK"
	default:
		return nil, ErrInvalidFirmwareMagic
	}

	sum := sha256.Sum256(data)
	normalized := make([]byte, len(data))
	copy(normalized, data)
	normalized[0] = 'P'
	normalized[1] = 'K'

	zr, err := zip.NewReader(bytes.NewReader(normalized), int64(len(normalized)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptFirmwarePackage, err)
	}

	info := &FirmwarePackageInfo{
		SHA256:      hex.EncodeToString(sum[:]),
		SizeBytes:   len(data),
		MagicHeader: magic,
		Members:     make([]FirmwareMemberInfo, 0, len(zr.File)),
	}

	hwidSet := make(map[string]struct{})
	modelSet := make(map[string]struct{})
	addHWID := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" {
			return
		}
		if _, exists := hwidSet[h]; !exists {
			hwidSet[h] = struct{}{}
			info.HWIDs = append(info.HWIDs, h)
		}
		if idx := strings.IndexByte(h, ':'); idx > 0 {
			m := strings.TrimSpace(h[:idx])
			if m != "" {
				if _, exists := modelSet[m]; !exists {
					modelSet[m] = struct{}{}
					info.Models = append(info.Models, m)
				}
			}
		}
	}

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("%w: open %s: %v", ErrCorruptFirmwarePackage, f.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("%w: read %s (CRC check): %v", ErrCorruptFirmwarePackage, f.Name, err)
		}

		info.Members = append(info.Members, FirmwareMemberInfo{
			Name:  f.Name,
			Size:  f.UncompressedSize64,
			CRC32: fmt.Sprintf("%08x", f.CRC32),
		})

		baseName := strings.ToLower(f.Name)
		switch {
		case baseName == "sign.img":
			info.HasSignature = f.UncompressedSize64 > 0
			info.SignatureBytes = f.UncompressedSize64
		case baseName == "hwid":
			parseHWIDContent(content, addHWID, info)
		case baseName == "check.img":
			// check.img may be a nested DH/PK zip containing "hwid" or JSON directly.
			parseCheckImg(content, addHWID, info)
		case baseName == "install":
			parseInstallManifest(content, info)
		}
	}

	return info, nil
}

func parseCheckImg(content []byte, addHWID func(string), info *FirmwarePackageInfo) {
	if len(content) >= 4 && ((content[0] == 'D' && content[1] == 'H') || (content[0] == 'P' && content[1] == 'K')) && content[2] == 0x03 && content[3] == 0x04 {
		nested := make([]byte, len(content))
		copy(nested, content)
		nested[0] = 'P'
		nested[1] = 'K'
		if zr, err := zip.NewReader(bytes.NewReader(nested), int64(len(nested))); err == nil {
			for _, f := range zr.File {
				if rc, err := f.Open(); err == nil {
					b, err := io.ReadAll(rc)
					_ = rc.Close()
					if err == nil {
						parseHWIDContent(b, addHWID, info)
					}
				}
			}
			return
		}
	}
	parseHWIDContent(content, addHWID, info)
}

func extractJSONObjectBytes(b []byte) []byte {
	start := bytes.IndexByte(b, '{')
	end := bytes.LastIndexByte(b, '}')
	if start >= 0 && end > start {
		return b[start : end+1]
	}
	return b
}

func parseHWIDContent(content []byte, addHWID func(string), info *FirmwarePackageInfo) {
	trimmed := bytesTrimNull(bytes.TrimSpace(content))
	if len(trimmed) == 0 {
		return
	}
	jsonSlice := extractJSONObjectBytes(trimmed)
	var doc map[string]any
	if err := json.Unmarshal(jsonSlice, &doc); err == nil {
		if v, ok := doc["Vendor"].(string); ok && info.Vendor == "" {
			info.Vendor = strings.TrimSpace(v)
		}
		if v, ok := doc["Build"].(string); ok && info.Build == "" {
			info.Build = strings.TrimSpace(v)
		}
		if v, ok := doc["BuildDateTime"].(string); ok && info.BuildDateTime == "" {
			info.BuildDateTime = strings.TrimSpace(v)
		}
		if v, ok := doc["SecurityBaselineVersion"].(string); ok && info.SecurityBaselineVersion == "" {
			info.SecurityBaselineVersion = strings.TrimSpace(v)
		}
		for _, key := range []string{"HardwareID", "hwid", "HWID"} {
			if arr, ok := doc[key].([]any); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok {
						addHWID(s)
					}
				}
			}
		}
		if arr, ok := doc["Devices"].([]any); ok {
			for _, item := range arr {
				if pair, ok := item.([]any); ok && len(pair) > 0 {
					if m, ok := pair[0].(string); ok && strings.TrimSpace(m) != "" {
						info.Models = appendUnique(info.Models, strings.TrimSpace(m))
					}
				}
			}
		}
		return
	}
	// Fallback: line-delimited HWID entries
	for _, line := range strings.Split(string(trimmed), "\n") {
		line = strings.Trim(strings.TrimSpace(line), `",`)
		if strings.Contains(line, ":") && !strings.HasPrefix(line, "{") && !strings.HasPrefix(line, "//") && !strings.Contains(line, `"`) {
			addHWID(line)
		}
	}
}

func parseInstallManifest(content []byte, info *FirmwarePackageInfo) {
	trimmed := bytesTrimNull(bytes.TrimSpace(content))
	if len(trimmed) == 0 {
		return
	}
	// Install can be JSON with "Commands" or line-oriented script with comments.
	var doc struct {
		Commands []string   `json:"Commands"`
		Devices  [][]string `json:"Devices"`
		Vendor   string     `json:"Vendor"`
	}
	if err := json.Unmarshal(extractJSONObjectBytes(trimmed), &doc); err == nil && len(doc.Commands) > 0 {
		info.InstallCommands = append(info.InstallCommands, doc.Commands...)
		if doc.Vendor != "" && info.Vendor == "" {
			info.Vendor = doc.Vendor
		}
		for _, pair := range doc.Devices {
			if len(pair) > 0 && strings.TrimSpace(pair[0]) != "" {
				info.Models = appendUnique(info.Models, strings.TrimSpace(pair[0]))
			}
		}
	}
	text := string(trimmed)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "bootloader") || strings.Contains(lower, "dhboot") || strings.Contains(lower, "u-boot") {
			info.TouchesBootloader = true
		}
		if strings.Contains(lower, "restoredefault") || strings.Contains(lower, "restore_default") {
			info.RestoresDefault = true
		}
		if strings.HasPrefix(line, "//Product-version:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "//Product-version:"))
			if v != "" && info.ProductVersion == "" {
				info.ProductVersion = v
			}
		}
	}
	for _, cmd := range info.InstallCommands {
		lower := strings.ToLower(cmd)
		if strings.Contains(lower, "bootloader") || strings.Contains(lower, "dhboot") || strings.Contains(lower, "u-boot") {
			info.TouchesBootloader = true
		}
	}
}

func appendUnique(slice []string, val string) []string {
	for _, item := range slice {
		if item == val {
			return slice
		}
	}
	return append(slice, val)
}
