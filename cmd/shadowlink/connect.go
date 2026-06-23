// SPDX-License-Identifier: GPL-3.0-only
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
	"github.com/shadowlink/shadowlink/internal/health"
	"github.com/shadowlink/shadowlink/internal/manager"
	"github.com/shadowlink/shadowlink/internal/platform"
	"github.com/shadowlink/shadowlink/internal/secret"
	"github.com/shadowlink/shadowlink/internal/subscription"
	"github.com/spf13/cobra"
)

func connectCmd() *cobra.Command {
	var failOpen bool
	cmd := &cobra.Command{
		Use:   "connect [vless-uri]",
		Short: "Connect to the selected server (or a one-off vless:// link)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prof, err := config.Load()
			if err != nil {
				return err
			}
			srv, err := pickServer(prof, args)
			if err != nil {
				return err
			}
			set := prof.Settings
			if failOpen {
				set.KillSwitch = false
				set.FailOpen = true
			}
			set.ClashAPISecret = randomSecret()

			// Carry-forward (Task 1.0): warn if our TUN address is already taken
			// (e.g. another sing-box client like Hiddify is up). Non-fatal — the
			// bring-up below would otherwise fail with "address already exists".
			if inUse, _ := core.LocalTUNAddrInUse(); inUse {
				fmt.Fprintf(os.Stderr, "warning: %s is already assigned to a local interface (another VPN client running?); exit it first if connect fails.\n", core.TUNAddress4CIDR)
			}

			m := manager.New(manager.Deps{
				Render:    core.RenderConfig,
				NewTunnel: func(b []byte) (manager.Tunnel, error) { return core.New(b) },
				KillSwitch: platform.NewKillSwitch(),
			})
			if err := m.Connect(srv, set); err != nil {
				return err
			}
			fmt.Printf("connected via %s\n", secret.MaskServer(srv))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			clash := core.NewClashClient(set.ClashAPIPort, set.ClashAPISecret)
			checker := &health.Checker{Prober: clash, Tag: "proxy", URL: "https://www.gstatic.com/generate_204", Interval: 15 * time.Second, Failures: 3}
			go checker.Run(ctx,
				func() { fmt.Println("health: connection degraded") },
				func() { fmt.Println("health: connection recovered") },
			)

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sig)
			<-sig
			fmt.Println("\ndisconnecting...")
			return m.Disconnect()
		},
	}
	cmd.Flags().BoolVar(&failOpen, "fail-open", false, "do NOT enable the kill switch (traffic may leak if the tunnel drops)")
	return cmd
}

func pickServer(prof config.Profile, args []string) (config.Server, error) {
	if len(args) == 1 {
		return subscription.ParseVLESS(args[0])
	}
	if len(prof.Servers) == 0 {
		return config.Server{}, fmt.Errorf("no servers; run: shadowlink import <uri|file|url>")
	}
	if prof.Selected != "" {
		for _, s := range prof.Servers {
			if s.Tag == prof.Selected {
				return s, nil
			}
		}
		fmt.Fprintf(os.Stderr, "warning: selected server %q not found; using %q\n", prof.Selected, prof.Servers[0].Tag)
	}
	return prof.Servers[0], nil
}

func randomSecret() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
