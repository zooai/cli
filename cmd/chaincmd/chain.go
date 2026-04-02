// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package chaincmd

import "github.com/spf13/cobra"

func NewChainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chain",
		Short: "Create, deploy, and manage Zoo chains (L1/L2/L3)",
		Long: `Manage Zoo blockchain configurations and deployments.

CHAIN TYPES:
  l1    Sovereign L1 with independent validation
  l2    Layer 2 chain on Lux/Zoo (default)
  l3    App-specific L3 chain (e.g., Beluga AI chain)

EXAMPLES:
  # Create Beluga AI chain as L3
  zoo chain create beluga --type=l3 --evm-chain-id=420420 --token-name=BELUGA --token-symbol=BLG

  # Deploy to running localnet
  zoo chain deploy beluga --local

  # List configured chains
  zoo chain list`,
	}
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newDeployCmd())
	cmd.AddCommand(newListCmd())
	return cmd
}
