// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/luxfi/constants"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/ids"
	"github.com/luxfi/math/set"
	ptxs "github.com/luxfi/proto/p/txs"
	"github.com/luxfi/sdk/info"
	"github.com/luxfi/sdk/platformvm"
	"github.com/luxfi/sdk/wallet/primary"
	"github.com/luxfi/utxo/secp256k1fx"
)

// DeployConfig holds parameters for deploying a chain.
type DeployConfig struct {
	URI             string
	GenesisBytes    []byte
	ChainName       string
	VMID            ids.ID
	ChainIDOverride uint64 // 0 means no override (EVM only)
	ChainID         string // existing chain/network ID (empty = create new)
	// Key source
	KMSURL     string
	KMSToken   string
	SecretName string
}

// DeployResult holds the result of a chain deployment.
type DeployResult struct {
	ChainID      ids.ID
	BlockchainID ids.ID
	Validators   int
}

// PatchChainID replaces the chainId field in EVM genesis JSON.
func PatchChainID(genesisBytes []byte, chainID uint64) ([]byte, error) {
	var genesis map[string]any
	if err := json.Unmarshal(genesisBytes, &genesis); err != nil {
		return nil, fmt.Errorf("unmarshal genesis: %w", err)
	}
	config, ok := genesis["config"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("genesis missing 'config' object")
	}
	config["chainId"] = chainID
	out, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal genesis: %w", err)
	}
	return out, nil
}

// DeployChain creates a chain on the Lux P-Chain and deploys a blockchain.
func DeployChain(ctx context.Context, cfg DeployConfig, logf func(string, ...any)) (*DeployResult, error) {
	genesisBytes := cfg.GenesisBytes

	if cfg.ChainIDOverride > 0 {
		var err error
		genesisBytes, err = PatchChainID(genesisBytes, cfg.ChainIDOverride)
		if err != nil {
			return nil, fmt.Errorf("patch chain ID: %w", err)
		}
		logf("Genesis chain ID overridden to %d", cfg.ChainIDOverride)
	}

	logf("Genesis: %d bytes", len(genesisBytes))

	privKey, err := LoadKey(cfg.KMSURL, cfg.KMSToken, cfg.SecretName)
	if err != nil {
		return nil, err
	}

	return deployWithKey(ctx, cfg, genesisBytes, privKey, logf)
}

