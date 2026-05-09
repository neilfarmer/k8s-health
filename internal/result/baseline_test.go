package result_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestSaveAndLoadBaseline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "base.json")

	rep := result.Report{
		Findings: []result.Finding{
			{Check: "a", Status: result.StatusOK, Resource: "r1"},
			{Check: "b", Status: result.StatusWarning, Resource: "r2"},
		},
	}
	if err := result.SaveBaseline(path, rep); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := result.LoadBaseline(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Findings) != 2 {
		t.Fatalf("want 2 findings, got %d", len(loaded.Findings))
	}
}

func TestLoadBaselineEmptyPath(t *testing.T) {
	t.Parallel()
	b, err := result.LoadBaseline("")
	if err != nil || b != nil {
		t.Fatalf("want nil,nil; got %v,%v", b, err)
	}
}

func TestLoadBaselineFromReportShape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "rep.json")
	// A full Report JSON, not just Baseline.
	if err := os.WriteFile(path, []byte(`{"findings":[{"check":"a","status":"OK"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := result.LoadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Findings) != 1 {
		t.Fatalf("want 1 finding, got %d", len(b.Findings))
	}
}

func TestCompareDiff(t *testing.T) {
	t.Parallel()
	base := []result.Finding{
		{Check: "a", Status: result.StatusOK, Resource: "r1"},
		{Check: "b", Status: result.StatusWarning, Resource: "r2"},
		{Check: "c", Status: result.StatusCritical, Resource: "r3"},
	}
	cur := []result.Finding{
		{Check: "a", Status: result.StatusOK, Resource: "r1"},        // persisting
		{Check: "b", Status: result.StatusWarning, Resource: "r2"},   // persisting
		{Check: "d", Status: result.StatusCritical, Resource: "r4"}, // new
	}
	d := result.Compare(base, cur)
	if len(d.Persisting) != 2 {
		t.Errorf("persisting: want 2, got %d", len(d.Persisting))
	}
	if len(d.New) != 1 {
		t.Errorf("new: want 1, got %d", len(d.New))
	}
	if len(d.Cleared) != 1 {
		t.Errorf("cleared: want 1, got %d", len(d.Cleared))
	}
}
