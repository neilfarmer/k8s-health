package checks

import (
	"strings"
	"testing"
)

// TestRegisteredChecksMetadata exercises the trivial accessors on every
// registered check so the coverage gate doesn't punish boilerplate.
func TestRegisteredChecksMetadata(t *testing.T) {
	t.Parallel()
	all := All()
	if len(all) == 0 {
		t.Fatal("expected at least one registered check")
	}
	for _, c := range all {
		if c.ID() == "" {
			t.Errorf("%T returned empty ID", c)
		}
		if !strings.Contains(c.ID(), ".") {
			t.Errorf("ID %q expected dotted form (e.g. pods.backoff)", c.ID())
		}
		if c.Description() == "" {
			t.Errorf("%s missing description", c.ID())
		}
		if len(c.Categories()) == 0 {
			t.Errorf("%s declares no categories", c.ID())
		}
		if !c.Requires().Has(CapAPIServer) {
			t.Errorf("%s does not require CapAPIServer (Phase-1 baseline)", c.ID())
		}
	}
}
