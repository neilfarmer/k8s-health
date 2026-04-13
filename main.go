package main

import (
	"os"

	"github.com/neilfarmer/k8s-health/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(2)
	}
}
