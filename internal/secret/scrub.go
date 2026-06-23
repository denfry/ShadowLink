// SPDX-License-Identifier: GPL-3.0-only
package secret

import "regexp"

// uuidRe matches a canonical UUID; tokenRe matches long base64/hex-ish runs
// (REALITY keys, secrets) of 16+ url-safe-base64 characters.
var (
	uuidRe  = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	tokenRe = regexp.MustCompile(`[A-Za-z0-9_\-]{16,}`)
)

// Scrub masks UUID-like and long token-like substrings inside a free-form
// string (e.g. an error message), so secrets never reach logs/UI.
func Scrub(s string) string {
	s = uuidRe.ReplaceAllStringFunc(s, Mask)
	s = tokenRe.ReplaceAllStringFunc(s, Mask)
	return s
}
