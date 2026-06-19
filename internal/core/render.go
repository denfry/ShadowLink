// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"encoding/json" // only to MARSHAL our own map; sing-box parsing uses sing's JSON in core.go
	"fmt"

	"github.com/shadowlink/shadowlink/internal/config"
)

// BuildParams is the input to config rendering.
type BuildParams struct {
	Server   config.Server
	Settings config.Settings
}

// RenderConfig emits a sing-box JSON config for one VLESS+REALITY+Vision server,
// system-wide TUN, proxied DoH (leak-resistant), and a loopback Clash API.
// Field names verified against sing-box v1.13.13.
func RenderConfig(p BuildParams) ([]byte, error) {
	if err := p.Server.Validate(); err != nil {
		return nil, err
	}
	s := p.Server
	set := p.Settings

	proxy := map[string]any{
		"type":        "vless",
		"tag":         "proxy",
		"server":      s.Host,
		"server_port": s.Port,
		"uuid":        s.UUID,
		"flow":        s.Flow,
		"network":     "tcp",
		"tls": map[string]any{
			"enabled":     true,
			"server_name": s.SNI,
			"utls":        map[string]any{"enabled": true, "fingerprint": s.Fingerprint},
			"reality":     map[string]any{"enabled": true, "public_key": s.PublicKey, "short_id": s.ShortID},
		},
	}

	cfg := map[string]any{
		"log": map[string]any{"level": set.LogLevel, "timestamp": true},
		"dns": map[string]any{
			"servers": []any{
				map[string]any{"tag": "proxy-doh", "type": "https", "server": set.DoHResolver, "detour": "proxy"},
			},
			"final":    "proxy-doh",
			"strategy": "prefer_ipv4",
		},
		"inbounds": []any{
			map[string]any{
				"type":         "tun",
				"tag":          "tun-in",
				"address":      []any{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
				"mtu":          1420,
				"auto_route":   true,
				"strict_route": true,
				"stack":        "mixed",
			},
		},
		"outbounds": []any{
			proxy,
			map[string]any{"type": "direct", "tag": "direct"},
		},
		"route": map[string]any{
			"auto_detect_interface": true,
			"final":                 "proxy",
			"rules": []any{
				map[string]any{"action": "sniff"},
				map[string]any{"protocol": "dns", "action": "hijack-dns"},
			},
		},
		"experimental": map[string]any{
			"cache_file": map[string]any{"enabled": true},
			"clash_api": map[string]any{
				"external_controller": fmt.Sprintf("127.0.0.1:%d", set.ClashAPIPort),
				"secret":              set.ClashAPISecret,
			},
		},
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal sing-box config: %w", err)
	}
	return b, nil
}
