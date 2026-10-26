package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/raysharp"
)

// enrichPort80Raysharp probes discovered port-80 hosts whose Vendor is empty
// via POST /API/Login/Range to automatically identify Raysharp NVRs and AI IPCs.
func enrichPort80Raysharp(ctx context.Context, results []Result) {
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
			baseURL := fmt.Sprintf("http://%s:80", results[idx].IP)
			info, err := raysharp.ProbeLoginRange(ctx, baseURL, 1500*time.Millisecond)
			if err != nil || info == nil {
				return
			}
			results[idx].Vendor = "raysharp"
			if results[idx].Model == "" && info.SiteVersion != "" {
				results[idx].Model = info.SiteVersion
			}
			if results[idx].Name == "" && info.DeviceType != "" {
				results[idx].Name = info.DeviceType
			}
		}()
	}
	wg.Wait()
}
