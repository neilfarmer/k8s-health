package checker

import (
	"context"
	"fmt"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressChecker inspects ingresses for configuration issues.
type IngressChecker struct{}

func (c *IngressChecker) Name() string        { return "ingresses" }
func (c *IngressChecker) Description() string { return "Ingresses" }

func (c *IngressChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	var ingresses []networkingv1.Ingress
	if opts.AllNamespaces() {
		list, err := opts.Client.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing ingresses: %w", err)
		}
		ingresses = list.Items
	} else {
		for _, ns := range opts.Namespaces {
			list, err := opts.Client.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("listing ingresses in namespace %s: %w", ns, err)
			}
			ingresses = append(ingresses, list.Items...)
		}
	}

	for i := range ingresses {
		c.checkIngress(ctx, &ingresses[i], opts, result)
	}

	return result, nil
}

func (c *IngressChecker) checkIngress(ctx context.Context, ing *networkingv1.Ingress, opts CheckOptions, result *Result) {
	// Check for ingresses with no address assigned
	age := time.Since(ing.CreationTimestamp.Time)
	if age > 5*time.Minute && len(ing.Status.LoadBalancer.Ingress) == 0 {
		result.Findings = append(result.Findings, Finding{
			Checker:   c.Name(),
			Severity:  SeverityWarning,
			Namespace: ing.Namespace,
			Kind:      "Ingress",
			Name:      ing.Name,
			Message:   fmt.Sprintf("Ingress has no address assigned after %s", formatDuration(age)),
		})
	}

	// Check for backends referencing non-existent services
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service == nil {
				continue
			}
			svcName := path.Backend.Service.Name
			_, err := opts.Client.CoreV1().Services(ing.Namespace).Get(ctx, svcName, metav1.GetOptions{})
			if err != nil {
				result.Findings = append(result.Findings, Finding{
					Checker:   c.Name(),
					Severity:  SeverityWarning,
					Namespace: ing.Namespace,
					Kind:      "Ingress",
					Name:      ing.Name,
					Message:   fmt.Sprintf("Ingress references non-existent service %q", svcName),
					Details: map[string]string{
						"service": svcName,
						"path":    path.Path,
					},
				})
			}
		}
	}
}
