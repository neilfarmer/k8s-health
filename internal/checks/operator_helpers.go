package checks

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
)

// isCRDMissing reports whether err indicates the requested CRD/GVR is
// not registered with the apiserver. Operator-gated checks use this to
// turn "operator not installed" into a SKIP rather than UNKNOWN.
func isCRDMissing(err error) bool {
	if err == nil {
		return false
	}
	if apierrors.IsNotFound(err) {
		// 404 here means the *kind* is not served (vs IsResourceExpired
		// which means it was, but isn't anymore). Either way, treat as
		// "operator not installed".
		return true
	}
	if meta.IsNoMatchError(err) {
		return true
	}
	return false
}
