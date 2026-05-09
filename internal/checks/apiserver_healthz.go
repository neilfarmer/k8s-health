package checks

import (
	"context"
	"fmt"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&apiserverHealthz{}) }

type apiserverHealthz struct{}

func (apiserverHealthz) ID() string             { return "apiserver.healthz" }
func (apiserverHealthz) Description() string    { return "API server /readyz returns ok" }
func (apiserverHealthz) Categories() []Category { return []Category{CategoryControlPlane} }
func (apiserverHealthz) Requires() Capabilities { return CapAPIServer }

func (c apiserverHealthz) Run(ctx context.Context, env *kube.Env) []result.Finding {
	rest := env.Clientset.Discovery().RESTClient()
	if rest == nil {
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusSkipped,
			Message: "no REST client available (fake clientset)",
		}}
	}
	body, err := rest.Get().AbsPath("/readyz").DoRaw(ctx)
	if err != nil {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusCritical,
			Message: fmt.Sprintf("/readyz: %v", err),
			Detail:  map[string]string{"endpoint": "/readyz"},
		}}
	}
	if string(body) != "ok" {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusWarning,
			Message: fmt.Sprintf("/readyz body: %q", string(body)),
		}}
	}
	return []result.Finding{okFinding(c.ID(), "API server /readyz=ok")}
}
