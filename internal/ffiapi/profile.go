// SPDX-License-Identifier: GPL-3.0-only
package ffiapi

import (
	"fmt"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/secret"
)

// ListServers returns the stored servers (masked) and the current selection.
func (a *API) ListServers() string {
	prof, err := a.load()
	if err != nil {
		return errJSON(err, false)
	}
	return okJSON(map[string]any{
		"servers":  serverSummaries(prof.Servers),
		"selected": prof.Selected,
	})
}

// Import adds server(s) from a uri/file/inline body, persists, returns the list.
func (a *API) Import(input string) string {
	servers, err := a.importFn(input)
	if err != nil {
		return errJSON(err, false)
	}
	for _, s := range servers {
		if verr := s.Validate(); verr != nil {
			return errJSON(verr, false)
		}
	}
	prof, err := a.load()
	if err != nil {
		return errJSON(err, false)
	}
	prof.Servers = mergeServers(prof.Servers, servers)
	if prof.Selected == "" && len(prof.Servers) > 0 {
		prof.Selected = prof.Servers[0].Tag
	}
	if err := a.save(prof); err != nil {
		return errJSON(err, false)
	}
	return okJSON(map[string]any{
		"servers":  serverSummaries(prof.Servers),
		"selected": prof.Selected,
	})
}

// Select sets the active server by tag.
func (a *API) Select(tag string) string {
	prof, err := a.load()
	if err != nil {
		return errJSON(err, false)
	}
	for _, s := range prof.Servers {
		if s.Tag == tag {
			prof.Selected = tag
			if err := a.save(prof); err != nil {
				return errJSON(err, false)
			}
			return okJSON(nil)
		}
	}
	return errJSON(fmt.Errorf("no server with tag %q", tag), false)
}

func serverSummaries(servers []config.Server) []map[string]any {
	out := make([]map[string]any, 0, len(servers))
	for _, s := range servers {
		out = append(out, map[string]any{
			"tag":    s.Tag,
			"host":   s.Host,
			"masked": secret.MaskServer(s),
		})
	}
	return out
}

func mergeServers(existing, incoming []config.Server) []config.Server {
	byTag := map[string]int{}
	for i, s := range existing {
		byTag[s.Tag] = i
	}
	for _, s := range incoming {
		if i, ok := byTag[s.Tag]; ok {
			existing[i] = s
		} else {
			existing = append(existing, s)
		}
	}
	return existing
}
