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

func (w *windowsKillSwitch) Enable(serverIP string) (retErr error) {
	if serverIP == "" {
		return fmt.Errorf("kill switch: serverIP is required (nothing would be reachable for the handshake)")
	}
	// Clear stale rules WITHOUT touching the policy: calling Disable() here would
	// restore allowoutbound and open a leak window before we re-block below.
	w.clearRules()
	// On any failure, roll back to a clean disabled state rather than leaving the
	// host half-configured (blocked policy with only some allow rules).
	defer func() {
		if retErr != nil {
			_ = w.Disable()
		}
	}()
	// Default-deny outbound for all profiles (fail-closed).
	if err := netsh("advfirewall", "set", "allprofiles", "firewallpolicy", "blockinbound,blockoutbound"); err != nil {
		return fmt.Errorf("set block policy: %w", err)
	}
	// Allow reaching the server IP on the physical NIC (REALITY handshake).
	if err := netsh("advfirewall", "firewall", "add", "rule",
		"name="+RuleName, "dir=out", "action=allow", "remoteip="+serverIP); err != nil {
		return fmt.Errorf("allow server ip: %w", err)
	}
	// Allow loopback (IPv4+IPv6) so the Clash API and local apps keep working.
	if err := netsh("advfirewall", "firewall", "add", "rule",
		"name="+RuleName, "dir=out", "action=allow", "remoteip=127.0.0.1,::1"); err != nil {
		return fmt.Errorf("allow loopback: %w", err)
	}
	return nil
}

func (w *windowsKillSwitch) Disable() error {
	w.clearRules()
	// NOTE: this restores the Windows DEFAULT (allowoutbound), not whatever policy
	// the user had before Enable. Phase 1 assumes the default; saving/restoring a
	// non-default prior policy is a known follow-up.
	if err := netsh("advfirewall", "set", "allprofiles", "firewallpolicy", "blockinbound,allowoutbound"); err != nil {
		return fmt.Errorf("restore policy: %w", err)
	}
	return nil
}

// clearRules removes ShadowLink's allow rules without altering the firewall
// policy, so callers control the block/allow transition order themselves.
func (w *windowsKillSwitch) clearRules() {
	_ = netsh("advfirewall", "firewall", "delete", "rule", "name="+RuleName)
}

func netsh(args ...string) error {
	out, err := exec.Command("netsh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh %v: %v: %s", args, err, string(out))
	}
	return nil
}
