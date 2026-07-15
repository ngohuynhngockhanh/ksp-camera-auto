//go:build !hiksdk

package camera

import (
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/hik"
)

// openHikClient (default, pure-Go build) talks ISAPI over HTTP on the
// configured port. This reaches Hikvision devices that expose their HTTP/ISAPI
// interface (port 80) — i.e. when the tool runs on the camera LAN. Build with
// `-tags hiksdk` to instead reach devices over the proprietary port 8000 via
// the HCNetSDK backend.
func openHikClient(d config.Device, timeout time.Duration) (*hik.Client, error) {
	return hik.Dial(d.Host, d.Port, false, d.Username, d.Password, timeout), nil
}
