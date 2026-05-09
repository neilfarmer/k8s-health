package render_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/neilfarmer/k8s-health/internal/render"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func sampleReport() result.Report {
	return result.Report{
		GeneratedAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
		Cluster:     "kind",
		Findings: []result.Finding{
			{Check: "nodes.ready", Status: result.StatusOK, Message: "all nodes Ready"},
			{Check: "pods.backoff", Status: result.StatusCritical, Resource: "pod/api in ns/payments", Message: "CrashLoopBackOff"},
		},
	}
}

func TestNewKnownFormats(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "table", "json", "yaml"} {
		r, err := render.New(name)
		if err != nil {
			t.Errorf("render.New(%q) returned error: %v", name, err)
		}
		if r == nil {
			t.Errorf("render.New(%q) returned nil renderer", name)
		}
	}
}

func TestNewUnknownFormat(t *testing.T) {
	t.Parallel()
	if _, err := render.New("xml"); err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestTableRender(t *testing.T) {
	t.Parallel()
	r, _ := render.New("table")
	var buf bytes.Buffer
	if err := r.Render(&buf, sampleReport()); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "CLUSTER") {
		t.Errorf("missing CLUSTER header: %q", out)
	}
	if !strings.Contains(out, "CRIT") || !strings.Contains(out, "pods.backoff") {
		t.Errorf("missing CRIT row: %q", out)
	}
}

func TestJSONRender(t *testing.T) {
	t.Parallel()
	r, _ := render.New("json")
	var buf bytes.Buffer
	if err := r.Render(&buf, sampleReport()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), `"check": "pods.backoff"`) {
		t.Errorf("missing check field: %q", buf.String())
	}
}

func TestYAMLRender(t *testing.T) {
	t.Parallel()
	r, _ := render.New("yaml")
	var buf bytes.Buffer
	if err := r.Render(&buf, sampleReport()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "check: pods.backoff") {
		t.Errorf("missing yaml field: %q", buf.String())
	}
}

func TestEmptyReport(t *testing.T) {
	t.Parallel()
	r, _ := render.New("table")
	var buf bytes.Buffer
	if err := r.Render(&buf, result.Report{}); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "no findings") {
		t.Errorf("expected 'no findings' message: %q", buf.String())
	}
}

// Exercise table rendering for every status (covers rank() switch arms) and
// truncation for long resource strings.
func TestTableAllStatusesAndTruncation(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 200)
	rep := result.Report{
		Findings: []result.Finding{
			{Check: "a", Status: result.StatusOK},
			{Check: "b", Status: result.StatusWarning, Resource: long, Message: long},
			{Check: "c", Status: result.StatusCritical},
			{Check: "d", Status: result.StatusUnknown},
			{Check: "e", Status: result.StatusSkipped},
		},
	}
	r, _ := render.New("table")
	var buf bytes.Buffer
	if err := r.Render(&buf, rep); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"OK", "WARN", "CRIT", "UNKNOWN", "SKIP"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing status %q: %q", want, out)
		}
	}
	if !strings.Contains(out, "…") {
		t.Errorf("expected ellipsis from truncation: %q", out)
	}
}
