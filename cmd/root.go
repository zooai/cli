// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zooai/cli/cmd/chaincmd"
	"github.com/zooai/cli/cmd/networkcmd"
)

var rootCmd = &cobra.Command{
	Use:   "zoo",
	Short: "Zoo CLI — deploy and manage Zoo EVM chains",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(newDeployCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(networkcmd.NewNetworkCmd())
	rootCmd.AddCommand(chaincmd.NewChainCmd())
}
