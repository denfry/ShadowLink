// SPDX-License-Identifier: GPL-3.0-only
package version

// Version is overridden at build time via -ldflags "-X .../version.Version=...".
var Version = "0.0.0-dev"

// String returns the build version string.
func String() string { return Version }
