package checks

import (
	"context"
	"errors"
	"fmt"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&controllerMgrHealthz{}) }

const controllerMgrSecurePort = 10257

type controllerMgrHealthz struct{}

func (controllerMgrHealthz) ID() string             { return "controllerMgr.healthz" }
func (controllerMgrHealthz) Description() string    { return "kube-controller-manager /healthz returns ok" }
func (controllerMgrHealthz) Categories() []Category { return []Category{CategoryControlPlane} }
func (controllerMgrHealthz) Requires() Capabilities { return CapAPIServer }

func (c controllerMgrHealthz) Run(ctx context.Context, env *kube.Env) []result.Finding {
	body, err := controlPlanePodHealthz(ctx, env, "kube-system",
		"component=kube-controller-manager", "https", controllerMgrSecurePort)
	if errors.Is(err, errControlPlaneNotFound) {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusSkipped,
			Message: "kube-controller-manager pod not found in kube-system (managed cluster?)",
		}}
	}
	if err != nil {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusCritical,
			Message: fmt.Sprintf("/healthz: %v", err),
		}}
	}
	if body != "ok" {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusWarning,
			Message: fmt.Sprintf("/healthz body: %q", body),
		}}
	}
	return []result.Finding{okFinding(c.ID(), "kube-controller-manager /healthz=ok")}
}
