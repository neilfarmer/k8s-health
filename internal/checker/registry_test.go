package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	r.Register(&PodChecker{})
	r.Register(&NodeChecker{})

	assert.Len(t, r.All(), 2)
	assert.Equal(t, []string{"pods", "nodes"}, r.Names())
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	r.Register(&PodChecker{})

	c, ok := r.Get("pods")
	require.True(t, ok)
	assert.Equal(t, "pods", c.Name())

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)
}

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()
	assert.NotEmpty(t, r.All())

	// Verify all expected checkers are registered
	expectedNames := []string{"pods", "nodes", "deployments", "daemonsets", "statefulsets", "jobs", "crds", "helm", "pvcs", "services", "ingresses", "events"}
	for _, name := range expectedNames {
		_, ok := r.Get(name)
		assert.True(t, ok, "expected checker %q to be registered", name)
	}
}
