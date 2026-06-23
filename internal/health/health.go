// SPDX-License-Identifier: GPL-3.0-only
package health

import (
	"context"
	"time"
)

// Prober measures outbound latency (e.g. the Clash API client).
type Prober interface {
	Delay(ctx context.Context, tag, url string, timeoutMS int) (int, error)
}

// Checker polls a prober and reports sustained up/down transitions.
type Checker struct {
	Prober   Prober
	Tag      string
	URL      string
	Interval time.Duration
	Failures int // consecutive failures before declaring down
}

// Run loops until ctx is done, invoking onDown/onUp on edge transitions.
func (c *Checker) Run(ctx context.Context, onDown, onUp func()) {
	t := time.NewTicker(c.Interval)
	defer t.Stop()
	consecutive := 0
	healthy := true
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, err := c.Prober.Delay(ctx, c.Tag, c.URL, 5000)
			if err != nil {
				consecutive++
				if healthy && consecutive >= c.Failures {
					healthy = false
					if onDown != nil {
						onDown()
					}
				}
				continue
			}
			consecutive = 0
			if !healthy {
				healthy = true
				if onUp != nil {
					onUp()
				}
			}
		}
	}
}
