// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package cmd

import "github.com/spf13/cobra"

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy Zoo chains to a Lux network",
	}
	cmd.AddCommand(newDeployEVMCmd())
	return cmd
}
