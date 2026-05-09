package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&certmanagerCertificatesNotReady{}) }

var certificatesGVR = schema.GroupVersionResource{
	Group: "cert-manager.io", Version: "v1", Resource: "certificates",
}

type certmanagerCertificatesNotReady struct{}

func (certmanagerCertificatesNotReady) ID() string { return "certmanager.certificates.notReady" }
func (certmanagerCertificatesNotReady) Description() string {
	return "cert-manager Certificate CRs report Ready=True"
}
func (certmanagerCertificatesNotReady) Categories() []Category { return []Category{CategoryControlPlane} }
func (certmanagerCertificatesNotReady) Requires() Capabilities { return CapAPIServer }

func (c certmanagerCertificatesNotReady) Run(ctx context.Context, env *kube.Env) []result.Finding {
	list, err := env.Dynamic.Resource(certificatesGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if isCRDMissing(err) {
			return []result.Finding{{
				Check: c.ID(), Status: result.StatusSkipped,
				Message: "cert-manager not installed (Certificates CRD absent)",
			}}
		}
		return unknownFromErr(c.ID(), "list certificates", err)
	}
	if len(list.Items) == 0 {
		return []result.Finding{okFinding(c.ID(), "no Certificates defined")}
	}

	out := []result.Finding{}
	for _, item := range list.Items {
		conds, found, err := unstructuredConditions(item.Object, "status", "conditions")
		if err != nil || !found {
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusWarning,
				Resource: resourceID("certificate", item.GetNamespace(), item.GetName()),
				Message:  "no status.conditions yet",
			})
			continue
		}
		ready := false
		var reason, msg string
		for _, cond := range conds {
			if cond["type"] != "Ready" {
				continue
			}
			if cond["status"] == "True" {
				ready = true
			}
			reason, _ = cond["reason"].(string)
			msg, _ = cond["message"].(string)
		}
		if !ready {
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusCritical,
				Resource: resourceID("certificate", item.GetNamespace(), item.GetName()),
				Message:  fmt.Sprintf("Ready=False reason=%s: %s", reason, msg),
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(),
			fmt.Sprintf("all %d Certificates Ready", len(list.Items)))}
	}
	return out
}
