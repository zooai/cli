// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package chaincmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	chainType      string
	evmChainID     uint64
	tokenName      string
	tokenSymbol    string
	airdropAddress string
	forceCreate    bool
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new chain configuration",
		Long: `Create a chain configuration for deployment.

TYPES:
  l1    Sovereign L1
  l2    Layer 2 (default)
  l3    App-specific L3 (Beluga, etc.)

EXAMPLES:
  zoo chain create beluga --type=l3 --evm-chain-id=420420 --token-name=BELUGA --token-symbol=BLG
  zoo chain create mychain --type=l2 --evm-chain-id=200201`,
		Args: cobra.ExactArgs(1),
		RunE: createChain,
	}
	cmd.Flags().StringVar(&chainType, "type", "l2", "Chain type: l1, l2, l3")
	cmd.Flags().Uint64Var(&evmChainID, "evm-chain-id", 0, "EVM chain ID (default: auto)")
	cmd.Flags().StringVar(&tokenName, "token-name", "", "Native token name")
	cmd.Flags().StringVar(&tokenSymbol, "token-symbol", "", "Native token symbol")
	cmd.Flags().StringVar(&airdropAddress, "airdrop", "", "Address to fund in genesis (hex, no 0x prefix)")
	cmd.Flags().BoolVar(&forceCreate, "force", false, "Overwrite existing config")
	return cmd
}

func createChain(cmd *cobra.Command, args []string) error {
	name := args[0]
	configDir := chainConfigDir(name)

	if !forceCreate {
		if _, err := os.Stat(configDir); err == nil {
			return fmt.Errorf("chain %q already exists (use --force to overwrite)", name)
		}
	}

	// Defaults per chain type
	if evmChainID == 0 {
		switch chainType {
		case "l1":
			evmChainID = 200200
		case "l3":
			evmChainID = 420420
		default:
			evmChainID = 200201
		}
	}
	if tokenName == "" {
		tokenName = strings.ToUpper(name)
	}
	if tokenSymbol == "" {
		tokenSymbol = strings.ToUpper(name)
		if len(tokenSymbol) > 5 {
			tokenSymbol = tokenSymbol[:5]
		}
	}
	if airdropAddress == "" {
		airdropAddress = "9011E888251AB053B7bD1cdB598Db4f9DEd94714"
	}

	// Generate genesis
	genesis := map[string]any{
		"config": map[string]any{
			"chainId":             evmChainID,
			"homesteadBlock":      0,
			"eip150Block":         0,
			"eip155Block":         0,
			"eip158Block":         0,
			"byzantiumBlock":      0,
			"constantinopleBlock": 0,
			"petersburgBlock":     0,
			"istanbulBlock":       0,
			"muirGlacierBlock":    0,
			"shanghaiTime":        0,
		},
		"nonce":      "0x0",
		"timestamp":  "0x0",
		"gasLimit":   "0x5f5e100",
		"difficulty": "0x0",
		"mixHash":    "0x0000000000000000000000000000000000000000000000000000000000000000",
		"coinbase":   "0x0000000000000000000000000000000000000000",
		"alloc": map[string]any{
			airdropAddress: map[string]string{
				"balance": "0x193e5939a08ce9dbd480000000",
			},
		},
	}

	// Write config
	os.MkdirAll(configDir, 0o755)

	genesisPath := filepath.Join(configDir, "genesis.json")
	f, err := os.Create(genesisPath)
	if err != nil {
		return fmt.Errorf("create genesis: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(genesis)

	// Write sidecar (metadata)
	sidecar := map[string]any{
		"name":        name,
		"type":        chainType,
		"chainId":     evmChainID,
		"tokenName":   tokenName,
		"tokenSymbol": tokenSymbol,
		"vm":          "evm",
	}
	sc, _ := os.Create(filepath.Join(configDir, "sidecar.json"))
	defer sc.Close()
	enc2 := json.NewEncoder(sc)
	enc2.SetIndent("", "  ")
	enc2.Encode(sidecar)

	layer := "L2"
	switch chainType {
	case "l1":
		layer = "L1"
	case "l3":
		layer = "L3"
	}

	fmt.Printf("Chain %q created (%s)\n", name, layer)
	fmt.Printf("  Chain ID:  %d\n", evmChainID)
	fmt.Printf("  Token:     %s (%s)\n", tokenName, tokenSymbol)
	fmt.Printf("  Genesis:   %s\n", genesisPath)
	fmt.Printf("\nDeploy with: zoo chain deploy %s --local\n", name)
	return nil
}

func chainConfigDir(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zoo", "chains", name)
}
