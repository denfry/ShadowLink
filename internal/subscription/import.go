// SPDX-License-Identifier: GPL-3.0-only
package subscription

import (
	"errors"
	"os"
	"strings"

	"github.com/shadowlink/shadowlink/internal/config"
)

// ErrRemote signals that the input is a remote URL the caller must fetch
// (possibly through the active tunnel) and then pass the body to ParseSubscription.
var ErrRemote = errors.New("remote subscription URL")

// Import turns user input (a uri, a file path, or a subscription body) into servers.
func Import(input string) ([]config.Server, error) {
	in := strings.TrimSpace(input)
	switch {
	case strings.HasPrefix(in, "vless://"):
		s, err := ParseVLESS(in)
		if err != nil {
			return nil, err
		}
		return []config.Server{s}, nil
	case strings.HasPrefix(in, "http://") || strings.HasPrefix(in, "https://"):
		return nil, ErrRemote
	default:
		if b, err := os.ReadFile(in); err == nil {
			return ParseSubscription(b)
		}
		return ParseSubscription([]byte(in)) // treat as inline body
	}
}
