package checks

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&pvcPending{}) }

const pvcPendingThreshold = 5 * time.Minute

type pvcPending struct{}

func (pvcPending) ID() string             { return "pvc.pending" }
func (pvcPending) Description() string    { return "PVCs not Bound past the threshold" }
func (pvcPending) Categories() []Category { return []Category{CategoryStorage} }
func (pvcPending) Requires() Capabilities { return CapAPIServer }

func (c pvcPending) Run(ctx context.Context, env *kube.Env) []result.Finding {
	pvcs, err := env.Clientset.CoreV1().PersistentVolumeClaims(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pvcs", err)
	}
	now := time.Now()
	var out []result.Finding
	for i := range pvcs.Items {
		p := &pvcs.Items[i]
		if p.Status.Phase == corev1.ClaimBound {
			continue
		}
		age := now.Sub(p.CreationTimestamp.Time)
		if age < pvcPendingThreshold {
			continue
		}
		out = append(out, result.Finding{
			Check:    c.ID(),
			Status:   result.StatusWarning,
			Resource: resourceID("pvc", p.Namespace, p.Name),
			Message:  fmt.Sprintf("phase=%s for %s", p.Status.Phase, age.Round(time.Second)),
			Detail:   map[string]string{"phase": string(p.Status.Phase)},
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all PVCs Bound or recent")}
	}
	return out
}
