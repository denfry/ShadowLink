// SPDX-License-Identifier: GPL-3.0-only
package platform

// RuleName identifies ShadowLink's firewall rules for creation and cleanup.
const RuleName = "ShadowLink-KillSwitch"

// KillSwitch enforces fail-closed networking independent of the core process.
type KillSwitch interface {
	// Enable blocks all egress except via the tunnel, allowing only the server IP
	// on the physical NIC (for the handshake). Persists across a core crash.
	Enable(serverIP string) error
	// Disable removes all ShadowLink firewall rules, restoring normal networking.
	Disable() error
}
