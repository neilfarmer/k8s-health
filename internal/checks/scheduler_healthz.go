package checks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&schedulerHealthz{}) }

const schedulerLeaseName = "kube-scheduler"

type schedulerHealthz struct{}

func (schedulerHealthz) ID() string { return "scheduler.healthz" }
func (schedulerHealthz) Description() string {
	return "kube-scheduler leader-election lease is fresh"
}
func (schedulerHealthz) Categories() []Category { return []Category{CategoryControlPlane} }
func (schedulerHealthz) Requires() Capabilities { return CapAPIServer }

func (c schedulerHealthz) Run(ctx context.Context, env *kube.Env) []result.Finding {
	now := nowFunc()
	h, err := controlPlaneLeaseHealth(ctx, env, "kube-system", schedulerLeaseName, now)
	if errors.Is(err, errControlPlaneLeaseNotFound) {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusSkipped,
			Message: "kube-scheduler lease not found in kube-system (managed cluster?)",
		}}
	}
	if err != nil {
		return unknownFromErr(c.ID(), "lease", err)
	}
	if h.Holder == "" {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusCritical,
			Message: "kube-scheduler lease has no holder (no leader elected)",
		}}
	}
	if h.Stale {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusCritical,
			Message: fmt.Sprintf("kube-scheduler lease stale: holder=%s renewed %s ago (max %s)",
				h.Holder, now.Sub(h.RenewedAt).Round(time.Second), h.MaxAge),
		}}
	}
	return []result.Finding{okFinding(c.ID(),
		fmt.Sprintf("kube-scheduler lease fresh (holder=%s, renewed %s ago)",
			h.Holder, now.Sub(h.RenewedAt).Round(time.Second)))}
}
