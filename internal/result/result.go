// Package result defines the shared finding/report types emitted by checks
// and the test runner.
//
// The Kubernetes-aware code does not live here — this package is intentionally
// dependency-light so the CLI scaffolding can build and test independently of
// any cluster integration.
package result

import "time"

// Status is the severity bucket for a single Finding.
type Status string

// Status values, ordered from least to most severe for ranking purposes.
const (
	StatusOK       Status = "OK"
	StatusWarning  Status = "WARN"
	StatusCritical Status = "CRIT"
	StatusUnknown  Status = "UNKNOWN"
	StatusSkipped  Status = "SKIP"
)

// Finding is one outcome from one check or test step.
type Finding struct {
	Check    string            `json:"check"`
	Status   Status            `json:"status"`
	Resource string            `json:"resource,omitempty"`
	Message  string            `json:"message,omitempty"`
	Detail   map[string]string `json:"detail,omitempty"`
	Duration time.Duration     `json:"duration,omitempty"`
}

// Report is the full set of findings from a single khealth invocation.
type Report struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Cluster     string    `json:"cluster,omitempty"`
	Distro      string    `json:"distro,omitempty"`
	Findings    []Finding `json:"findings"`
}

// Worst returns the most severe status in r. If r has no findings, it returns
// StatusOK. SKIP and UNKNOWN are never "worse" than WARN/CRIT.
func (r Report) Worst() Status {
	worst := StatusOK
	for _, f := range r.Findings {
		if rank(f.Status) > rank(worst) {
			worst = f.Status
		}
	}
	return worst
}

func rank(s Status) int {
	switch s {
	case StatusCritical:
		return 3
	case StatusWarning:
		return 2
	case StatusUnknown:
		return 1
	case StatusOK, StatusSkipped:
		return 0
	}
	return 0
}

// ExitCode maps the worst status in r to a process exit code, per the
// contract documented in docs/adr/0006-output-formats.md.
//
// strict=true promotes WARN to a non-zero exit (2). CRIT always exits 1.
func (r Report) ExitCode(strict bool) int {
	switch r.Worst() {
	case StatusCritical:
		return 1
	case StatusWarning:
		if strict {
			return 2
		}
		return 0
	default:
		return 0
	}
}
