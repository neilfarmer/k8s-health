package checks

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&webhooksTimeout{}) }

// webhookMaxTimeoutSec is the largest timeout we tolerate for a
// failurePolicy=Fail webhook. Longer values stall every API write while
// the webhook is unhealthy. 10s matches Kubernetes' own conservative
// guidance.
const webhookMaxTimeoutSec int32 = 10

type webhooksTimeout struct{}

func (webhooksTimeout) ID() string { return "webhooks.timeout" }
func (webhooksTimeout) Description() string {
	return "admission webhooks with failurePolicy=Fail and excessive or unset timeouts"
}
func (webhooksTimeout) Categories() []Category { return []Category{CategoryControlPlane} }
func (webhooksTimeout) Requires() Capabilities { return CapAPIServer }

func (c webhooksTimeout) Run(ctx context.Context, env *kube.Env) []result.Finding {
	out := []result.Finding{}

	vals, err := env.Clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
	if err == nil {
		for i := range vals.Items {
			cfg := &vals.Items[i]
			for j := range cfg.Webhooks {
				out = append(out, classifyValidating(c.ID(), cfg.Name, &cfg.Webhooks[j])...)
			}
		}
	}
	muts, err := env.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
	if err == nil {
		for i := range muts.Items {
			cfg := &muts.Items[i]
			for j := range cfg.Webhooks {
				out = append(out, classifyMutating(c.ID(), cfg.Name, &cfg.Webhooks[j])...)
			}
		}
	}

	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no risky admission webhook timeouts")}
	}
	return out
}

func classifyValidating(id, cfgName string, w *admissionregistrationv1.ValidatingWebhook) []result.Finding {
	if w.FailurePolicy == nil || *w.FailurePolicy != admissionregistrationv1.Fail {
		return nil
	}
	return webhookFinding(id, cfgName, w.Name, w.TimeoutSeconds)
}

func classifyMutating(id, cfgName string, w *admissionregistrationv1.MutatingWebhook) []result.Finding {
	if w.FailurePolicy == nil || *w.FailurePolicy != admissionregistrationv1.Fail {
		return nil
	}
	return webhookFinding(id, cfgName, w.Name, w.TimeoutSeconds)
}

func webhookFinding(id, cfgName, hookName string, timeout *int32) []result.Finding {
	res := fmt.Sprintf("webhook/%s/%s", cfgName, hookName)
	if timeout == nil {
		return []result.Finding{{
			Check: id, Status: result.StatusWarning, Resource: res,
			Message: fmt.Sprintf("failurePolicy=Fail with no timeoutSeconds set (defaults to 30s, > %ds budget)", webhookMaxTimeoutSec),
		}}
	}
	if *timeout > webhookMaxTimeoutSec {
		return []result.Finding{{
			Check: id, Status: result.StatusWarning, Resource: res,
			Message: fmt.Sprintf("failurePolicy=Fail with timeoutSeconds=%d (> %ds)", *timeout, webhookMaxTimeoutSec),
		}}
	}
	return nil
}
