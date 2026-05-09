package checks

import (
	"context"
	"errors"
	"fmt"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&schedulerHealthz{}) }

const schedulerSecurePort = 10259

type schedulerHealthz struct{}

func (schedulerHealthz) ID() string             { return "scheduler.healthz" }
func (schedulerHealthz) Description() string    { return "kube-scheduler /healthz returns ok" }
func (schedulerHealthz) Categories() []Category { return []Category{CategoryControlPlane} }
func (schedulerHealthz) Requires() Capabilities { return CapAPIServer }

func (c schedulerHealthz) Run(ctx context.Context, env *kube.Env) []result.Finding {
	body, err := controlPlanePodHealthz(ctx, env, "kube-system",
		"component=kube-scheduler", "https", schedulerSecurePort)
	if errors.Is(err, errControlPlaneNotFound) {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusSkipped,
			Message: "kube-scheduler pod not found in kube-system (managed cluster?)",
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
	return []result.Finding{okFinding(c.ID(), "kube-scheduler /healthz=ok")}
}
