package checks

import (
	"context"
	"testing"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

type fakeCheck struct {
	id   string
	cats []Category
	caps Capabilities
}

func (f *fakeCheck) ID() string                                          { return f.id }
func (f *fakeCheck) Description() string                                 { return "fake " + f.id }
func (f *fakeCheck) Categories() []Category                              { return f.cats }
func (f *fakeCheck) Requires() Capabilities                              { return f.caps }
func (f *fakeCheck) Run(_ context.Context, _ *kube.Env) []result.Finding { return nil }

func TestCapabilitiesHas(t *testing.T) {
	t.Parallel()
	combined := CapAPIServer | CapInCluster
	if !combined.Has(CapAPIServer) {
		t.Fatal("expected CapAPIServer present")
	}
	if !combined.Has(CapInCluster) {
		t.Fatal("expected CapInCluster present")
	}
	if combined.Has(Capabilities(1 << 5)) {
		t.Fatal("did not expect bit 5")
	}
}

func TestRegistryFilter(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(&fakeCheck{id: "a", cats: []Category{CategoryWorkload}})
	r.Register(&fakeCheck{id: "b", cats: []Category{CategoryNode}})
	r.Register(&fakeCheck{id: "c", cats: []Category{CategoryWorkload, CategoryEvents}})

	cases := []struct {
		name    string
		include []string
		exclude []string
		cats    []Category
		wantIDs []string
	}{
		{"all", nil, nil, nil, []string{"a", "b", "c"}},
		{"include a", []string{"a"}, nil, nil, []string{"a"}},
		{"exclude b", nil, []string{"b"}, nil, []string{"a", "c"}},
		{"workload category", nil, nil, []Category{CategoryWorkload}, []string{"a", "c"}},
		{"events category", nil, nil, []Category{CategoryEvents}, []string{"c"}},
		{"include + cat conflict", []string{"a"}, nil, []Category{CategoryEvents}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := r.Filter(tc.include, tc.exclude, tc.cats)
			gotIDs := make([]string, len(got))
			for i, c := range got {
				gotIDs[i] = c.ID()
			}
			if !equalStringSlice(gotIDs, tc.wantIDs) {
				t.Fatalf("got %v want %v", gotIDs, tc.wantIDs)
			}
		})
	}
}

func TestRegistryDuplicateIDPanics(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(&fakeCheck{id: "dup"})
	defer func() {
		if rv := recover(); rv == nil {
			t.Fatal("expected panic on duplicate ID")
		}
	}()
	r.Register(&fakeCheck{id: "dup"})
}

func TestDefaultRegistryAccessors(t *testing.T) {
	t.Parallel()
	if len(All()) == 0 {
		t.Fatal("expected production checks registered in Default")
	}
	if got := Filter(nil, nil, []Category{CategoryWorkload}); len(got) == 0 {
		t.Fatal("expected at least one workload check registered")
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
