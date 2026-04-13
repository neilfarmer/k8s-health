package checker

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CRDChecker inspects CustomResourceDefinitions for health issues.
type CRDChecker struct{}

func (c *CRDChecker) Name() string        { return "crds" }
func (c *CRDChecker) Description() string { return "CustomResourceDefinitions" }

func (c *CRDChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	if opts.DynClient == nil {
		return result, nil
	}

	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	crdList, err := opts.DynClient.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Gracefully handle permission errors
		result.Findings = append(result.Findings, Finding{
			Checker:  c.Name(),
			Severity: SeverityInfo,
			Kind:     "CRD",
			Name:     "cluster",
			Message:  fmt.Sprintf("Unable to list CRDs: %v", err),
		})
		return result, nil
	}

	for _, item := range crdList.Items {
		name := item.GetName()
		conditions, found, err := getNestedSlice(item.Object, "status", "conditions")
		if err != nil || !found {
			continue
		}

		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := condMap["type"].(string)
			condStatus, _ := condMap["status"].(string)
			condMsg, _ := condMap["message"].(string)

			if condType == "Established" && condStatus != "True" {
				result.Findings = append(result.Findings, Finding{
					Checker:  c.Name(),
					Severity: SeverityWarning,
					Kind:     "CRD",
					Name:     name,
					Message:  fmt.Sprintf("CRD is not Established: %s", condMsg),
				})
			}
			if condType == "NamesAccepted" && condStatus != "True" {
				result.Findings = append(result.Findings, Finding{
					Checker:  c.Name(),
					Severity: SeverityWarning,
					Kind:     "CRD",
					Name:     name,
					Message:  fmt.Sprintf("CRD names not accepted: %s", condMsg),
				})
			}
		}
	}

	return result, nil
}

func getNestedSlice(obj map[string]interface{}, fields ...string) ([]interface{}, bool, error) {
	current := obj
	for i, field := range fields {
		val, ok := current[field]
		if !ok {
			return nil, false, nil
		}
		if i == len(fields)-1 {
			slice, ok := val.([]interface{})
			return slice, ok, nil
		}
		nested, ok := val.(map[string]interface{})
		if !ok {
			return nil, false, fmt.Errorf("expected map at %s, got %T", field, val)
		}
		current = nested
	}
	return nil, false, nil
}
