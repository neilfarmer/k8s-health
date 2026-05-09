// Package checks defines the Check interface, a small registry, and one
// file per check. Each check registers itself in init() so adding a new
// check is one file plus one Register() call (per ADR-0004).
package checks

import (
	"context"
	"sort"
	"sync"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

// Category groups checks for CLI subcommands (`khealth check pods`,
// `check nodes`, etc.) and for filtering reports.
type Category string

// Defined categories. New categories should be added here and reflected in
// the corresponding `khealth check <subcommand>` mapping in internal/cli.
const (
	CategoryWorkload     Category = "workload"
	CategoryNode         Category = "node"
	CategoryStorage      Category = "storage"
	CategoryNetwork      Category = "network"
	CategoryControlPlane Category = "controlplane"
	CategoryEvents       Category = "events"
)

// Capabilities is a bitmask of runtime requirements a Check declares. The
// runner skips checks whose required capabilities aren't present in the
// current environment.
type Capabilities uint32

// Defined capabilities. CapAPIServer is the baseline (every Phase-1 check
// needs it). CapInCluster is set by the runner when khealth runs as an
// in-cluster Job and unlocks checks that require host-level paths.
const (
	CapAPIServer Capabilities = 1 << iota
	CapInCluster
)

// Has reports whether c includes all of want.
func (c Capabilities) Has(want Capabilities) bool { return c&want == want }

// Check is the interface every concrete check satisfies.
type Check interface {
	ID() string
	Description() string
	Categories() []Category
	Requires() Capabilities
	Run(ctx context.Context, env *kube.Env) []result.Finding
}

// Registry holds a set of Checks. The package-level Register / All /
// Filter helpers operate on a default *Registry; tests can construct a
// fresh one via NewRegistry to avoid touching global state.
type Registry struct {
	mu sync.RWMutex
	m  map[string]Check
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{m: map[string]Check{}} }

// Register adds a check. Panics on duplicate IDs.
func (r *Registry) Register(c Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.m[c.ID()]; dup {
		panic("checks: duplicate id: " + c.ID())
	}
	r.m[c.ID()] = c
}

// All returns every registered check, sorted by ID for stable output.
func (r *Registry) All() []Check {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Check, 0, len(r.m))
	for _, c := range r.m {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// Filter returns checks matching include (if non-empty) and not in exclude.
// cats filters to checks that overlap with at least one of the given
// categories; nil or empty cats means "all categories". distro gates
// DistroAware checks; pass DistroAuto or "" to disable distro filtering
// (every check, including distro-specific ones, is allowed through).
func (r *Registry) Filter(include, exclude []string, cats []Category, distro Distro) []Check {
	includeSet := stringSet(include)
	excludeSet := stringSet(exclude)
	catSet := map[Category]struct{}{}
	for _, c := range cats {
		catSet[c] = struct{}{}
	}
	all := r.All()
	out := make([]Check, 0, len(all))
	for _, c := range all {
		if _, skip := excludeSet[c.ID()]; skip {
			continue
		}
		if len(includeSet) > 0 {
			if _, ok := includeSet[c.ID()]; !ok {
				continue
			}
		}
		if len(catSet) > 0 && !categoryMatch(c, catSet) {
			continue
		}
		if distro != "" && distro != DistroAuto && !AppliesToDistro(c, distro) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// Default is the package-level registry that init() functions populate.
var Default = NewRegistry()

// Register adds c to the default registry.
func Register(c Check) { Default.Register(c) }

// All returns every check in the default registry.
func All() []Check { return Default.All() }

// Filter is shorthand for Default.Filter.
func Filter(include, exclude []string, cats []Category, distro Distro) []Check {
	return Default.Filter(include, exclude, cats, distro)
}

func stringSet(in []string) map[string]struct{} {
	m := make(map[string]struct{}, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		m[s] = struct{}{}
	}
	return m
}

func categoryMatch(c Check, want map[Category]struct{}) bool {
	for _, cat := range c.Categories() {
		if _, ok := want[cat]; ok {
			return true
		}
	}
	return false
}
