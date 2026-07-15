//go:build hiksdk

package camera

import (
	"fmt"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/hik"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/hiksdk"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// openHikClient (hiksdk build) logs into the device on its proprietary port
// (8000) via HCNetSDK and carries the exact same ISAPI XML over
// NET_DVR_STDXMLConfig. The SDK component-library path comes from the
// KSPCAM_HIKSDK_PATH environment variable (set to the dir containing
// libhcnetsdk.so + HCNetSDKCom/).
func openHikClient(d config.Device, timeout time.Duration) (*hik.Client, error) {
	sess, err := hiksdk.Open(d.Host, d.Port, d.Username, d.Password)
	if err != nil {
		return nil, fmt.Errorf("hik sdk login %s:%d: %w", d.Host, d.Port, err)
	}
	return hik.NewWithClient(isapi.NewWithTransport(sess.Transport()), sess.Close), nil
}
