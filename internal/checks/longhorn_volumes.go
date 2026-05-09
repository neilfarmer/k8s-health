package checks

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&longhornVolumesDegraded{}) }

var longhornVolumesGVR = schema.GroupVersionResource{
	Group: "longhorn.io", Version: "v1beta2", Resource: "volumes",
}

type longhornVolumesDegraded struct{}

func (longhornVolumesDegraded) ID() string { return "longhorn.volumes.degraded" }
func (longhornVolumesDegraded) Description() string {
	return "Longhorn Volume CRs report robustness=healthy"
}
func (longhornVolumesDegraded) Categories() []Category { return []Category{CategoryStorage} }
func (longhornVolumesDegraded) Requires() Capabilities { return CapAPIServer }

func (c longhornVolumesDegraded) Run(ctx context.Context, env *kube.Env) []result.Finding {
	list, err := env.Dynamic.Resource(longhornVolumesGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if isCRDMissing(err) {
			return []result.Finding{{
				Check: c.ID(), Status: result.StatusSkipped,
				Message: "Longhorn not installed (Volume CRD absent)",
			}}
		}
		return unknownFromErr(c.ID(), "list longhorn volumes", err)
	}
	if len(list.Items) == 0 {
		return []result.Finding{okFinding(c.ID(), "no Longhorn Volumes defined")}
	}

	out := []result.Finding{}
	for _, item := range list.Items {
		robustness, _, _ := unstructured.NestedString(item.Object, "status", "robustness")
		state, _, _ := unstructured.NestedString(item.Object, "status", "state")
		switch robustness {
		case "healthy", "":
			// healthy or unset (not yet reconciled). Skip.
		case "degraded":
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusWarning,
				Resource: resourceID("volume", item.GetNamespace(), item.GetName()),
				Message:  fmt.Sprintf("robustness=degraded state=%s", state),
			})
		case "faulted", "unknown":
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusCritical,
				Resource: resourceID("volume", item.GetNamespace(), item.GetName()),
				Message:  fmt.Sprintf("robustness=%s state=%s", robustness, state),
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(),
			fmt.Sprintf("all %d Longhorn Volumes healthy", len(list.Items)))}
	}
	return out
}
