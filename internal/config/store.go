// SPDX-License-Identifier: GPL-3.0-only
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Dir returns the per-user config directory for ShadowLink, creating it.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate config dir: %w", err)
	}
	dir := filepath.Join(base, "shadowlink")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

func profilePath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "profile.json"), nil
}

// Load reads the stored profile, or returns a default one if none exists.
func Load() (Profile, error) {
	p, err := profilePath()
	if err != nil {
		return Profile{}, err
	}
	return LoadFrom(p)
}

// Save persists the profile to the default location.
func Save(p Profile) error {
	path, err := profilePath()
	if err != nil {
		return err
	}
	return SaveTo(path, p)
}

// LoadFrom reads a profile from path; a missing file yields a default profile.
func LoadFrom(path string) (Profile, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Profile{Settings: DefaultSettings()}, nil
	}
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	if p.Settings.DoHResolver == "" {
		p.Settings = DefaultSettings()
	}
	return p, nil
}

// SaveTo writes a profile atomically with owner-only permissions.
func SaveTo(path string, p Profile) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	return os.Rename(tmp, path)
}
