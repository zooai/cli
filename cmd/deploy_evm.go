// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/luxfi/ids"
	"github.com/spf13/cobra"
	"github.com/zoo-labs/cli/internal/deploy"
)

//go:embed genesis_evm.json
var defaultEVMGenesis []byte

// Standard Lux EVM VM ID.
var evmVMID = ids.FromStringOrPanic("srEXiWaHuhNyGwPUi444Tu47ZEDwxTWrbQiuD7FmgSAQ6X7Dy")

func newDeployEVMCmd() *cobra.Command {
	var (
		uri        string
		genesis    string
		chainName  string
		chainID    uint64
		existingID string
		kmsURL     string
		kmsToken   string
		secretName string
	)

	cmd := &cobra.Command{
		Use:   "evm",
		Short: "Deploy the Zoo EVM chain to a Lux network",
		Long: `Deploy the Zoo EVM blockchain by creating a chain on the Lux P-Chain.

Key loading priority:
  1. KMS -- if --kms-url or KMS_URL is set
  2. PRIVATE_KEY env (hex)
  3. MNEMONIC env (BIP39) + KEY_INDEX

The command:
  1. Loads a deployer key (KMS > env)
  2. Checks P-Chain balance, imports from X/C-Chain if needed
  3. Creates a chain (or uses --chain-id for existing)
  4. Creates the blockchain with the Zoo EVM VM
  5. Adds all current validators

Examples:
  # Deploy with mnemonic
  export MNEMONIC="your twelve word mnemonic phrase here"
  export KEY_INDEX=1
  zoo deploy evm --uri https://api.lux-dev.network --evm-chain-id 200200

  # Deploy to existing chain
  zoo deploy evm --chain-id <existing-chain-id>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeployEVM(cmd, uri, genesis, chainName, chainID, existingID, kmsURL, kmsToken, secretName)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&uri, "uri", envOr("NODE_URI", "https://api.lux-dev.network"), "Node RPC endpoint")
	cmd.Flags().StringVar(&genesis, "genesis", os.Getenv("GENESIS_FILE"), "Path to genesis JSON (empty = embedded default)")
	cmd.Flags().StringVar(&chainName, "chain-name", envOr("CHAIN_NAME", "Zoo EVM"), "Blockchain name")
	cmd.Flags().Uint64Var(&chainID, "evm-chain-id", envUint64("EVM_CHAIN_ID", 0), "EVM chain ID override (0 = use genesis value)")
	cmd.Flags().StringVar(&existingID, "chain-id", os.Getenv("CHAIN_ID"), "Existing chain ID (empty = create new)")
	cmd.Flags().StringVar(&kmsURL, "kms-url", os.Getenv("KMS_URL"), "KMS URL for key fetching")
	cmd.Flags().StringVar(&kmsToken, "kms-token", os.Getenv("KMS_TOKEN"), "KMS auth token")
	cmd.Flags().StringVar(&secretName, "secret-name", envOr("KMS_SECRET_NAME", "zoo-deployer-key"), "KMS secret name for deployer key")

	return cmd
}

func runDeployEVM(cmd *cobra.Command, uri, genesisFile, chainName string, evmChainID uint64, existingChainID, kmsURL, kmsToken, secretName string) error {
	genesisBytes := defaultEVMGenesis
	genesisSource := "embedded"
	if genesisFile != "" {
		var err error
		genesisBytes, err = os.ReadFile(genesisFile)
		if err != nil {
			return fmt.Errorf("read genesis %s: %w", genesisFile, err)
		}
		genesisSource = genesisFile
	}

	logf := func(format string, args ...any) {
		cmd.PrintErrf(format+"\n", args...)
	}

	logf("Genesis: %d bytes from %s", len(genesisBytes), genesisSource)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	result, err := deploy.DeployChain(ctx, deploy.DeployConfig{
		URI:             uri,
		GenesisBytes:    genesisBytes,
		ChainName:       chainName,
		VMID:            evmVMID,
		ChainIDOverride: evmChainID,
		ChainID:         existingChainID,
		KMSURL:          kmsURL,
		KMSToken:        kmsToken,
		SecretName:      secretName,
	}, logf)
	if err != nil {
		return err
	}

	cmd.Println()
	cmd.Println("=== Zoo EVM Deployed ===")
	cmd.Printf("Chain ID:      %s\n", result.ChainID)
	cmd.Printf("Blockchain ID: %s\n", result.BlockchainID)
	cmd.Printf("VM ID:         %s\n", evmVMID)
	cmd.Printf("Chain Name:    %s\n", chainName)
	cmd.Printf("Validators:    %d\n", result.Validators)
	cmd.Println()
	cmd.Printf("RPC: %s/ext/bc/%s/rpc\n", uri, result.BlockchainID)

	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envUint64(key string, fallback uint64) uint64 {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err == nil {
			return n
		}
	}
	return fallback
}
