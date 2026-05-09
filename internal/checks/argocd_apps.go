package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() {
	Register(&argocdAppsOutOfSync{})
	Register(&argocdAppsUnhealthy{})
}

var argocdApplicationsGVR = schema.GroupVersionResource{
	Group: "argoproj.io", Version: "v1alpha1", Resource: "applications",
}

func listArgoApps(ctx context.Context, env *kube.Env, id string) ([]unstructured.Unstructured, []result.Finding) {
	list, err := env.Dynamic.Resource(argocdApplicationsGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if isCRDMissing(err) {
			return nil, []result.Finding{{
				Check: id, Status: result.StatusSkipped,
				Message: "ArgoCD not installed (Application CRD absent)",
			}}
		}
		return nil, unknownFromErr(id, "list argocd apps", err)
	}
	return list.Items, nil
}

// --- argocd.apps.outOfSync -------------------------------------------------

type argocdAppsOutOfSync struct{}

func (argocdAppsOutOfSync) ID() string { return "argocd.apps.outOfSync" }
func (argocdAppsOutOfSync) Description() string {
	return "ArgoCD Applications with sync status != Synced"
}
func (argocdAppsOutOfSync) Categories() []Category { return []Category{CategoryWorkload} }
func (argocdAppsOutOfSync) Requires() Capabilities { return CapAPIServer }

func (c argocdAppsOutOfSync) Run(ctx context.Context, env *kube.Env) []result.Finding {
	apps, skip := listArgoApps(ctx, env, c.ID())
	if skip != nil {
		return skip
	}
	if len(apps) == 0 {
		return []result.Finding{okFinding(c.ID(), "no Applications defined")}
	}

	out := []result.Finding{}
	for _, item := range apps {
		status, _, _ := unstructured.NestedString(item.Object, "status", "sync", "status")
		if status == "" || status == "Synced" {
			continue
		}
		out = append(out, result.Finding{
			Check: c.ID(), Status: result.StatusWarning,
			Resource: resourceID("application", item.GetNamespace(), item.GetName()),
			Message:  fmt.Sprintf("sync status=%s", status),
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(),
			fmt.Sprintf("all %d ArgoCD Applications Synced", len(apps)))}
	}
	return out
}

// --- argocd.apps.unhealthy ------------------------------------------------

type argocdAppsUnhealthy struct{}

func (argocdAppsUnhealthy) ID() string { return "argocd.apps.unhealthy" }
func (argocdAppsUnhealthy) Description() string {
	return "ArgoCD Applications with health status != Healthy"
}
func (argocdAppsUnhealthy) Categories() []Category { return []Category{CategoryWorkload} }
func (argocdAppsUnhealthy) Requires() Capabilities { return CapAPIServer }

func (c argocdAppsUnhealthy) Run(ctx context.Context, env *kube.Env) []result.Finding {
	apps, skip := listArgoApps(ctx, env, c.ID())
	if skip != nil {
		return skip
	}
	if len(apps) == 0 {
		return []result.Finding{okFinding(c.ID(), "no Applications defined")}
	}

	out := []result.Finding{}
	for _, item := range apps {
		status, _, _ := unstructured.NestedString(item.Object, "status", "health", "status")
		if status == "" || status == "Healthy" {
			continue
		}
		sev := result.StatusWarning
		if status == "Degraded" || status == "Missing" {
			sev = result.StatusCritical
		}
		out = append(out, result.Finding{
			Check: c.ID(), Status: sev,
			Resource: resourceID("application", item.GetNamespace(), item.GetName()),
			Message:  fmt.Sprintf("health status=%s", status),
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(),
			fmt.Sprintf("all %d ArgoCD Applications Healthy", len(apps)))}
	}
	return out
}
