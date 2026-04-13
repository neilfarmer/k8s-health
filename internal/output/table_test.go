package output

import (
	"bytes"
	"testing"

	"github.com/fatih/color"
	"github.com/neilfarmer/k8s-health/internal/checker"
	"github.com/stretchr/testify/assert"
)

func init() {
	color.NoColor = true
}

func TestTableFormatter_RenderWithFindings(t *testing.T) {
	results := []*checker.Result{
		{
			CheckerName: "pods",
			Findings: []checker.Finding{
				{
					Checker:   "pods",
					Severity:  checker.SeverityCritical,
					Namespace: "default",
					Kind:      "Pod",
					Name:      "crash-pod",
					Message:   "Container is in CrashLoopBackOff",
				},
			},
		},
	}

	var buf bytes.Buffer
	f := &TableFormatter{NoColor: true, Verbose: false}
	f.Render(&buf, results, "test-ctx", nil, checker.SeverityWarning)

	output := buf.String()
	assert.Contains(t, output, "k8s-health Cluster Scan Report")
	assert.Contains(t, output, "test-ctx")
	assert.Contains(t, output, "Pods")
	assert.Contains(t, output, "1 issue")
	assert.Contains(t, output, "CRITICAL")
	assert.Contains(t, output, "CrashLoopBackOff")
	assert.Contains(t, output, "SUMMARY")
	assert.Contains(t, output, "Critical: 1")
}

func TestTableFormatter_RenderNoFindings(t *testing.T) {
	results := []*checker.Result{
		{CheckerName: "pods"},
		{CheckerName: "nodes"},
	}

	var buf bytes.Buffer
	f := &TableFormatter{NoColor: true, Verbose: false}
	f.Render(&buf, results, "test-ctx", []string{"default"}, checker.SeverityWarning)

	output := buf.String()
	assert.Contains(t, output, "SUMMARY")
	assert.Contains(t, output, "Checks with issues: 0")
	assert.NotContains(t, output, "CRITICAL")
}

func TestTableFormatter_RenderVerbose(t *testing.T) {
	results := []*checker.Result{
		{CheckerName: "pods"},
	}

	var buf bytes.Buffer
	f := &TableFormatter{NoColor: true, Verbose: true}
	f.Render(&buf, results, "test-ctx", nil, checker.SeverityWarning)

	output := buf.String()
	assert.Contains(t, output, "healthy")
}

func TestTableFormatter_RenderError(t *testing.T) {
	results := []*checker.Result{
		{
			CheckerName: "crds",
			Error:       assert.AnError,
		},
	}

	var buf bytes.Buffer
	f := &TableFormatter{NoColor: true, Verbose: false}
	f.Render(&buf, results, "test-ctx", nil, checker.SeverityWarning)

	output := buf.String()
	assert.Contains(t, output, "error")
}

func TestTitleCase(t *testing.T) {
	assert.Equal(t, "Pods", titleCase("pods"))
	assert.Equal(t, "Nodes", titleCase("nodes"))
	assert.Equal(t, "", titleCase(""))
}
