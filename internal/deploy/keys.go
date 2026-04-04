// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package deploy

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	bip32 "github.com/luxfi/go-bip32"
	"github.com/luxfi/go-bip39"

	"github.com/luxfi/crypto/secp256k1"
)

// kmsSecretResponse is the response from Hanzo KMS secret endpoint.
type kmsSecretResponse struct {
	Data struct {
		Value string `json:"value"`
	} `json:"data"`
}

// LoadKey loads a secp256k1 private key for P-chain operations.
// Priority: KMS secret -> PRIVATE_KEY env -> MNEMONIC env.
func LoadKey(kmsURL, kmsToken, secretName string) (*secp256k1.PrivateKey, error) {
	// 1. KMS
	if kmsURL != "" {
		return loadKeyFromKMS(kmsURL, kmsToken, secretName)
	}

	// 2. Hex private key
	if keyHex := os.Getenv("PRIVATE_KEY"); keyHex != "" {
		return parseHexKey(keyHex)
	}

	// 3. BIP39 mnemonic
	mnemonic := os.Getenv("MNEMONIC")
	if mnemonic == "" {
		return nil, fmt.Errorf("set KMS_URL, PRIVATE_KEY, or MNEMONIC")
	}
	return deriveKeyFromMnemonic(mnemonic)
}

// loadKeyFromKMS fetches a secret from Hanzo KMS and parses it as a key.
func loadKeyFromKMS(kmsURL, kmsToken, secretName string) (*secp256k1.PrivateKey, error) {
	if kmsToken == "" {
		return nil, fmt.Errorf("KMS_TOKEN required when KMS_URL is set")
	}
	if secretName == "" {
		return nil, fmt.Errorf("KMS secret name is empty")
	}

	url := strings.TrimRight(kmsURL, "/") + "/v1/secret/data/" + secretName

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("KMS request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+kmsToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("KMS fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("KMS returned %d: %s", resp.StatusCode, string(body))
	}

	var kmsResp kmsSecretResponse
	if err := json.NewDecoder(resp.Body).Decode(&kmsResp); err != nil {
		return nil, fmt.Errorf("KMS decode: %w", err)
	}

	secretValue := kmsResp.Data.Value
	if secretValue == "" {
		return nil, fmt.Errorf("KMS secret %q is empty", secretName)
	}

	// Zero after use.
	defer func() {
		b := []byte(secretValue)
		for i := range b {
			b[i] = 0
		}
	}()

	// Try hex key first, then mnemonic.
	if key, err := parseHexKey(secretValue); err == nil {
		return key, nil
	}
	return deriveKeyFromMnemonic(secretValue)
}

// parseHexKey parses a hex-encoded secp256k1 private key.
func parseHexKey(keyHex string) (*secp256k1.PrivateKey, error) {
	keyHex = strings.TrimPrefix(keyHex, "0x")
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decode hex key: %w", err)
	}
	defer func() {
		for i := range keyBytes {
			keyBytes[i] = 0
		}
	}()

	privKey, err := secp256k1.ToPrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return privKey, nil
}

// deriveKeyFromMnemonic derives a secp256k1 key from a BIP39 mnemonic.
// Path: m/44'/{coinType}'/0'/0/{index}
func deriveKeyFromMnemonic(mnemonic string) (*secp256k1.PrivateKey, error) {
	mnemonic = strings.TrimSpace(mnemonic)
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid BIP39 mnemonic")
	}

	seed := bip39.NewSeed(mnemonic, "")
	defer func() {
		for i := range seed {
			seed[i] = 0
		}
	}()

	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("BIP32 master key: %w", err)
	}
	defer func() {
		for i := range masterKey.Key {
			masterKey.Key[i] = 0
		}
	}()

	coinType := uint32(9000)
	keyIndex := uint32(1)

	if v := os.Getenv("COIN_TYPE"); v != "" {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid COIN_TYPE: %w", err)
		}
		coinType = uint32(n)
	}
	if v := os.Getenv("KEY_INDEX"); v != "" {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid KEY_INDEX: %w", err)
		}
		keyIndex = uint32(n)
	}

	// BIP44: m/44'/{coinType}'/0'/0/{index}
	// Each intermediate key is zeroed after deriving the next.
	purpose, err := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		return nil, fmt.Errorf("BIP32 derivation purpose: %w", err)
	}

	coin, err := purpose.NewChildKey(bip32.FirstHardenedChild + coinType)
	zeroKey(purpose)
	if err != nil {
		return nil, fmt.Errorf("BIP32 derivation coin type: %w", err)
	}

	account, err := coin.NewChildKey(bip32.FirstHardenedChild + 0)
	zeroKey(coin)
	if err != nil {
		return nil, fmt.Errorf("BIP32 derivation account: %w", err)
	}

	change, err := account.NewChildKey(0)
	zeroKey(account)
	if err != nil {
		return nil, fmt.Errorf("BIP32 derivation change: %w", err)
	}

	child, err := change.NewChildKey(keyIndex)
	zeroKey(change)
	if err != nil {
		return nil, fmt.Errorf("BIP32 derivation index: %w", err)
	}

	keyBytes := make([]byte, len(child.Key))
	copy(keyBytes, child.Key)
	zeroKey(child)

	defer func() {
		for i := range keyBytes {
			keyBytes[i] = 0
		}
	}()

	privKey, err := secp256k1.ToPrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("derived key invalid: %w", err)
	}
	return privKey, nil
}

// zeroKey zeroes the key material in a BIP32 key.
func zeroKey(k *bip32.Key) {
	if k == nil {
		return
	}
	for i := range k.Key {
		k.Key[i] = 0
	}
}
