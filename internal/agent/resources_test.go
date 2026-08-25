// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"testing"
	"time"
)

func TestResourceCollector_LatestBeforeSample(t *testing.T) {
	c := NewResourceCollector(time.Hour, "/")
	got := c.Latest()
	if got.CPUCores != 0 || got.MemTotalBytes != 0 {
		t.Errorf("Latest() before any sample = %+v, want the zero value", got)
	}
}

func TestResourceCollector_Sample(t *testing.T) {
	c := NewResourceCollector(time.Hour, "/")
	c.sample(context.Background())

	got := c.Latest()
	if got.CPUCores <= 0 {
		t.Errorf("Latest().CPUCores = %d, want > 0", got.CPUCores)
	}
	if got.MemTotalBytes == 0 {
		t.Error("Latest().MemTotalBytes = 0, want a real total")
	}
	if got.DiskTotalBytes == 0 {
		t.Error("Latest().DiskTotalBytes = 0, want a real total for \"/\"")
	}
}

func TestResourceCollector_RunSamplesImmediatelyAndStopsOnCancel(t *testing.T) {
	c := NewResourceCollector(time.Millisecond, "/")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	// Run samples synchronously before entering its loop, but it does so
	// in the background goroutine above — poll briefly rather than
	// assuming a fixed sleep is enough.
	deadline := time.After(time.Second)
	for c.Latest().MemTotalBytes == 0 {
		select {
		case <-deadline:
			t.Fatal("Run() did not produce a sample within 1s")
		case <-time.After(time.Millisecond):
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() did not return within 1s of context cancellation")
	}
}
