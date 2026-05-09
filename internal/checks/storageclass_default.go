package checks

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&storageclassDefault{}) }

const defaultSCAnnotation = "storageclass.kubernetes.io/is-default-class"

type storageclassDefault struct{}

func (storageclassDefault) ID() string             { return "storageclass.default" }
func (storageclassDefault) Description() string    { return "exactly one StorageClass annotated as default" }
func (storageclassDefault) Categories() []Category { return []Category{CategoryStorage} }
func (storageclassDefault) Requires() Capabilities { return CapAPIServer }

func (c storageclassDefault) Run(ctx context.Context, env *kube.Env) []result.Finding {
	scs, err := env.Clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list storageclasses", err)
	}
	defaults := []string{}
	for i := range scs.Items {
		s := &scs.Items[i]
		if s.Annotations[defaultSCAnnotation] == "true" {
			defaults = append(defaults, s.Name)
		}
	}
	switch len(defaults) {
	case 1:
		return []result.Finding{okFinding(c.ID(), fmt.Sprintf("default StorageClass: %s", defaults[0]))}
	case 0:
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusWarning,
			Message: "no StorageClass marked as default (PVCs without storageClassName will fail to bind)",
		}}
	default:
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusCritical,
			Message: fmt.Sprintf("%d StorageClasses marked default: %s (kubernetes will reject ambiguous PVCs)", len(defaults), strings.Join(defaults, ", ")),
		}}
	}
}
