// SPDX-License-Identifier: GPL-3.0-only
package secret

import (
	"strings"
	"testing"
)

func TestScrubHidesUUIDAndKeys(t *testing.T) {
	in := "start failed for 11111111-2222-3333-4444-555555555555 pbk=SUPERSECRETKEY1234567890"
	out := Scrub(in)
	if strings.Contains(out, "2222-3333") || strings.Contains(out, "SUPERSECRETKEY1234567890") {
		t.Fatalf("secret leaked: %q", out)
	}
	if !strings.Contains(out, "start failed for") {
		t.Fatalf("scrub destroyed the message: %q", out)
	}
}
