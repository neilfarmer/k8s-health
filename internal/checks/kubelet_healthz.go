package checks

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&kubeletHealthz{}) }

type kubeletHealthz struct{}

func (kubeletHealthz) ID() string { return "kubelet.healthz" }
func (kubeletHealthz) Description() string {
	return "Each node's kubelet /healthz returns ok via the apiserver proxy"
}
func (kubeletHealthz) Categories() []Category { return []Category{CategoryNode} }
func (kubeletHealthz) Requires() Capabilities { return CapAPIServer }

func (c kubeletHealthz) Run(ctx context.Context, env *kube.Env) []result.Finding {
	nodes, err := env.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list nodes", err)
	}
	rest := env.Clientset.CoreV1().RESTClient()
	if rest == nil {
		return []result.Finding{{Check: c.ID(), Status: result.StatusSkipped, Message: "no REST client (fake clientset)"}}
	}
	var out []result.Finding
	for i := range nodes.Items {
		n := &nodes.Items[i]
		body, err := rest.Get().
			Resource("nodes").
			Name(n.Name).
			SubResource("proxy").
			Suffix("healthz").
			DoRaw(ctx)
		if err != nil {
			if apierrors.IsForbidden(err) {
				out = append(out, result.Finding{
					Check: c.ID(), Status: result.StatusSkipped,
					Resource: resourceID("node", "", n.Name),
					Message:  "node proxy forbidden (no RBAC for nodes/proxy)",
				})
				continue
			}
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusCritical,
				Resource: resourceID("node", "", n.Name),
				Message:  fmt.Sprintf("/healthz: %v", err),
			})
			continue
		}
		if string(body) != "ok" {
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusWarning,
				Resource: resourceID("node", "", n.Name),
				Message:  fmt.Sprintf("/healthz body: %q", string(body)),
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all kubelets /healthz=ok")}
	}
	return out
}
