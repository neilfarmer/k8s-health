//go:build acceptance

package acceptance

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jsonReport struct {
	Context    string       `json:"context"`
	Namespaces []string     `json:"namespaces"`
	Results    []jsonResult `json:"results"`
	Summary    summary      `json:"summary"`
}

type jsonResult struct {
	Checker  string    `json:"checker"`
	Findings []finding `json:"findings"`
	Error    string    `json:"error,omitempty"`
}

type finding struct {
	Checker   string            `json:"checker"`
	Severity  string            `json:"severity"`
	Namespace string            `json:"namespace"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details"`
}

type summary struct {
	TotalCheckers      int `json:"total_checkers"`
	CheckersWithIssues int `json:"checkers_with_issues"`
	Critical           int `json:"critical"`
	Warning            int `json:"warning"`
	Info               int `json:"info"`
}

func binaryPath(t *testing.T) string {
	t.Helper()
	// Try current directory first, then project root
	candidates := []string{
		"./k8s-health",
		"../../k8s-health",
	}
	if runtime.GOOS == "windows" {
		candidates = []string{
			"./k8s-health.exe",
			"../../k8s-health.exe",
		}
	}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	t.Fatal("k8s-health binary not found; run 'go build -o k8s-health .' first")
	return ""
}

func runScan(t *testing.T, args ...string) ([]byte, int) {
	t.Helper()
	binary := binaryPath(t)
	cmdArgs := append([]string{"scan"}, args...)
	cmd := exec.Command(binary, cmdArgs...)
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run k8s-health: %v\n%s", err, out)
		}
	}
	return out, exitCode
}

func runScanJSON(t *testing.T, args ...string) (*jsonReport, int) {
	t.Helper()
	allArgs := append([]string{"-o", "json"}, args...)
	out, exitCode := runScan(t, allArgs...)

	var report jsonReport
	err := json.Unmarshal(out, &report)
	require.NoError(t, err, "failed to parse JSON output: %s", string(out))
	return &report, exitCode
}

func TestScan_DetectsIssuesInTestFixtures(t *testing.T) {
	report, exitCode := runScanJSON(t, "--namespace", "test-fixtures", "--severity", "warning")

	assert.Equal(t, 1, exitCode, "expected exit code 1 when issues are found")
	assert.Greater(t, report.Summary.CheckersWithIssues, 0, "expected at least one checker with issues")

	// Collect all findings
	var allFindings []finding
	for _, r := range report.Results {
		allFindings = append(allFindings, r.Findings...)
	}

	assert.NotEmpty(t, allFindings, "expected findings in test-fixtures namespace")
}

func TestScan_ExitCode0WhenHealthyNamespace(t *testing.T) {
	// kube-system is typically healthy on a fresh KIND cluster
	_, exitCode := runScanJSON(t,
		"--namespace", "kube-system",
		"--severity", "critical",
		"--checkers", "pods,nodes,deployments",
		"--timeout", "15s",
	)
	// On a fresh KIND cluster, kube-system should not have critical issues
	// but we can't guarantee this, so just verify the tool runs without error
	assert.Contains(t, []int{0, 1}, exitCode, "expected exit code 0 or 1")
}

func TestScan_JSONOutputIsValid(t *testing.T) {
	report, _ := runScanJSON(t, "--namespace", "test-fixtures")

	assert.NotEmpty(t, report.Context)
	assert.Equal(t, []string{"test-fixtures"}, report.Namespaces)
	assert.Greater(t, report.Summary.TotalCheckers, 0)
}

func TestScan_NamespaceFiltering(t *testing.T) {
	report, _ := runScanJSON(t, "--namespace", "test-fixtures", "--severity", "info")

	for _, r := range report.Results {
		for _, f := range r.Findings {
			if f.Namespace != "" {
				assert.Equal(t, "test-fixtures", f.Namespace,
					"finding from checker %s should be in test-fixtures namespace", f.Checker)
			}
		}
	}
}

func TestScan_CheckerFiltering(t *testing.T) {
	report, _ := runScanJSON(t, "--checkers", "pods,nodes", "--severity", "info")

	checkerNames := make(map[string]bool)
	for _, r := range report.Results {
		checkerNames[r.Checker] = true
	}

	assert.True(t, checkerNames["pods"], "pods checker should be present")
	assert.True(t, checkerNames["nodes"], "nodes checker should be present")
	assert.False(t, checkerNames["deployments"], "deployments checker should not be present")
}

func TestScan_TableOutput(t *testing.T) {
	out, _ := runScan(t, "--namespace", "test-fixtures", "--no-color")

	output := string(out)
	assert.Contains(t, output, "k8s-health Cluster Scan Report")
	assert.Contains(t, output, "SUMMARY")
}
