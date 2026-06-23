// SPDX-License-Identifier: GPL-3.0-only
package main

import (
	"fmt"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/secret"
	"github.com/shadowlink/shadowlink/internal/subscription"
	"github.com/spf13/cobra"
)

func importCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <vless-uri | file | subscription-url>",
		Short: "Import server(s) into the stored profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			servers, err := subscription.Import(args[0])
			if err == subscription.ErrRemote {
				return fmt.Errorf("remote subscription fetch lands in Phase 2; for now save it to a file and import the file")
			}
			if err != nil {
				return err
			}
			for _, s := range servers {
				if verr := s.Validate(); verr != nil {
					return verr
				}
			}
			prof, err := config.Load()
			if err != nil {
				return err
			}
			prof.Servers = mergeServers(prof.Servers, servers)
			if prof.Selected == "" && len(prof.Servers) > 0 {
				prof.Selected = prof.Servers[0].Tag
			}
			if err := config.Save(prof); err != nil {
				return err
			}
			fmt.Printf("imported %d server(s); %d total\n", len(servers), len(prof.Servers))
			return nil
		},
	}
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

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show stored servers and selection",
		RunE: func(cmd *cobra.Command, args []string) error {
			prof, err := config.Load()
			if err != nil {
				return err
			}
			if len(prof.Servers) == 0 {
				fmt.Println("no servers imported")
				return nil
			}
			fmt.Printf("kill-switch: %v  split-tunnel: %v\n", prof.Settings.KillSwitch, prof.Settings.SplitTunnel)
			for _, s := range prof.Servers {
				marker := " "
				if s.Tag == prof.Selected {
					marker = "*"
				}
				fmt.Printf(" %s %s\n", marker, secret.MaskServer(s))
			}
			return nil
		},
	}
}
