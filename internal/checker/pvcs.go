package checker

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCChecker inspects PersistentVolumeClaims for issues.
type PVCChecker struct{}

func (c *PVCChecker) Name() string        { return "pvcs" }
func (c *PVCChecker) Description() string { return "PersistentVolumeClaims" }

func (c *PVCChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	var pvcs []corev1.PersistentVolumeClaim
	if opts.AllNamespaces() {
		list, err := opts.Client.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing pvcs: %w", err)
		}
		pvcs = list.Items
	} else {
		for _, ns := range opts.Namespaces {
			list, err := opts.Client.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("listing pvcs in namespace %s: %w", ns, err)
			}
			pvcs = append(pvcs, list.Items...)
		}
	}

	for i := range pvcs {
		c.checkPVC(&pvcs[i], result)
	}

	return result, nil
}

func (c *PVCChecker) checkPVC(pvc *corev1.PersistentVolumeClaim, result *Result) {
	switch pvc.Status.Phase {
	case corev1.ClaimPending:
		age := time.Since(pvc.CreationTimestamp.Time)
		if age > 2*time.Minute {
			result.Findings = append(result.Findings, Finding{
				Checker:   c.Name(),
				Severity:  SeverityWarning,
				Namespace: pvc.Namespace,
				Kind:      "PVC",
				Name:      pvc.Name,
				Message:   fmt.Sprintf("PVC has been Pending for %s", formatDuration(age)),
				Details: map[string]string{
					"phase": string(pvc.Status.Phase),
					"age":   formatDuration(age),
				},
			})
		}
	case corev1.ClaimLost:
		result.Findings = append(result.Findings, Finding{
			Checker:   c.Name(),
			Severity:  SeverityCritical,
			Namespace: pvc.Namespace,
			Kind:      "PVC",
			Name:      pvc.Name,
			Message:   "PVC is in Lost state (bound PersistentVolume was deleted)",
		})
	}
}
