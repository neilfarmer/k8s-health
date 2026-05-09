package checks

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
)

// errControlPlaneLeaseNotFound is returned when the leader-election Lease
// for a control-plane component is absent. Managed control planes
// (EKS/GKE/AKS) hide these and the caller should turn it into a SKIP.
var errControlPlaneLeaseNotFound = errors.New("control-plane lease not found")

// leaseStaleFactor multiplies leaseDurationSeconds to decide if a renewTime
// is stale. Kubernetes leader election treats a lease as held while the
// holder renews within leaseDuration; we allow 2x to absorb clock skew and
// brief renewal hiccups before declaring a component unhealthy.
const leaseStaleFactor = 2

// nowFunc is overridden in tests so lease freshness can be evaluated
// against deterministic timestamps.
var nowFunc = time.Now

// leaseHealth describes the leader-election state of a control-plane
// component. It is computed from a coordination.k8s.io/v1 Lease.
type leaseHealth struct {
	Holder    string
	RenewedAt time.Time
	MaxAge    time.Duration
	Stale     bool
}

// controlPlaneLeaseHealth fetches the named Lease and reports holder +
// freshness. Components like kube-controller-manager and kube-scheduler
// hold these leases as part of HA leader election; a stale renewTime means
// the elected leader has stopped renewing and is therefore unhealthy.
func controlPlaneLeaseHealth(ctx context.Context, env *kube.Env, namespace, name string, now time.Time) (leaseHealth, error) {
	lease, err := env.Clientset.CoordinationV1().Leases(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return leaseHealth{}, errControlPlaneLeaseNotFound
		}
		return leaseHealth{}, fmt.Errorf("get lease %s/%s: %w", namespace, name, err)
	}

	h := leaseHealth{}
	if lease.Spec.HolderIdentity != nil {
		h.Holder = *lease.Spec.HolderIdentity
	}

	dur := int32(15)
	if lease.Spec.LeaseDurationSeconds != nil && *lease.Spec.LeaseDurationSeconds > 0 {
		dur = *lease.Spec.LeaseDurationSeconds
	}
	h.MaxAge = time.Duration(int(dur)*leaseStaleFactor) * time.Second

	if lease.Spec.RenewTime != nil {
		h.RenewedAt = lease.Spec.RenewTime.Time
	} else if lease.Spec.AcquireTime != nil {
		h.RenewedAt = lease.Spec.AcquireTime.Time
	}

	if h.Holder == "" {
		h.Stale = true
		return h, nil
	}
	if h.RenewedAt.IsZero() || now.Sub(h.RenewedAt) > h.MaxAge {
		h.Stale = true
	}
	return h, nil
}
