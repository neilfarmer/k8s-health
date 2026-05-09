package checks

import (
	"context"
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&rbacWildcards{}) }

// rbacBuiltinPrefixes are role names we expect to be wildcarded by
// definition (system:* and the standard cluster-admin role). Skipping
// them keeps the signal focused on user-authored RBAC.
var rbacBuiltinPrefixes = []string{
	"system:",
	"cluster-admin",
}

type rbacWildcards struct{}

func (rbacWildcards) ID() string { return "rbac.wildcardVerbs" }
func (rbacWildcards) Description() string {
	return "non-system Roles/ClusterRoles granting wildcard verbs+resources (broad blast radius)"
}
func (rbacWildcards) Categories() []Category { return []Category{CategoryControlPlane} }
func (rbacWildcards) Requires() Capabilities { return CapAPIServer }

func (c rbacWildcards) Run(ctx context.Context, env *kube.Env) []result.Finding {
	out := []result.Finding{}

	crs, err := env.Clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err == nil {
		for i := range crs.Items {
			cr := &crs.Items[i]
			if isBuiltinRBAC(cr.Name) {
				continue
			}
			if rule, ok := firstWildcardRule(cr.Rules); ok {
				out = append(out, result.Finding{
					Check: c.ID(), Status: result.StatusWarning,
					Resource: "clusterrole/" + cr.Name,
					Message:  fmt.Sprintf("wildcard rule: verbs=%v resources=%v apiGroups=%v", rule.Verbs, rule.Resources, rule.APIGroups),
				})
			}
		}
	}

	rs, err := env.Clientset.RbacV1().Roles(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err == nil {
		for i := range rs.Items {
			r := &rs.Items[i]
			if isBuiltinRBAC(r.Name) {
				continue
			}
			if rule, ok := firstWildcardRule(r.Rules); ok {
				out = append(out, result.Finding{
					Check: c.ID(), Status: result.StatusWarning,
					Resource: resourceID("role", r.Namespace, r.Name),
					Message:  fmt.Sprintf("wildcard rule: verbs=%v resources=%v apiGroups=%v", rule.Verbs, rule.Resources, rule.APIGroups),
				})
			}
		}
	}

	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no user RBAC with wildcard verbs+resources")}
	}
	return out
}

func firstWildcardRule(rules []rbacv1.PolicyRule) (rbacv1.PolicyRule, bool) {
	for _, r := range rules {
		if containsWildcard(r.Verbs) && containsWildcard(r.Resources) {
			return r, true
		}
	}
	return rbacv1.PolicyRule{}, false
}

func containsWildcard(xs []string) bool {
	for _, x := range xs {
		if x == "*" {
			return true
		}
	}
	return false
}

func isBuiltinRBAC(name string) bool {
	for _, p := range rbacBuiltinPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
