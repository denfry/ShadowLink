// SPDX-License-Identifier: GPL-3.0-only
package secret

import (
	"fmt"

	"github.com/shadowlink/shadowlink/internal/config"
)

// Mask hides all but a short prefix of a sensitive value.
func Mask(s string) string {
	switch {
	case s == "":
		return ""
	case len(s) <= 4:
		return "****"
	default:
		return s[:2] + "****"
	}
}

// MaskServer renders a server for logs without exposing UUID or REALITY key.
func MaskServer(s config.Server) string {
	return fmt.Sprintf("%s (%s:%d uuid=%s pbk=%s)", s.Tag, s.Host, s.Port, Mask(s.UUID), Mask(s.PublicKey))
}
