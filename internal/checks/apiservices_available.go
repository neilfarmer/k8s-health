package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&apiservicesAvailable{}) }

var apiServicesGVR = schema.GroupVersionResource{
	Group: "apiregistration.k8s.io", Version: "v1", Resource: "apiservices",
}

type apiservicesAvailable struct{}

func (apiservicesAvailable) ID() string { return "apiservices.available" }
func (apiservicesAvailable) Description() string {
	return "aggregated APIServices report Available=True"
}
func (apiservicesAvailable) Categories() []Category { return []Category{CategoryControlPlane} }
func (apiservicesAvailable) Requires() Capabilities { return CapAPIServer }

func (c apiservicesAvailable) Run(ctx context.Context, env *kube.Env) []result.Finding {
	list, err := env.Dynamic.Resource(apiServicesGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list apiservices", err)
	}
	bad := []result.Finding{}
	for _, item := range list.Items {
		conds, found, err := unstructuredConditions(item.Object, "status", "conditions")
		if err != nil || !found {
			continue
		}
		for _, cond := range conds {
			if cond["type"] != "Available" {
				continue
			}
			if cond["status"] == "True" {
				continue
			}
			reason, _ := cond["reason"].(string)
			msg, _ := cond["message"].(string)
			bad = append(bad, result.Finding{
				Check: c.ID(), Status: result.StatusCritical,
				Resource: "apiservice/" + item.GetName(),
				Message:  fmt.Sprintf("Available=%v reason=%s: %s", cond["status"], reason, msg),
			})
		}
	}
	if len(bad) == 0 {
		return []result.Finding{okFinding(c.ID(), fmt.Sprintf("all %d APIServices Available", len(list.Items)))}
	}
	return bad
}
