package checks

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// unstructuredConditions reads a []conditions slice (list of maps) from
// nested fields on an unstructured object. Returns the conditions, a
// found flag, and an error when the path resolved to a non-list value.
func unstructuredConditions(obj map[string]any, fields ...string) (conds []map[string]any, found bool, err error) {
	var raw []any
	raw, found, err = unstructured.NestedSlice(obj, fields...)
	if err != nil || !found {
		return nil, found, err
	}
	conds = make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			conds = append(conds, m)
		}
	}
	return conds, true, nil
}
