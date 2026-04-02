// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package networkcmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	localMode  bool
	k8sCluster string
)

const lightMnemonic = "light light light light light light light light light light light energy"

func NewNetworkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage Zoo network (start, stop, status)",
	}
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newStatusCmd())
	return cmd
}

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a Zoo network",
		Long: `Start a Zoo network via the operator (K8s native).

  zoo network start --local     3-node localnet on colima/k3s (light mnemonic)
  zoo network start --devnet    Deploy to Zoo devnet (GKE)
  zoo network start --testnet   Deploy to Zoo testnet (GKE)
  zoo network start --mainnet   Deploy to Zoo mainnet (GKE)`,
		RunE: startNetwork,
	}
	cmd.Flags().BoolVarP(&localMode, "local", "l", false, "start 3-node localnet on K8s (operator-native)")
	cmd.Flags().StringVar(&k8sCluster, "k8s", "", "K8s context (default: colima for --local)")
	return cmd
}

func startNetwork(cmd *cobra.Command, args []string) error {
	if !localMode {
		return fmt.Errorf("please specify --local (other modes coming soon)")
	}

	ctx := k8sCluster
	if ctx == "" {
		ctx = "colima"
	}

	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║  Zoo Labs — Localnet (3 nodes, K8s)          ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println("")

	if err := checkK8s(ctx); err != nil {
		return fmt.Errorf("K8s not available (context: %s): %w", ctx, err)
	}
	fmt.Printf("K8s context: %s\n", ctx)

	home, _ := os.UserHomeDir()
	operatorDir := filepath.Join(home, "work", "zoo", "operator", "k8s")

	// Apply operator CRDs
	fmt.Println("-> Operator CRDs")
	crds, _ := filepath.Glob(filepath.Join(operatorDir, "crds", "*.yaml"))
	for _, crd := range crds {
		kubectl(ctx, "apply", "-f", crd)
	}

	// Apply RBAC + deployment
	fmt.Println("-> Operator")
	kubectl(ctx, "apply",
		"-f", filepath.Join(operatorDir, "rbac", "serviceaccount.yaml"),
		"-f", filepath.Join(operatorDir, "rbac", "clusterrole.yaml"),
		"-f", filepath.Join(operatorDir, "rbac", "clusterrolebinding.yaml"),
		"-f", filepath.Join(operatorDir, "deployment.yaml"))

	// Apply network + platform
	fmt.Println("-> Network + Platform")
	for _, f := range []string{"networks/devnet.yaml", "platforms/devnet.yaml"} {
		p := filepath.Join(operatorDir, f)
		if _, err := os.Stat(p); err == nil {
			kubectl(ctx, "apply", "-f", p)
		}
	}

	fmt.Println("")
	fmt.Println("Zoo localnet deploying via operator.")
	fmt.Println("  Status: zoo network status")
	fmt.Println("  Stop:   zoo network stop")
	return nil
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Zoo network",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := k8sCluster
			if ctx == "" {
				ctx = "colima"
			}
			fmt.Println("Stopping Zoo network...")
			kubectl(ctx, "delete", "namespace", "zoo", "--ignore-not-found")
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Zoo network status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := k8sCluster
			if ctx == "" {
				ctx = "colima"
			}
			kubectl(ctx, "get", "pods", "--all-namespaces", "-l", "app.kubernetes.io/part-of=zoo")
			return nil
		},
	}
}

func checkK8s(ctx string) error {
	cmd := exec.Command("kubectl", "--context", ctx, "cluster-info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func kubectl(ctx string, args ...string) {
	fullArgs := append([]string{"--context", ctx}, args...)
	cmd := exec.Command("kubectl", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
