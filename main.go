// Copyright (C) 2026, Zoo Labs Foundation. All rights reserved.
package main

import (
	"fmt"
	"os"

	"github.com/zooai/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
