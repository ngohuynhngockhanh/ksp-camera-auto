//go:build !linux

package traffic

import (
	"context"
	"time"
)

func (m *Manager) sniffLoop(ctx context.Context, ifaceName string) {
	// Fallback/stub for non-linux systems
	<-ctx.Done()
}
