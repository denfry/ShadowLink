// SPDX-License-Identifier: GPL-3.0-only

// Package secret masks sensitive values (UUIDs, REALITY keys, subscription
// URLs) so they never appear in logs or error messages.
package secret

import (
	"fmt"

	"github.com/shadowlink/shadowlink/internal/config"
)

// Mask hides all but a short (2-char) prefix hint of a sensitive value.
// It counts runes, so it never splits a multi-byte UTF-8 sequence even if a
// future caller passes a non-ASCII secret.
func Mask(s string) string {
	r := []rune(s)
	switch {
	case len(r) == 0:
		return ""
	case len(r) <= 4:
		return "****"
	default:
		return string(r[:2]) + "****"
	}
}

// MaskServer renders a server for logs without exposing UUID or REALITY key.
func MaskServer(s config.Server) string {
	return fmt.Sprintf("%s (%s:%d uuid=%s pbk=%s)", s.Tag, s.Host, s.Port, Mask(s.UUID), Mask(s.PublicKey))
}
