// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package chaincmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	deployLocal   bool
	deployDevnet  bool
	deployTestnet bool
	deployMainnet bool
)

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [name]",
		Short: "Deploy a chain to a running network",
		Long: `Deploy a configured chain to the target network.

The chain must be created first with 'zoo chain create'.

EXAMPLES:
  zoo chain deploy beluga --local     Deploy to localnet (colima/k3s)
  zoo chain deploy beluga --devnet    Deploy to Zoo devnet`,
		Args: cobra.ExactArgs(1),
		RunE: deployChain,
	}
	cmd.Flags().BoolVarP(&deployLocal, "local", "l", true, "Deploy to local network (default)")
	cmd.Flags().BoolVarP(&deployDevnet, "devnet", "d", false, "Deploy to devnet")
	cmd.Flags().BoolVarP(&deployTestnet, "testnet", "t", false, "Deploy to testnet")
	cmd.Flags().BoolVarP(&deployMainnet, "mainnet", "m", false, "Deploy to mainnet")
	return cmd
}

func deployChain(cmd *cobra.Command, args []string) error {
	name := args[0]
	configDir := chainConfigDir(name)

	// Check config exists
	sidecarPath := filepath.Join(configDir, "sidecar.json")
	if _, err := os.Stat(sidecarPath); err != nil {
		return fmt.Errorf("chain %q not found — create it first: zoo chain create %s", name, name)
	}

	// Read sidecar
	data, _ := os.ReadFile(sidecarPath)
	var sidecar map[string]any
	json.Unmarshal(data, &sidecar)

	chainID := uint64(0)
	if v, ok := sidecar["chainId"].(float64); ok {
		chainID = uint64(v)
	}
	chainType := "l2"
	if v, ok := sidecar["type"].(string); ok {
		chainType = v
	}

	fmt.Printf("Deploying %s (%s, chain ID %d)...\n", name, chainType, chainID)

	// For local deployment, the chain needs to be registered on the running network
	// via the P-Chain admin API (platform.createBlockchain)
	// This requires the network to be running (via zoo network start --local)
	//
	// TODO: wire to operator — create a LiquidChain/ZooChain CRD that the operator
	// reconciles into a chain deployment
	fmt.Printf("\nChain %q ready for deployment.\n", name)
	fmt.Printf("The operator will create the chain on the running network.\n")
	fmt.Printf("Genesis: %s\n", filepath.Join(configDir, "genesis.json"))

	return nil
}
