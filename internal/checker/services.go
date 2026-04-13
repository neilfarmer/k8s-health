package checker

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceChecker inspects services and their endpoints.
type ServiceChecker struct{}

func (c *ServiceChecker) Name() string        { return "services" }
func (c *ServiceChecker) Description() string { return "Services" }

func (c *ServiceChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	var services []corev1.Service
	if opts.AllNamespaces() {
		list, err := opts.Client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing services: %w", err)
		}
		services = list.Items
	} else {
		for _, ns := range opts.Namespaces {
			list, err := opts.Client.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("listing services in namespace %s: %w", ns, err)
			}
			services = append(services, list.Items...)
		}
	}

	for i := range services {
		c.checkService(ctx, &services[i], opts, result)
	}

	return result, nil
}

func (c *ServiceChecker) checkService(ctx context.Context, svc *corev1.Service, opts CheckOptions, result *Result) {
	// Check LoadBalancer services without external IP
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		age := time.Since(svc.CreationTimestamp.Time)
		if age > 5*time.Minute && len(svc.Status.LoadBalancer.Ingress) == 0 {
			result.Findings = append(result.Findings, Finding{
				Checker:   c.Name(),
				Severity:  SeverityWarning,
				Namespace: svc.Namespace,
				Kind:      "Service",
				Name:      svc.Name,
				Message:   fmt.Sprintf("LoadBalancer service has no external IP after %s", formatDuration(age)),
				Details: map[string]string{
					"type": "LoadBalancer",
					"age":  formatDuration(age),
				},
			})
		}
	}

	// Check for services with selectors that have no ready endpoints
	if len(svc.Spec.Selector) > 0 && svc.Spec.Type != corev1.ServiceTypeExternalName {
		endpoints, err := opts.Client.CoreV1().Endpoints(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			return // Skip if we can't get endpoints
		}

		readyAddresses := 0
		for _, subset := range endpoints.Subsets {
			readyAddresses += len(subset.Addresses)
		}

		if readyAddresses == 0 {
			// Only flag if the service has been around for a bit
			age := time.Since(svc.CreationTimestamp.Time)
			if age > 2*time.Minute {
				result.Findings = append(result.Findings, Finding{
					Checker:   c.Name(),
					Severity:  SeverityWarning,
					Namespace: svc.Namespace,
					Kind:      "Service",
					Name:      svc.Name,
					Message:   "Service has no ready endpoints",
					Details: map[string]string{
						"type": string(svc.Spec.Type),
					},
				})
			}
		}
	}
}
