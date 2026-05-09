package checks

import (
	"fmt"

	"github.com/neilfarmer/k8s-health/internal/result"
)

// resourceID returns "kind/name" or "kind/name in ns/<ns>" depending on
// whether ns is set. Used to keep finding.Resource strings consistent.
func resourceID(kind, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s", kind, name)
	}
	return fmt.Sprintf("%s/%s in ns/%s", kind, name, namespace)
}

// okFinding returns a single OK finding for the given check id.
func okFinding(id, msg string) result.Finding {
	return result.Finding{Check: id, Status: result.StatusOK, Message: msg}
}

// unknownFromErr returns a single UNKNOWN finding wrapping err.
func unknownFromErr(id, op string, err error) []result.Finding {
	return []result.Finding{{
		Check:   id,
		Status:  result.StatusUnknown,
		Message: fmt.Sprintf("%s: %v", op, err),
	}}
}
