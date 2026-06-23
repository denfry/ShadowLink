// SPDX-License-Identifier: GPL-3.0-only
package subscription

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/shadowlink/shadowlink/internal/config"
)

// ParseSubscription decodes a subscription body and returns its servers.
// Lenient: try std base64, then url-safe (raw, no padding), then raw text.
func ParseSubscription(body []byte) ([]config.Server, error) {
	text := decodeLenient(strings.TrimSpace(string(body)))
	var servers []config.Server
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || !strings.HasPrefix(line, "vless://") {
			continue // skip blanks, comments, and other schemes
		}
		s, err := ParseVLESS(line)
		if err != nil {
			continue // skip unparseable lines rather than failing the whole feed
		}
		servers = append(servers, s)
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("no vless servers found in subscription")
	}
	return servers, nil
}

// decodeLenient returns the subscription as plain text, accepting a base64
// layer only when the decoded bytes actually look like share URIs. The guard
// matters because a plain-text body can coincidentally be valid base64; without
// it, such input would be silently mis-decoded into garbage and report "no
// servers". We require a "://" scheme separator in the decoded output before
// trusting a base64 interpretation.
func decodeLenient(s string) string {
	if d, err := base64.StdEncoding.DecodeString(s); err == nil && looksLikeURIs(d) {
		return string(d)
	}
	if d, err := base64.RawURLEncoding.DecodeString(s); err == nil && looksLikeURIs(d) {
		return string(d)
	}
	return s // assume already plain text
}

func looksLikeURIs(decoded []byte) bool {
	return strings.Contains(string(decoded), "://")
}
