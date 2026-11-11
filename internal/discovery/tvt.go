package discovery

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/tvt"
)

// enrichPort80TVT probes discovered port-80 hosts whose Vendor is empty via
// POST /reqLogin to automatically identify TVT / Secuvision NVMS-9000 NVRs and IPCs.
func enrichPort80TVT(ctx context.Context, results []Result) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for i := range results {
		if results[i].Port != 80 || results[i].Vendor != "" || results[i].IP == "" {
			continue
		}
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			baseURL := "http://" + net.JoinHostPort(results[idx].IP, strconv.Itoa(80))
			info, err := tvt.ProbePreLogin(ctx, baseURL, 1500*time.Millisecond)
			if err != nil || info == nil {
				return
			}
			results[idx].Vendor = "tvt"
			if results[idx].Model == "" && info.SystemType != "" {
				results[idx].Model = info.SystemType
			}
		}()
	}
	wg.Wait()
}
