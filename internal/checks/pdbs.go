package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() {
	Register(&pdbsCoverage{})
	Register(&pdbsMisconfigured{})
}

// pdbsProtectedNamespaces is the set of namespaces where multi-replica
// workloads are expected to have a PDB. Conservative scope: cluster
// system namespaces only — flagging every user workload would be
// noise. Users can extend later via config.
var pdbsProtectedNamespaces = map[string]struct{}{
	"kube-system":     {},
	"cattle-system":   {},
	"longhorn-system": {},
}

type pdbsCoverage struct{}

func (pdbsCoverage) ID() string             { return "pdbs.coverage" }
func (pdbsCoverage) Description() string    { return "system Deployments/StatefulSets with replicas>=2 are covered by a PDB" }
func (pdbsCoverage) Categories() []Category { return []Category{CategoryWorkload} }
func (pdbsCoverage) Requires() Capabilities { return CapAPIServer }

func (c pdbsCoverage) Run(ctx context.Context, env *kube.Env) []result.Finding {
	out := []result.Finding{}
	for ns := range pdbsProtectedNamespaces {
		pdbs, err := env.Clientset.PolicyV1().PodDisruptionBudgets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		pdbSels := make([]labels.Selector, 0, len(pdbs.Items))
		for i := range pdbs.Items {
			s, _ := metav1.LabelSelectorAsSelector(pdbs.Items[i].Spec.Selector)
			pdbSels = append(pdbSels, s)
		}

		deps, err := env.Clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
		if err == nil {
			for i := range deps.Items {
				d := &deps.Items[i]
				if d.Spec.Replicas == nil || *d.Spec.Replicas < 2 {
					continue
				}
				if !covered(pdbSels, d.Spec.Template.Labels) {
					out = append(out, result.Finding{
						Check: c.ID(), Status: result.StatusWarning,
						Resource: resourceID("deployment", d.Namespace, d.Name),
						Message:  fmt.Sprintf("%d replicas, no PDB selecting it", *d.Spec.Replicas),
					})
				}
			}
		}

		sts, err := env.Clientset.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
		if err == nil {
			for i := range sts.Items {
				s := &sts.Items[i]
				if s.Spec.Replicas == nil || *s.Spec.Replicas < 2 {
					continue
				}
				if !covered(pdbSels, s.Spec.Template.Labels) {
					out = append(out, result.Finding{
						Check: c.ID(), Status: result.StatusWarning,
						Resource: resourceID("statefulset", s.Namespace, s.Name),
						Message:  fmt.Sprintf("%d replicas, no PDB selecting it", *s.Spec.Replicas),
					})
				}
			}
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all multi-replica system workloads covered by a PDB")}
	}
	return out
}

func covered(sels []labels.Selector, labelsMap map[string]string) bool {
	if len(labelsMap) == 0 {
		return false
	}
	set := labels.Set(labelsMap)
	for _, s := range sels {
		if s == nil || s.Empty() {
			continue
		}
		if s.Matches(set) {
			return true
		}
	}
	return false
}

type pdbsMisconfigured struct{}

func (pdbsMisconfigured) ID() string             { return "pdbs.misconfigured" }
func (pdbsMisconfigured) Description() string    { return "PDBs whose minAvailable equals or exceeds matching pod count (drains will block)" }
func (pdbsMisconfigured) Categories() []Category { return []Category{CategoryWorkload} }
func (pdbsMisconfigured) Requires() Capabilities { return CapAPIServer }

func (c pdbsMisconfigured) Run(ctx context.Context, env *kube.Env) []result.Finding {
	pdbs, err := env.Clientset.PolicyV1().PodDisruptionBudgets(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pdbs", err)
	}
	out := []result.Finding{}
	for i := range pdbs.Items {
		p := &pdbs.Items[i]
		if p.Spec.MinAvailable == nil {
			continue
		}
		if p.Spec.MinAvailable.Type == 0 { // intstr.Int
			minAv := p.Spec.MinAvailable.IntValue()
			expected := int(p.Status.ExpectedPods)
			if expected > 0 && minAv >= expected {
				out = append(out, result.Finding{
					Check: c.ID(), Status: result.StatusCritical,
					Resource: resourceID("poddisruptionbudget", p.Namespace, p.Name),
					Message:  fmt.Sprintf("minAvailable=%d but only %d expected pods (drains will block)", minAv, expected),
				})
			}
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no PDBs with blocking minAvailable")}
	}
	return out
}
