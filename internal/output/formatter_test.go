package output

import (
	"testing"

	"github.com/neilfarmer/k8s-health/internal/checker"
	"github.com/stretchr/testify/assert"
)

func TestBuildJSONReport(t *testing.T) {
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
				{
					Checker:   "pods",
					Severity:  checker.SeverityWarning,
					Namespace: "default",
					Kind:      "Pod",
					Name:      "pending-pod",
					Message:   "Pod has been Pending",
				},
			},
		},
		{
			CheckerName: "nodes",
			Findings:    nil,
		},
	}

	report := BuildJSONReport(results, "test-context", nil, checker.SeverityWarning)

	assert.Equal(t, "test-context", report.Context)
	assert.Equal(t, []string{"all"}, report.Namespaces)
	assert.Equal(t, 2, report.Summary.TotalCheckers)
	assert.Equal(t, 1, report.Summary.CheckersWithIssues)
	assert.Equal(t, 1, report.Summary.Critical)
	assert.Equal(t, 1, report.Summary.Warning)
	assert.Equal(t, 0, report.Summary.Info)

	// Check findings filtered correctly
	assert.Len(t, report.Results[0].Findings, 2)
	assert.Empty(t, report.Results[1].Findings)
}

func TestBuildJSONReport_SeverityFilter(t *testing.T) {
	results := []*checker.Result{
		{
			CheckerName: "pods",
			Findings: []checker.Finding{
				{Severity: checker.SeverityCritical, Message: "critical"},
				{Severity: checker.SeverityWarning, Message: "warning"},
				{Severity: checker.SeverityInfo, Message: "info"},
			},
		},
	}

	report := BuildJSONReport(results, "ctx", nil, checker.SeverityCritical)

	assert.Len(t, report.Results[0].Findings, 1)
	assert.Equal(t, 1, report.Summary.Critical)
	assert.Equal(t, 0, report.Summary.Warning)
}

func TestBuildJSONReport_WithNamespaces(t *testing.T) {
	results := []*checker.Result{}
	report := BuildJSONReport(results, "ctx", []string{"ns1", "ns2"}, checker.SeverityWarning)

	assert.Equal(t, []string{"ns1", "ns2"}, report.Namespaces)
}
