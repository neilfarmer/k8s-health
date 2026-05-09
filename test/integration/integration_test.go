//go:build integration

// Package integration exercises the built khealth binary against a real
// Kubernetes cluster (a kind cluster, in CI). The KHEALTH_BIN environment
// variable points at the binary built by `make build` or goreleaser.
//
// At this stage the tests only validate the CLI surface end-to-end. As real
// checks land, additional cases will assert the report contents against a
// known-good cluster state.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func khealthBin(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("KHEALTH_BIN")
	if bin == "" {
		// Fall back to a sibling dist/khealth so `go test` works locally
		// after `make build`.
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}
		guess := filepath.Join(wd, "..", "..", "dist", "khealth")
		if _, err := os.Stat(guess); err == nil {
			return guess
		}
		t.Skip("KHEALTH_BIN not set and dist/khealth not found; skipping")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("KHEALTH_BIN=%q: %v", bin, err)
	}
	return bin
}

func run(t *testing.T, ctx context.Context, bin string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestVersion(t *testing.T) {
	bin := khealthBin(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdout, _, err := run(t, ctx, bin, "version", "--json")
	if err != nil {
		t.Fatalf("version: %v\n%s", err, stdout)
	}
	var info struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Date    string `json:"date"`
	}
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("parsing version JSON: %v\n%s", err, stdout)
	}
	if info.Version == "" {
		t.Fatalf("expected non-empty version: %+v", info)
	}
}

// TestCheckClusterStub asserts the scaffolded `check cluster` command runs
// against a real kubeconfig without crashing. Once real checks land this test
// will assert the JSON report shape instead of a stub message.
func TestCheckClusterStub(t *testing.T) {
	bin := khealthBin(t)
	if os.Getenv("KUBECONFIG") == "" {
		if home, _ := os.UserHomeDir(); home != "" {
			if _, err := os.Stat(filepath.Join(home, ".kube", "config")); errors.Is(err, os.ErrNotExist) {
				t.Skip("no kubeconfig found; integration cluster is required")
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, stderr, err := run(t, ctx, bin, "check", "cluster", "--output", "json")
	if err != nil {
		t.Fatalf("check cluster: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "not yet implemented") {
		t.Fatalf("scaffolding output unexpected: %q", stdout)
	}
}