func deployWithKey(ctx context.Context, cfg DeployConfig, genesisBytes []byte, privKey *secp256k1.PrivateKey, logf func(string, ...any)) (*DeployResult, error) {
	addr := privKey.Address()
	logf("Key Lux address: %s", addr)

	kc := secp256k1fx.NewKeychain(privKey)
	adapter := primary.NewKeychainAdapter(kc)

	infoClient := info.NewClient(cfg.URI)
	nodeID, _, err := infoClient.GetNodeID(ctx)
	if err != nil {
		return nil, fmt.Errorf("node unreachable at %s: %w", cfg.URI, err)
	}
	logf("Connected to node: %s", nodeID)

	pClient := platformvm.NewClient(cfg.URI)
	validators, err := pClient.GetCurrentValidators(ctx, ids.Empty, nil)
	if err != nil {
		return nil, fmt.Errorf("get validators: %w", err)
	}
	logf("Found %d validators", len(validators))

	var minEnd uint64
	for _, v := range validators {
		if minEnd == 0 || v.EndTime < minEnd {
			minEnd = v.EndTime
		}
		logf("  Validator: %s (end: %d)", v.NodeID, v.EndTime)
	}

	wallet, err := primary.MakeWallet(ctx, &primary.WalletConfig{
		URI: cfg.URI, LUXKeychain: adapter, EVMKeychain: adapter,
	})
	if err != nil {
		return nil, fmt.Errorf("wallet: %w", err)
	}

	luxAssetID := wallet.X().Builder().Context().XAssetID
	pBalance, err := wallet.P().Builder().GetBalance()
	if err != nil {
		return nil, fmt.Errorf("P-chain balance: %w", err)
	}
	pBal := pBalance[luxAssetID]
	logf("P-Chain balance: %d nLUX (%.4f LUX)", pBal, float64(pBal)/1e9)

	// Import from X->P if balance insufficient.
	if pBal < 1_000_000_000 {
		logf("P-Chain balance insufficient. Importing from X-Chain...")
		xChainID := wallet.X().Builder().Context().BlockchainID
		_, err := wallet.P().IssueImportTx(xChainID, &secp256k1fx.OutputOwners{
			Threshold: 1, Addrs: []ids.ShortID{addr},
		})
		if err != nil {
			logf("X->P import failed (%v), trying C->P export...", err)
			_, cErr := wallet.C().IssueExportTx(
				constants.PlatformChainID,
				[]*secp256k1fx.TransferOutput{{
					Amt: 10_000_000_000,
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1, Addrs: []ids.ShortID{addr},
					},
				}},
			)
			if cErr != nil {
				return nil, fmt.Errorf("C->P export: %w (X->P also failed: %v)", cErr, err)
			}
			time.Sleep(5 * time.Second)
			cChainID := wallet.C().Builder().Context().BlockchainID
			_, iErr := wallet.P().IssueImportTx(cChainID, &secp256k1fx.OutputOwners{
				Threshold: 1, Addrs: []ids.ShortID{addr},
			})
			if iErr != nil {
				return nil, fmt.Errorf("P-chain import from C: %w", iErr)
			}
		}
		time.Sleep(5 * time.Second)

		wallet, err = primary.MakeWallet(ctx, &primary.WalletConfig{
			URI: cfg.URI, LUXKeychain: adapter, EVMKeychain: adapter,
		})
		if err != nil {
			return nil, fmt.Errorf("wallet re-sync: %w", err)
		}
	}

	// Create chain on P-Chain.
	var chainID ids.ID
	if cfg.ChainID != "" {
		chainID, err = ids.FromString(cfg.ChainID)
		if err != nil {
			return nil, fmt.Errorf("invalid chain ID %q: %w", cfg.ChainID, err)
		}
		logf("Using existing chain: %s", chainID)
	} else {
		logf("Creating chain...")
		owner := &secp256k1fx.OutputOwners{
			Threshold: 1, Addrs: []ids.ShortID{addr},
		}
		chainTx, err := wallet.P().IssueCreateNetworkTx(owner)
		if err != nil {
			return nil, fmt.Errorf("create chain: %w", err)
		}
		chainID = chainTx.ID()
		logf("Chain ID: %s", chainID)
		time.Sleep(15 * time.Second)
	}

	// Create blockchain.
	logf("Creating blockchain '%s' with VM %s...", cfg.ChainName, cfg.VMID)

	fetchSet := set.Of(chainID)
	wallet2, err := primary.MakeWallet(ctx, &primary.WalletConfig{
		URI: cfg.URI, LUXKeychain: adapter, EVMKeychain: adapter,
		PChainTxsToFetch: fetchSet,
	})
	if err != nil {
		return nil, fmt.Errorf("wallet re-sync for blockchain: %w", err)
	}

	bcTx, err := wallet2.P().IssueCreateChainTx(
		chainID, genesisBytes, cfg.VMID, nil, cfg.ChainName,
	)
	if err != nil {
		return nil, fmt.Errorf("create blockchain: %w", err)
	}
	blockchainID := bcTx.ID()
	logf("Blockchain ID: %s", blockchainID)

	time.Sleep(5 * time.Second)

	// Add validators to the chain.
	if len(validators) > 0 {
		wallet3, err := primary.MakeWallet(ctx, &primary.WalletConfig{
			URI: cfg.URI, LUXKeychain: adapter, EVMKeychain: adapter,
		})
		if err != nil {
			logf("WARNING: validator wallet sync: %v", err)
		} else {
			startTime := time.Now().Add(60 * time.Second)
			endTime := startTime.Add(300 * 24 * time.Hour)
			if minEnd > 0 {
				safeEnd := time.Unix(int64(minEnd), 0).Add(-1 * time.Hour)
				if safeEnd.Before(endTime) {
					endTime = safeEnd
				}
			}
			for _, v := range validators {
				logf("Adding validator %s to chain %s...", v.NodeID, chainID)
				_, err := wallet3.P().IssueAddChainValidatorTx(&ptxs.ChainValidator{
					Validator: ptxs.Validator{
						NodeID: v.NodeID,
						Start:  uint64(startTime.Unix()),
						End:    uint64(endTime.Unix()),
						Wght:   20,
					},
					Chain: chainID,
				})
				if err != nil {
					logf("WARNING: add validator %s: %v", v.NodeID, err)
				} else {
					logf("Validator added: %s", v.NodeID)
				}
				time.Sleep(1 * time.Second)
			}
		}
	}

	return &DeployResult{
		ChainID:      chainID,
		BlockchainID: blockchainID,
		Validators:   len(validators),
	}, nil
}
