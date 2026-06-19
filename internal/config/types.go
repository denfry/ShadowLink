// SPDX-License-Identifier: GPL-3.0-only
package config

import "fmt"

// Server is one VLESS+REALITY+Vision outbound, in domain terms (core-agnostic).
type Server struct {
	Tag         string `json:"tag"`
	UUID        string `json:"uuid"`
	Host        string `json:"host"`
	Port        uint16 `json:"port"`
	Flow        string `json:"flow"`        // "" or "xtls-rprx-vision"
	Network     string `json:"network"`     // "tcp"
	SNI         string `json:"sni"`         // tls.server_name (camouflage)
	Fingerprint string `json:"fingerprint"` // utls fingerprint
	PublicKey   string `json:"public_key"`  // reality pbk
	ShortID     string `json:"short_id"`    // reality sid
}

// Settings are user-tunable, profile-wide options.
type Settings struct {
	KillSwitch     bool   `json:"kill_switch"`
	FailOpen       bool   `json:"fail_open"`
	SplitTunnel    bool   `json:"split_tunnel"`
	DoHResolver    string `json:"doh_resolver"` // e.g. "1.1.1.1"
	LogLevel       string `json:"log_level"`
	ClashAPIPort   int    `json:"clash_api_port"`
	ClashAPISecret string `json:"clash_api_secret"`
}

// Profile is the persisted state: known servers + selection + settings.
type Profile struct {
	Servers  []Server `json:"servers"`
	Selected string   `json:"selected"` // server Tag; "" means first
	Settings Settings `json:"settings"`
}

// DefaultSettings returns safe defaults (fail-closed kill switch on).
func DefaultSettings() Settings {
	return Settings{
		KillSwitch:   true,
		FailOpen:     false,
		SplitTunnel:  false,
		DoHResolver:  "1.1.1.1",
		LogLevel:     "info",
		ClashAPIPort: 9595,
	}
}

// Validate checks invariants the embedded core (and server) require.
func (s Server) Validate() error {
	if s.UUID == "" || s.Host == "" || s.Port == 0 {
		return fmt.Errorf("server %q: uuid, host and port are required", s.Tag)
	}
	if s.Flow != "" && s.Flow != "xtls-rprx-vision" {
		return fmt.Errorf("server %q: unsupported flow %q (sing-box accepts only \"\" or xtls-rprx-vision)", s.Tag, s.Flow)
	}
	if s.PublicKey == "" {
		return fmt.Errorf("server %q: REALITY public_key (pbk) is required", s.Tag)
	}
	if s.Network != "" && s.Network != "tcp" {
		return fmt.Errorf("server %q: Vision requires network tcp, got %q", s.Tag, s.Network)
	}
	return nil
}
