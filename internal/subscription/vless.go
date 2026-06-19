// SPDX-License-Identifier: GPL-3.0-only
package subscription

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/shadowlink/shadowlink/internal/config"
)

// ParseVLESS parses a vless:// share link into a domain Server.
// Unknown query keys are ignored (forward-compat with pqv/mlkem etc.).
func ParseVLESS(uri string) (config.Server, error) {
	var s config.Server
	u, err := url.Parse(strings.TrimSpace(uri))
	if err != nil {
		return s, fmt.Errorf("parse uri: %w", err)
	}
	if u.Scheme != "vless" {
		return s, fmt.Errorf("not a vless:// uri (scheme %q)", u.Scheme)
	}
	if u.User == nil || u.User.Username() == "" {
		return s, fmt.Errorf("missing uuid in vless uri")
	}
	s.UUID = u.User.Username()
	s.Host = u.Hostname()
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil {
		return s, fmt.Errorf("invalid port %q: %w", u.Port(), err)
	}
	s.Port = uint16(port)

	q := u.Query()
	s.Flow = q.Get("flow")
	s.Network = q.Get("type")
	if s.Network == "" {
		s.Network = "tcp"
	}
	s.SNI = q.Get("sni")
	if s.SNI == "" {
		s.SNI = s.Host
	}
	s.Fingerprint = q.Get("fp")
	if s.Fingerprint == "" {
		s.Fingerprint = "chrome"
	}
	s.PublicKey = q.Get("pbk")
	s.ShortID = q.Get("sid")

	s.Tag = u.Fragment
	if s.Tag == "" {
		s.Tag = s.Host
	}
	return s, nil
}
