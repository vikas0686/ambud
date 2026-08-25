// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/httputil"
)

// ResourceCollector periodically samples host CPU/RAM/disk usage in
// the background, so GET /v1/resources reads a cached value instead of
// blocking a request on syscalls every time — see docs/ROADMAP.md
// Phase 2.
type ResourceCollector struct {
	interval time.Duration
	diskPath string

	mu     sync.RWMutex
	latest apitypes.Resources
}

// NewResourceCollector returns a collector that samples every interval
// and reports disk usage for diskPath (typically "/").
func NewResourceCollector(interval time.Duration, diskPath string) *ResourceCollector {
	return &ResourceCollector{interval: interval, diskPath: diskPath}
}

// Run samples resources once immediately (so Latest has a value right
// away) and then every interval, until ctx is cancelled. Call it in its
// own goroutine.
func (c *ResourceCollector) Run(ctx context.Context) {
	c.sample(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.sample(ctx)
		}
	}
}

// sample takes one reading. Per-metric failures (e.g. an unreadable
// disk path) are logged and leave that metric at its zero value rather
// than aborting the whole sample — a resource endpoint that reports
// "0 disk, real CPU/RAM" on a bad node is more useful than one that
// reports nothing at all.
func (c *ResourceCollector) sample(ctx context.Context) {
	var r apitypes.Resources

	if cores, err := cpu.CountsWithContext(ctx, true); err != nil {
		slog.Warn("sample CPU core count failed", "error", err)
	} else {
		r.CPUCores = cores
	}

	if pct, err := cpu.PercentWithContext(ctx, 0, false); err != nil {
		slog.Warn("sample CPU percent failed", "error", err)
	} else if len(pct) > 0 {
		r.CPUUsedPercent = pct[0]
	}

	if vm, err := mem.VirtualMemoryWithContext(ctx); err != nil {
		slog.Warn("sample memory failed", "error", err)
	} else {
		r.MemTotalBytes = vm.Total
		r.MemUsedBytes = vm.Used
		r.MemUsedPercent = vm.UsedPercent
	}

	if du, err := disk.UsageWithContext(ctx, c.diskPath); err != nil {
		slog.Warn("sample disk usage failed", "path", c.diskPath, "error", err)
	} else {
		r.DiskTotalBytes = du.Total
		r.DiskUsedBytes = du.Used
		r.DiskUsedPercent = du.UsedPercent
	}

	c.mu.Lock()
	c.latest = r
	c.mu.Unlock()
}

// Latest returns the most recent sample. Before the first sample
// completes it's the zero value, not an error — callers get a resource
// snapshot with zeros rather than a "not ready yet" failure.
func (c *ResourceCollector) Latest() apitypes.Resources {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest
}

func (h *handlers) getResources(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, h.collector.Latest())
}
