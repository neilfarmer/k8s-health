package checks

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() {
	Register(&ingressesDanglingService{})
	Register(&ingressesMissingClass{})
}

// --- ingresses.danglingService --------------------------------------------

type ingressesDanglingService struct{}

func (ingressesDanglingService) ID() string { return "ingresses.danglingService" }
func (ingressesDanglingService) Description() string {
	return "Ingress backends reference Services that don't exist or lack the named port"
}
func (ingressesDanglingService) Categories() []Category { return []Category{CategoryNetwork} }
func (ingressesDanglingService) Requires() Capabilities { return CapAPIServer }

func (c ingressesDanglingService) Run(ctx context.Context, env *kube.Env) []result.Finding {
	ings, err := env.Clientset.NetworkingV1().Ingresses(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list ingresses", err)
	}
	svcByKey := map[string]*networkingv1Svc{}
	out := []result.Finding{}

	for i := range ings.Items {
		ing := &ings.Items[i]
		for _, ref := range collectIngressBackends(ing) {
			key := ing.Namespace + "/" + ref.name
			svc, ok := svcByKey[key]
			if !ok {
				s, err := env.Clientset.CoreV1().Services(ing.Namespace).Get(ctx, ref.name, metav1.GetOptions{})
				if err != nil {
					svc = &networkingv1Svc{exists: false}
				} else {
					svc = &networkingv1Svc{exists: true}
					for _, p := range s.Spec.Ports {
						svc.portNames = append(svc.portNames, p.Name)
						svc.portNumbers = append(svc.portNumbers, p.Port)
					}
				}
				svcByKey[key] = svc
			}
			if !svc.exists {
				out = append(out, result.Finding{
					Check: c.ID(), Status: result.StatusCritical,
					Resource: resourceID("ingress", ing.Namespace, ing.Name),
					Message:  fmt.Sprintf("backend service/%s not found", ref.name),
				})
				continue
			}
			if ref.portName != "" && !containsString(svc.portNames, ref.portName) {
				out = append(out, result.Finding{
					Check: c.ID(), Status: result.StatusCritical,
					Resource: resourceID("ingress", ing.Namespace, ing.Name),
					Message:  fmt.Sprintf("service/%s has no port named %q", ref.name, ref.portName),
				})
			}
			if ref.portNumber > 0 && !containsInt32(svc.portNumbers, ref.portNumber) {
				out = append(out, result.Finding{
					Check: c.ID(), Status: result.StatusCritical,
					Resource: resourceID("ingress", ing.Namespace, ing.Name),
					Message:  fmt.Sprintf("service/%s has no port %d", ref.name, ref.portNumber),
				})
			}
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all ingress backends resolve")}
	}
	return out
}

type networkingv1Svc struct {
	exists      bool
	portNames   []string
	portNumbers []int32
}

type ingressBackendRef struct {
	name       string
	portName   string
	portNumber int32
}

func collectIngressBackends(ing *networkingv1.Ingress) []ingressBackendRef {
	out := []ingressBackendRef{}
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		out = append(out, fromIngressService(ing.Spec.DefaultBackend.Service))
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				out = append(out, fromIngressService(path.Backend.Service))
			}
		}
	}
	return out
}

func fromIngressService(s *networkingv1.IngressServiceBackend) ingressBackendRef {
	return ingressBackendRef{
		name:       s.Name,
		portName:   s.Port.Name,
		portNumber: s.Port.Number,
	}
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func containsInt32(xs []int32, n int32) bool {
	for _, x := range xs {
		if x == n {
			return true
		}
	}
	return false
}

// --- ingresses.missingClass -----------------------------------------------

type ingressesMissingClass struct{}

func (ingressesMissingClass) ID() string { return "ingresses.missingClass" }
func (ingressesMissingClass) Description() string {
	return "Ingress ingressClassName references an IngressClass that doesn't exist"
}
func (ingressesMissingClass) Categories() []Category { return []Category{CategoryNetwork} }
func (ingressesMissingClass) Requires() Capabilities { return CapAPIServer }

func (c ingressesMissingClass) Run(ctx context.Context, env *kube.Env) []result.Finding {
	ings, err := env.Clientset.NetworkingV1().Ingresses(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list ingresses", err)
	}
	classes, err := env.Clientset.NetworkingV1().IngressClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list ingressclasses", err)
	}
	classSet := map[string]struct{}{}
	for i := range classes.Items {
		classSet[classes.Items[i].Name] = struct{}{}
	}

	out := []result.Finding{}
	for i := range ings.Items {
		ing := &ings.Items[i]
		if ing.Spec.IngressClassName == nil || *ing.Spec.IngressClassName == "" {
			continue
		}
		if _, ok := classSet[*ing.Spec.IngressClassName]; !ok {
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusCritical,
				Resource: resourceID("ingress", ing.Namespace, ing.Name),
				Message:  fmt.Sprintf("ingressClassName=%q does not exist", *ing.Spec.IngressClassName),
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all ingressClassName values resolve")}
	}
	return out
}
