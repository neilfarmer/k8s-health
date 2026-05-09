// Package main is the khealth CLI entry point.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/neilfarmer/k8s-health/internal/cli"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.NewRootCmd().ExecuteContext(ctx); err != nil {
		var ec cli.ExitCoder
		if errors.As(err, &ec) {
			// findings-driven exit; the renderer already wrote the report,
			// no need to print the error again.
			return ec.Code()
		}
		fmt.Fprintln(os.Stderr, "khealth:", err)
		return 3
	}
	return 0
}
