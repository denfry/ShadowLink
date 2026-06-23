// SPDX-License-Identifier: GPL-3.0-only

//go:build !windows

package platform

import "fmt"

type stubKillSwitch struct{}

// NewKillSwitch returns a stub on non-Windows OSes (Phase 1 ships Windows first).
func NewKillSwitch() KillSwitch { return &stubKillSwitch{} }

func (s *stubKillSwitch) Enable(serverIP string) error {
	return fmt.Errorf("kill-switch not implemented on this OS yet; rerun with --fail-open to proceed without it")
}

func (s *stubKillSwitch) Disable() error { return nil }
