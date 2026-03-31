// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Zoo CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("zoo-cli v0.1.0")
		},
	}
}
