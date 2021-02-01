package main

import (
	"os"

	"github.com/cybozu-go/moco-agent/cmd/moco-agent/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
