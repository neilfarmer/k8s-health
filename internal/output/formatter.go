package output

import (
	"time"

	"github.com/neilfarmer/k8s-health/internal/checker"
)

// JSONReport is the top-level structure for JSON output.
type JSONReport struct {
	Timestamp  string        `json:"timestamp"`
	Context    string        `json:"context"`
	Namespaces []string      `json:"namespaces"`
	Results    []JSONResult  `json:"results"`
	Summary    ReportSummary `json:"summary"`
}

// JSONResult wraps checker results for JSON output.
type JSONResult struct {
	Checker    string            `json:"checker"`
	Findings   []checker.Finding `json:"findings"`
	Error      string            `json:"error,omitempty"`
	DurationMs int64             `json:"duration_ms"`
}

// ReportSummary provides aggregate statistics.
type ReportSummary struct {
	TotalCheckers      int `json:"total_checkers"`
	CheckersWithIssues int `json:"checkers_with_issues"`
	Critical           int `json:"critical"`
	Warning            int `json:"warning"`
	Info               int `json:"info"`
}

// BuildJSONReport constructs a JSONReport from checker results.
func BuildJSONReport(results []*checker.Result, contextName string, namespaces []string, minSeverity checker.Severity) *JSONReport {
	ns := namespaces
	if len(ns) == 0 {
		ns = []string{"all"}
	}

	report := &JSONReport{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Context:    contextName,
		Namespaces: ns,
	}

	for _, r := range results {
		jr := JSONResult{
			Checker:    r.CheckerName,
			DurationMs: r.Duration.Milliseconds(),
		}
		if r.Error != nil {
			jr.Error = r.Error.Error()
		}

		// Filter findings by severity
		for _, f := range r.Findings {
			if f.Severity >= minSeverity {
				jr.Findings = append(jr.Findings, f)
			}
		}
		if jr.Findings == nil {
			jr.Findings = []checker.Finding{}
		}

		report.Results = append(report.Results, jr)

		// Accumulate summary
		for _, f := range r.Findings {
			if f.Severity < minSeverity {
				continue
			}
			switch f.Severity {
			case checker.SeverityCritical:
				report.Summary.Critical++
			case checker.SeverityWarning:
				report.Summary.Warning++
			case checker.SeverityInfo:
				report.Summary.Info++
			}
		}
		if len(jr.Findings) > 0 || jr.Error != "" {
			report.Summary.CheckersWithIssues++
		}
	}

	report.Summary.TotalCheckers = len(results)
	return report
}
