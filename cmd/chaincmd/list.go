// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package chaincmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured chains",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			chainsDir := filepath.Join(home, ".zoo", "chains")
			entries, err := os.ReadDir(chainsDir)
			if err != nil {
				fmt.Println("No chains configured. Create one: zoo chain create beluga --type=l3")
				return nil
			}
			fmt.Printf("%-15s %-5s %-10s %-8s %s\n", "NAME", "TYPE", "CHAIN ID", "TOKEN", "STATUS")
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				sc, err := os.ReadFile(filepath.Join(chainsDir, e.Name(), "sidecar.json"))
				if err != nil {
					continue
				}
				var s map[string]any
				json.Unmarshal(sc, &s)
				name, _ := s["name"].(string)
				typ, _ := s["type"].(string)
				cid := uint64(0)
				if v, ok := s["chainId"].(float64); ok {
					cid = uint64(v)
				}
				sym, _ := s["tokenSymbol"].(string)
				fmt.Printf("%-15s %-5s %-10d %-8s %s\n", name, typ, cid, sym, "configured")
			}
			return nil
		},
	}
}
