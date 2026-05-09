package checks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&podsMissingRefs{}) }

type podsMissingRefs struct{}

func (podsMissingRefs) ID() string { return "pods.missingRefs" }
func (podsMissingRefs) Description() string {
	return "pods reference ConfigMaps/Secrets/SAs/PVCs that don't exist"
}
func (podsMissingRefs) Categories() []Category { return []Category{CategoryWorkload} }
func (podsMissingRefs) Requires() Capabilities { return CapAPIServer }

func (c podsMissingRefs) Run(ctx context.Context, env *kube.Env) []result.Finding {
	pods, err := env.Clientset.CoreV1().Pods(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pods", err)
	}

	// Pre-list referenced kinds once per namespace; cheaper than per-pod GETs.
	cms, _ := env.Clientset.CoreV1().ConfigMaps(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	secs, _ := env.Clientset.CoreV1().Secrets(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	sas, _ := env.Clientset.CoreV1().ServiceAccounts(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	pvcs, _ := env.Clientset.CoreV1().PersistentVolumeClaims(env.NamespaceForList()).List(ctx, metav1.ListOptions{})

	cmSet := indexByNSName(cms.Items, func(o corev1.ConfigMap) (string, string) { return o.Namespace, o.Name })
	secSet := indexByNSName(secs.Items, func(o corev1.Secret) (string, string) { return o.Namespace, o.Name })
	saSet := indexByNSName(sas.Items, func(o corev1.ServiceAccount) (string, string) { return o.Namespace, o.Name })
	pvcSet := indexByNSName(pvcs.Items, func(o corev1.PersistentVolumeClaim) (string, string) { return o.Namespace, o.Name })

	out := []result.Finding{}
	for i := range pods.Items {
		p := &pods.Items[i]
		for _, msg := range podMissingRefs(p, cmSet, secSet, saSet, pvcSet) {
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusCritical,
				Resource: resourceID("pod", p.Namespace, p.Name),
				Message:  msg,
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all pod references resolve")}
	}
	return out
}

func podMissingRefs(p *corev1.Pod, cm, sec, sa, pvc map[string]struct{}) []string {
	missing := []string{}

	saName := p.Spec.ServiceAccountName
	if saName == "" {
		saName = "default"
	}
	if _, ok := sa[p.Namespace+"/"+saName]; !ok {
		missing = append(missing, fmt.Sprintf("serviceaccount/%s missing", saName))
	}

	for vi := range p.Spec.Volumes {
		v := &p.Spec.Volumes[vi]
		switch {
		case v.PersistentVolumeClaim != nil:
			name := v.PersistentVolumeClaim.ClaimName
			if _, ok := pvc[p.Namespace+"/"+name]; !ok {
				missing = append(missing, fmt.Sprintf("persistentvolumeclaim/%s missing", name))
			}
		case v.ConfigMap != nil:
			if v.ConfigMap.Optional != nil && *v.ConfigMap.Optional {
				continue
			}
			if _, ok := cm[p.Namespace+"/"+v.ConfigMap.Name]; !ok {
				missing = append(missing, fmt.Sprintf("configmap/%s missing (volume %s)", v.ConfigMap.Name, v.Name))
			}
		case v.Secret != nil:
			if v.Secret.Optional != nil && *v.Secret.Optional {
				continue
			}
			if _, ok := sec[p.Namespace+"/"+v.Secret.SecretName]; !ok {
				missing = append(missing, fmt.Sprintf("secret/%s missing (volume %s)", v.Secret.SecretName, v.Name))
			}
		case v.Projected != nil:
			for si := range v.Projected.Sources {
				src := &v.Projected.Sources[si]
				if src.ConfigMap != nil {
					if src.ConfigMap.Optional != nil && *src.ConfigMap.Optional {
						continue
					}
					if _, ok := cm[p.Namespace+"/"+src.ConfigMap.Name]; !ok {
						missing = append(missing, fmt.Sprintf("configmap/%s missing (projected)", src.ConfigMap.Name))
					}
				}
				if src.Secret != nil {
					if src.Secret.Optional != nil && *src.Secret.Optional {
						continue
					}
					if _, ok := sec[p.Namespace+"/"+src.Secret.Name]; !ok {
						missing = append(missing, fmt.Sprintf("secret/%s missing (projected)", src.Secret.Name))
					}
				}
			}
		}
	}

	for ci := range p.Spec.Containers {
		ctr := &p.Spec.Containers[ci]
		for _, ef := range ctr.EnvFrom {
			if ef.ConfigMapRef != nil && (ef.ConfigMapRef.Optional == nil || !*ef.ConfigMapRef.Optional) {
				if _, ok := cm[p.Namespace+"/"+ef.ConfigMapRef.Name]; !ok {
					missing = append(missing, fmt.Sprintf("configmap/%s missing (envFrom in %s)", ef.ConfigMapRef.Name, ctr.Name))
				}
			}
			if ef.SecretRef != nil && (ef.SecretRef.Optional == nil || !*ef.SecretRef.Optional) {
				if _, ok := sec[p.Namespace+"/"+ef.SecretRef.Name]; !ok {
					missing = append(missing, fmt.Sprintf("secret/%s missing (envFrom in %s)", ef.SecretRef.Name, ctr.Name))
				}
			}
		}
	}
	return missing
}

func indexByNSName[T any](items []T, key func(T) (string, string)) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, it := range items {
		ns, name := key(it)
		m[ns+"/"+name] = struct{}{}
	}
	return m
}
