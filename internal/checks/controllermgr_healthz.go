package checks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&controllerMgrHealthz{}) }

const controllerMgrLeaseName = "kube-controller-manager"

type controllerMgrHealthz struct{}

func (controllerMgrHealthz) ID() string { return "controllerMgr.healthz" }
func (controllerMgrHealthz) Description() string {
	return "kube-controller-manager leader-election lease is fresh"
}
func (controllerMgrHealthz) Categories() []Category { return []Category{CategoryControlPlane} }
func (controllerMgrHealthz) Requires() Capabilities { return CapAPIServer }

func (c controllerMgrHealthz) Run(ctx context.Context, env *kube.Env) []result.Finding {
	now := nowFunc()
	h, err := controlPlaneLeaseHealth(ctx, env, "kube-system", controllerMgrLeaseName, now)
	if errors.Is(err, errControlPlaneLeaseNotFound) {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusSkipped,
			Message: "kube-controller-manager lease not found in kube-system (managed cluster?)",
		}}
	}
	if err != nil {
		return unknownFromErr(c.ID(), "lease", err)
	}
	if h.Holder == "" {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusCritical,
			Message: "kube-controller-manager lease has no holder (no leader elected)",
		}}
	}
	if h.Stale {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusCritical,
			Message: fmt.Sprintf("kube-controller-manager lease stale: holder=%s renewed %s ago (max %s)",
				h.Holder, now.Sub(h.RenewedAt).Round(time.Second), h.MaxAge),
		}}
	}
	return []result.Finding{okFinding(c.ID(),
		fmt.Sprintf("kube-controller-manager lease fresh (holder=%s, renewed %s ago)",
			h.Holder, now.Sub(h.RenewedAt).Round(time.Second)))}
}
