// SPDX-License-Identifier: GPL-3.0-only

//go:build windows

package platform

import (
	"fmt"
	"os/exec"
)

type windowsKillSwitch struct{}

// NewKillSwitch returns the Windows (netsh advfirewall) kill switch.
func NewKillSwitch() KillSwitch { return &windowsKillSwitch{} }

func (w *windowsKillSwitch) Enable(serverIP string) error {
	// Clean any stale rules first (idempotent).
	_ = w.Disable()
	// Default-deny outbound for all profiles.
	if err := netsh("advfirewall", "set", "allprofiles", "firewallpolicy", "blockinbound,blockoutbound"); err != nil {
		return fmt.Errorf("set block policy: %w", err)
	}
	// Allow reaching the server IP on the physical NIC (REALITY handshake).
	if serverIP != "" {
		if err := netsh("advfirewall", "firewall", "add", "rule",
			"name="+RuleName, "dir=out", "action=allow", "remoteip="+serverIP); err != nil {
			return fmt.Errorf("allow server ip: %w", err)
		}
	}
	// Allow loopback so the Clash API and local apps keep working.
	if err := netsh("advfirewall", "firewall", "add", "rule",
		"name="+RuleName, "dir=out", "action=allow", "remoteip=127.0.0.1"); err != nil {
		return fmt.Errorf("allow loopback: %w", err)
	}
	return nil
}

func (w *windowsKillSwitch) Disable() error {
	// Remove our rules and restore default-allow outbound.
	_ = netsh("advfirewall", "firewall", "delete", "rule", "name="+RuleName)
	if err := netsh("advfirewall", "set", "allprofiles", "firewallpolicy", "blockinbound,allowoutbound"); err != nil {
		return fmt.Errorf("restore policy: %w", err)
	}
	return nil
}

func netsh(args ...string) error {
	out, err := exec.Command("netsh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh %v: %v: %s", args, err, string(out))
	}
	return nil
}
