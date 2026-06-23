// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"net"
	"testing"
)

func TestIPv4AssignedMatches(t *testing.T) {
	_, n, _ := net.ParseCIDR("172.18.0.1/30")
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.1.5"), Mask: n.Mask},
		&net.IPNet{IP: net.ParseIP("172.18.0.1"), Mask: n.Mask},
	}
	if !IPv4Assigned("172.18.0.1", addrs) {
		t.Fatal("expected 172.18.0.1 to be detected as assigned")
	}
	if IPv4Assigned("10.0.0.1", addrs) {
		t.Fatal("did not expect 10.0.0.1 to be assigned")
	}
}

func TestTUNConstantsAreNonDefault(t *testing.T) {
	if TUNAddress4CIDR == "172.19.0.1/30" {
		t.Fatal("TUN must NOT use the sing-box default 172.19.0.1 (collides with Hiddify/Nekoray)")
	}
	if TUNInterfaceName == "" || TUNInterfaceName == "tun0" {
		t.Fatalf("TUN interface name must be distinct, got %q", TUNInterfaceName)
	}
}
