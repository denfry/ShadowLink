// SPDX-License-Identifier: GPL-3.0-only
package core

import "net"

// ShadowLink uses a DISTINCT TUN identity so it never collides with other
// embedded-sing-box clients (Hiddify/Nekoray default to interface "tun0" on
// 172.19.0.1). The Phase 0 gate hit "set ipv4 address: The object already
// exists" because of exactly that clash.
const (
	TUNInterfaceName = "shadowlink0"
	TUNAddress4CIDR  = "172.18.0.1/30"
	TUNAddress6CIDR  = "fdfe:dcba:9876::1/126"
	tunAddress4IP    = "172.18.0.1"
)

// IPv4Assigned reports whether ip is present among addrs (the concrete types
// net.InterfaceAddrs returns: *net.IPNet / *net.IPAddr). Pure -> unit-testable
// without touching the host's interfaces.
func IPv4Assigned(ip string, addrs []net.Addr) bool {
	target := net.ParseIP(ip)
	if target == nil {
		return false
	}
	for _, a := range addrs {
		var got net.IP
		switch v := a.(type) {
		case *net.IPNet:
			got = v.IP
		case *net.IPAddr:
			got = v.IP
		}
		if got != nil && got.Equal(target) {
			return true
		}
	}
	return false
}

// LocalTUNAddrInUse reports whether ShadowLink's TUN IPv4 is already assigned to
// a local interface (e.g. another VPN client is up). The CLI warns on true
// before bring-up, which would otherwise fail with "address already exists".
func LocalTUNAddrInUse() (bool, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false, err
	}
	return IPv4Assigned(tunAddress4IP, addrs), nil
}
