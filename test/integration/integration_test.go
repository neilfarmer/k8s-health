//go:build integration

// Package integration exercises the built khealth binary against a real
// Kubernetes cluster (a kind cluster, in CI). The KHEALTH_BIN environment
// variable points at the binary built by `make build` or goreleaser.
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

func requireKubeconfig(t *testing.T) {
	t.Helper()
	if os.Getenv("KUBECONFIG") != "" {
		return
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no kubeconfig and no HOME; skipping")
	}
	if _, err := os.Stat(filepath.Join(home, ".kube", "config")); errors.Is(err, os.ErrNotExist) {
		t.Skip("no kubeconfig found; integration cluster is required")
	}
}

type runResult struct {
	stdout string
	stderr string
	err    error
}

func runBin(ctx context.Context, bin string, args ...string) runResult {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return runResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

type khealthReport struct {
	GeneratedAt string `json:"generatedAt"`
	Cluster     string `json:"cluster"`
	Findings    []struct {
		Check    string `json:"check"`
		Status   string `json:"status"`
		Resource string `json:"resource"`
		Message  string `json:"message"`
	} `json:"findings"`
}

func parseReport(t *testing.T, out string) khealthReport {
	t.Helper()
	var r khealthReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("parsing JSON report: %v\n%s", err, out)
	}
	return r
}

func TestVersion(t *testing.T) {
	bin := khealthBin(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res := runBin(ctx, bin, "version", "--json")
	if res.err != nil {
		t.Fatalf("version: %v\n%s", res.err, res.stdout)
	}
	var info struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &info); err != nil {
		t.Fatalf("parsing version JSON: %v\n%s", err, res.stdout)
	}
	if info.Version == "" {
		t.Fatalf("expected non-empty version: %+v", info)
	}
}

// TestCheckNodesAgainstKind asserts that the nodes.ready check reports OK
// against a freshly-built kind cluster.
func TestCheckNodesAgainstKind(t *testing.T) {
	requireKubeconfig(t)
	bin := khealthBin(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// nodes.ready should be OK on a healthy kind cluster. Even if the run
	// returns exit-code 1 because of unrelated WARNs (events, etc.), the
	// JSON report still gets written before exit.
	res := runBin(ctx, bin, "check", "nodes", "--output", "json", "--checks", "nodes.ready")
	if res.err != nil {
		// findings-driven non-zero exit is OK for this assertion as long
		// as the report parses; we still want to inspect the body.
		if !isFindingsExit(res) {
			t.Fatalf("check nodes failed: %v\nstderr: %s", res.err, res.stderr)
		}
	}
	rep := parseReport(t, res.stdout)
	if len(rep.Findings) == 0 {
		t.Fatalf("expected at least one finding; got %s", res.stdout)
	}
	for _, f := range rep.Findings {
		if f.Check != "nodes.ready" {
			continue
		}
		if f.Status != "OK" {
			t.Errorf("nodes.ready status=%q want OK; msg=%q", f.Status, f.Message)
		}
	}
}

// TestCheckPodsBackoffAgainstKind injects a CrashLoopBackOff pod, runs
// pods.backoff, and asserts a CRIT finding pointing at it.
func TestCheckPodsBackoffAgainstKind(t *testing.T) {
	requireKubeconfig(t)
	bin := khealthBin(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ns := "khealth-it-backoff"
	if err := kubectl(ctx, "create", "namespace", ns); err != nil {
		t.Skipf("kubectl create ns failed (kind not available?): %v", err)
	}
	t.Cleanup(func() {
		// best effort
		clean, c := context.WithTimeout(context.Background(), 30*time.Second)
		defer c()
		_ = kubectl(clean, "delete", "namespace", ns, "--wait=false", "--ignore-not-found")
	})

	pod := []byte(`apiVersion: v1
kind: Pod
metadata:
  name: bad
spec:
  restartPolicy: Always
  containers:
    - name: app
      image: busybox:1.36
      command: ["sh", "-c", "exit 1"]
`)
	if err := kubectlApply(ctx, ns, pod); err != nil {
		t.Fatalf("kubectl apply: %v", err)
	}

	// Wait for the pod to enter a backoff state.
	if err := waitForBackoff(ctx, ns, "bad"); err != nil {
		t.Fatalf("pod did not enter backoff: %v", err)
	}

	res := runBin(ctx, bin, "check", "pods",
		"--output", "json",
		"--checks", "pods.backoff",
		"--namespace", ns,
	)
	if res.err != nil && !isFindingsExit(res) {
		t.Fatalf("check pods: %v\nstderr: %s", res.err, res.stderr)
	}
	rep := parseReport(t, res.stdout)

	var crit int
	for _, f := range rep.Findings {
		if f.Check == "pods.backoff" && f.Status == "CRIT" {
			crit++
		}
	}
	if crit == 0 {
		t.Fatalf("expected at least one CRIT pods.backoff finding, got %+v", rep.Findings)
	}
}

func isFindingsExit(r runResult) bool {
	if r.err == nil {
		return false
	}
	if exit, ok := r.err.(*exec.ExitError); ok {
		return exit.ExitCode() == 1 || exit.ExitCode() == 2
	}
	return false
}

func kubectl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func kubectlApply(ctx context.Context, namespace string, manifest []byte) error {
	cmd := exec.CommandContext(ctx, "kubectl", "-n", namespace, "apply", "-f", "-")
	cmd.Stdin = bytes.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitForBackoff(ctx context.Context, namespace, name string) error {
	deadline, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	for {
		out, err := exec.CommandContext(deadline, "kubectl", "-n", namespace, "get", "pod", name,
			"-o", "jsonpath={.status.containerStatuses[0].state.waiting.reason}").CombinedOutput()
		if err == nil {
			reason := strings.TrimSpace(string(out))
			if reason == "CrashLoopBackOff" || reason == "ImagePullBackOff" || reason == "ErrImagePull" {
				return nil
			}
		}
		select {
		case <-deadline.Done():
			return errors.New("timeout waiting for backoff state")
		case <-time.After(5 * time.Second):
		}
	}
}
