package main

import (
	"errors"
	"os"

	"github.com/neilfarmer/k8s-health/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		if errors.Is(err, cmd.ErrIssuesFound) {
			os.Exit(1)
		}
		os.Exit(2)
	}
}
