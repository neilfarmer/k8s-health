package checker

// Registry holds all registered checkers.
type Registry struct {
	checkers []Checker
}

// NewRegistry creates an empty checker registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a checker to the registry.
func (r *Registry) Register(c Checker) {
	r.checkers = append(r.checkers, c)
}

// All returns all registered checkers.
func (r *Registry) All() []Checker {
	return r.checkers
}

// Get returns a checker by name.
func (r *Registry) Get(name string) (Checker, bool) {
	for _, c := range r.checkers {
		if c.Name() == name {
			return c, true
		}
	}
	return nil, false
}

// Names returns all registered checker names.
func (r *Registry) Names() []string {
	names := make([]string, len(r.checkers))
	for i, c := range r.checkers {
		names[i] = c.Name()
	}
	return names
}

// DefaultRegistry returns a registry with all built-in checkers.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&PodChecker{})
	r.Register(&NodeChecker{})
	r.Register(&DeploymentChecker{})
	r.Register(&DaemonSetChecker{})
	r.Register(&StatefulSetChecker{})
	r.Register(&JobChecker{})
	r.Register(&CRDChecker{})
	r.Register(&HelmChecker{})
	r.Register(&PVCChecker{})
	r.Register(&ServiceChecker{})
	r.Register(&IngressChecker{})
	r.Register(&EventChecker{})
	return r
}
