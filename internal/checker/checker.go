package checker

import (
	"context"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Severity levels for findings.
type Severity int

const (
	SeverityInfo     Severity = iota
	SeverityWarning
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// MarshalText implements encoding.TextMarshaler for JSON output.
func (s Severity) MarshalText() ([]byte, error) {
	switch s {
	case SeverityInfo:
		return []byte("info"), nil
	case SeverityWarning:
		return []byte("warning"), nil
	case SeverityCritical:
		return []byte("critical"), nil
	default:
		return []byte("unknown"), nil
	}
}

// Finding represents a single issue discovered by a checker.
type Finding struct {
	Checker   string            `json:"checker"`
	Severity  Severity          `json:"severity"`
	Namespace string            `json:"namespace,omitempty"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details,omitempty"`
}

// Resource returns a "Kind/Name" string for display.
func (f Finding) Resource() string {
	return f.Kind + "/" + f.Name
}

// Result aggregates findings from a single checker run.
type Result struct {
	CheckerName string        `json:"checker_name"`
	Findings    []Finding     `json:"findings"`
	Error       error         `json:"-"`
	ErrorMsg    string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration_ms"`
}

// Checker is the interface all health checks implement.
type Checker interface {
	Name() string
	Description() string
	Check(ctx context.Context, opts CheckOptions) (*Result, error)
}

// CheckOptions provides shared configuration to all checkers.
type CheckOptions struct {
	Client     kubernetes.Interface
	DynClient  dynamic.Interface
	RestConfig *rest.Config
	Namespaces []string
}

// AllNamespaces returns true if no specific namespaces were requested.
func (o CheckOptions) AllNamespaces() bool {
	return len(o.Namespaces) == 0
}
