package checks

import "testing"

// kubelet.healthz uses the apiserver's nodes/{name}/proxy subresource which
// fake clientset's CoreV1().RESTClient() doesn't usefully implement. The
// real coverage comes from the integration test against kind. Here we just
// touch the metadata accessors.
func TestKubeletHealthzMetadata(t *testing.T) {
	t.Parallel()
	c := kubeletHealthz{}
	if c.ID() != "kubelet.healthz" {
		t.Errorf("ID = %q", c.ID())
	}
	if c.Description() == "" {
		t.Error("empty description")
	}
	if !c.Requires().Has(CapAPIServer) {
		t.Error("requires CapAPIServer")
	}
	if len(c.Categories()) == 0 {
		t.Error("no categories")
	}
}
