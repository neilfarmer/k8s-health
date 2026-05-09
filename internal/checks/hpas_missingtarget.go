package checks

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&hpasMissingTarget{}) }

type hpasMissingTarget struct{}

func (hpasMissingTarget) ID() string { return "hpas.missingTarget" }
func (hpasMissingTarget) Description() string {
	return "HorizontalPodAutoscaler scaleTargetRef points at a missing workload"
}
func (hpasMissingTarget) Categories() []Category { return []Category{CategoryWorkload} }
func (hpasMissingTarget) Requires() Capabilities { return CapAPIServer }

func (c hpasMissingTarget) Run(ctx context.Context, env *kube.Env) []result.Finding {
	hpas, err := env.Clientset.AutoscalingV2().HorizontalPodAutoscalers(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list hpas", err)
	}
	out := []result.Finding{}
	for i := range hpas.Items {
		h := &hpas.Items[i]
		ref := h.Spec.ScaleTargetRef
		if exists, knownKind := workloadExists(ctx, env, h.Namespace, ref.Kind, ref.Name); knownKind && !exists {
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusCritical,
				Resource: resourceID("horizontalpodautoscaler", h.Namespace, h.Name),
				Message:  fmt.Sprintf("scaleTargetRef %s/%s does not exist", ref.Kind, ref.Name),
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all HPA targets resolve")}
	}
	return out
}

// workloadExists checks the most common scaleTargetRef kinds.
// Returns (exists, knownKind). For unknown kinds we return knownKind=false
// so the caller skips the finding rather than emitting noise.
func workloadExists(ctx context.Context, env *kube.Env, ns, kind, name string) (exists, knownKind bool) {
	switch kind {
	case "Deployment":
		_, err := env.Clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		return !apierrors.IsNotFound(err), true
	case "StatefulSet":
		_, err := env.Clientset.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		return !apierrors.IsNotFound(err), true
	case "ReplicaSet":
		_, err := env.Clientset.AppsV1().ReplicaSets(ns).Get(ctx, name, metav1.GetOptions{})
		return !apierrors.IsNotFound(err), true
	case "ReplicationController":
		_, err := env.Clientset.CoreV1().ReplicationControllers(ns).Get(ctx, name, metav1.GetOptions{})
		return !apierrors.IsNotFound(err), true
	}
	return false, false
}
