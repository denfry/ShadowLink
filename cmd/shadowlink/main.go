// SPDX-License-Identifier: GPL-3.0-only
package main

import (
	"fmt"
	"os"

	"github.com/shadowlink/shadowlink/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:     "shadowlink",
		Short:   "ShadowLink — personal VPN client over an embedded sing-box core",
		Version: version.String(),
	}
	root.AddCommand(connectCmd(), importCmd(), statusCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
