package xiongmai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Sentinel errors for clean error classification across bulk orchestrator,
// credential tester, discovery, and HTTP handlers.
var (
	ErrDialUnreachable      = errors.New("xiongmai: dial unreachable")
	ErrAuthFailed           = errors.New("xiongmai: authentication failed")
	ErrAccountLocked        = errors.New("xiongmai: account or IP temporarily locked")
	ErrSessionExpired       = errors.New("xiongmai: session expired")
	ErrRSAHandshakeRequired = errors.New("xiongmai: device requires RSA/CGI login (Ret 205)")
	ErrUnsupported          = errors.New("xiongmai: feature or config unsupported by firmware")
	ErrNoRecordings         = errors.New("xiongmai: no recordings found")
)

// Xiongmai Ret status codes (from Web/plugin/language.js:1632).
const (
	RetOK              = 100
	RetUnknownErr      = 101
	RetNoSupport       = 102
	RetInvalidReq      = 103
	RetAlreadyLoggedIn = 104
	RetNoLogin         = 105
	RetBadPassword     = 106
	RetNoPower         = 107
	RetTimeout         = 108
	RetNotFoundFile    = 109
	RetFindAll         = 110
	RetNoFindFile2     = 119
	RetJSONError       = 117
	RetEncryptError    = 124
	RetCGIFormatError  = 136
	RetBadPassword2    = 203
	RetRSARequired     = 205
	RetAccountLocked   = 206
	RetIPBanned        = 207
	RetNeedReboot      = 603
	RetConfigNotExist  = 609
)

// RetError wraps a non-100 Xiongmai Ret status code with human context and
// sentinel classification via errors.Is.
type RetError struct {
	Op     string
	Ret    int
	Detail string
}

func (e *RetError) Error() string {
	msg := retCodeDescription(e.Ret)
	if e.Detail != "" {
		msg = e.Detail
	}
	if e.Op != "" {
		return fmt.Sprintf("xiongmai %s: Ret=%d (%s)", e.Op, e.Ret, msg)
	}
	return fmt.Sprintf("xiongmai: Ret=%d (%s)", e.Ret, msg)
}

func (e *RetError) Is(target error) bool {
	switch target {
	case ErrAuthFailed:
		return e.Ret == RetBadPassword || e.Ret == RetBadPassword2 || e.Ret == RetNoPower
	case ErrAccountLocked:
		return e.Ret == RetAccountLocked || e.Ret == RetIPBanned
	case ErrSessionExpired:
		return e.Ret == RetNoLogin || e.Ret == RetCGIFormatError || e.Ret == RetEncryptError
	case ErrRSAHandshakeRequired:
		return e.Ret == RetRSARequired
	case ErrUnsupported:
		return e.Ret == RetNoSupport || e.Ret == RetConfigNotExist || e.Ret == RetInvalidReq
	case ErrNoRecordings:
		return e.Ret == RetNotFoundFile || e.Ret == RetNoFindFile2
	default:
		return false
	}
}

func retCodeDescription(ret int) string {
	switch ret {
	case RetOK:
		return "OK"
	case RetNeedReboot:
		return "saved (reboot required)"
	case RetNoSupport:
		return "not supported by firmware"
	case RetInvalidReq:
		return "invalid request"
	case RetNoLogin:
		return "not logged in / session expired"
	case RetBadPassword, RetBadPassword2:
		return "invalid username or password"
	case RetNoPower:
		return "insufficient account permission"
	case RetNotFoundFile, RetNoFindFile2:
		return "no recording files in range"
	case RetEncryptError:
		return "RSA/AES encryption mismatch"
	case RetCGIFormatError:
		return "CGI format / expired salt"
	case RetRSARequired:
		return "RSA handshake required"
	case RetAccountLocked:
		return "account temporarily locked"
	case RetIPBanned:
		return "IP blocked by device filter"
	case RetConfigNotExist:
		return "configuration table does not exist"
	default:
		return "device error"
	}
}

// IsRetSuccess reports whether ret indicates a successful read/write.
func IsRetSuccess(ret int) bool {
	return ret == RetOK || ret == RetNeedReboot || ret == RetFindAll
}

// LoginInfo captures metadata returned during Sofia TCP or HTTP CGI login.
type LoginInfo struct {
	SessionIDHex  string
	SessionIDNum  uint32
	ChannelNum    int
	ExtraChannel  int
	DeviceType    string
	AliveInterval int
	DataUseAES    bool
	TransportName string
}

// RecordFile represents one segment returned by OPFileQuery.
type RecordFile struct {
	Channel    int
	FileName   string
	BeginTime  time.Time
	EndTime    time.Time
	SizeBytes  int64
	DiskNo     int
	SerialNo   int
	RawBegin   string
	RawEnd     string
	StreamType string
}

// Transport abstracts the underlying wire protocol for Xiongmai devices so
// domain operations (System, Encode, OSD, Storage, Network, Playback) work
// identically over Sofia TCP (:34567) and HTTP CGI + WebSocket (:80).
type Transport interface {
	Name() string
	Login(ctx context.Context) (*LoginInfo, error)
	LoginInfo() *LoginInfo
	GetInfo(ctx context.Context, name string) (map[string]any, error)
	GetAbility(ctx context.Context, name string) (map[string]any, error)
	GetChannelAbility(ctx context.Context, name string) (map[string]any, error)
	GetConfig(ctx context.Context, name string) (map[string]any, error)
	SetConfig(ctx context.Context, name string, payload any) (map[string]any, error)
	OpDev(ctx context.Context, name string, param any) (map[string]any, error)
	PTZ(ctx context.Context, param any) (map[string]any, error)
	ModifyPassword(ctx context.Context, user, oldPass, newPass string) error
	Snapshot(ctx context.Context, channel int) ([]byte, error)
	DownloadRecordFile(ctx context.Context, w io.Writer, channel int, file RecordFile, start, end time.Time) error
	Close() error
}

func extractRet(m map[string]any) int {
	if m == nil {
		return 0
	}
	switch v := m["Ret"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var n int
		_, _ = fmt.Sscanf(strings.TrimSpace(v), "%d", &n)
		return n
	default:
		return 0
	}
}

func parseSessionHex(s string) uint32 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if s == "" {
		return 0
	}
	var v uint32
	_, _ = fmt.Sscanf(s, "%x", &v)
	return v
}
