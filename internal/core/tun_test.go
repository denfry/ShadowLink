// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"net"
	"strings"
	"testing"
)

func TestIPv4AssignedMatches(t *testing.T) {
	_, n, _ := net.ParseCIDR("172.18.0.1/30")
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.1.5"), Mask: n.Mask},
		&net.IPNet{IP: net.ParseIP("172.18.0.1"), Mask: n.Mask},
		&net.IPAddr{IP: net.ParseIP("10.9.8.7")}, // covers the *net.IPAddr switch arm
	}
	if !IPv4Assigned("172.18.0.1", addrs) {
		t.Fatal("expected 172.18.0.1 to be detected as assigned")
	}
	if !IPv4Assigned("10.9.8.7", addrs) {
		t.Fatal("expected 10.9.8.7 (*net.IPAddr form) to be detected as assigned")
	}
	if IPv4Assigned("10.0.0.1", addrs) {
		t.Fatal("did not expect 10.0.0.1 to be assigned")
	}
}

func TestTUNConstantsAreNonDefault(t *testing.T) {
	// Reject any form of the colliding default IP (172.19.0.1), not just one
	// exact CIDR string — a different mask/host in that space still clashes.
	if strings.HasPrefix(TUNAddress4CIDR, "172.19.0.1") {
		t.Fatalf("TUN must NOT use the sing-box default 172.19.0.1 (collides with Hiddify/Nekoray), got %q", TUNAddress4CIDR)
	}
	if TUNInterfaceName != "shadowlink0" {
		t.Fatalf("TUN interface name must be the distinct shadowlink0, got %q", TUNInterfaceName)
	}
}
