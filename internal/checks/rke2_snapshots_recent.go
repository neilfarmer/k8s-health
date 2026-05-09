package checks

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&rke2SnapshotsRecent{}) }

var etcdSnapshotFilesGVR = schema.GroupVersionResource{
	Group: "k3s.cattle.io", Version: "v1", Resource: "etcdsnapshotfiles",
}

const (
	snapshotWarnAfter = 24 * time.Hour
	snapshotCritAfter = 7 * 24 * time.Hour
)

type rke2SnapshotsRecent struct{}

func (rke2SnapshotsRecent) ID() string { return "rke2.snapshots.recent" }
func (rke2SnapshotsRecent) Description() string {
	return "RKE2 etcd snapshot taken within the last 24h"
}
func (rke2SnapshotsRecent) Categories() []Category { return []Category{CategoryControlPlane} }
func (rke2SnapshotsRecent) Requires() Capabilities { return CapAPIServer }
func (rke2SnapshotsRecent) Distros() []Distro      { return []Distro{DistroRKE2} }

func (c rke2SnapshotsRecent) Run(ctx context.Context, env *kube.Env) []result.Finding {
	list, err := env.Dynamic.Resource(etcdSnapshotFilesGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusSkipped,
			Message: fmt.Sprintf("etcdsnapshotfiles CR not available: %v", err),
		}}
	}
	if len(list.Items) == 0 {
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusCritical,
			Message: "no ETCDSnapshotFile resources found (snapshots not running?)",
		}}
	}

	var newest time.Time
	for _, item := range list.Items {
		ts := item.GetCreationTimestamp().Time
		if ts.After(newest) {
			newest = ts
		}
	}
	age := nowFunc().Sub(newest).Round(time.Minute)
	switch {
	case age >= snapshotCritAfter:
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusCritical,
			Message: fmt.Sprintf("newest snapshot is %s old", age),
		}}
	case age >= snapshotWarnAfter:
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusWarning,
			Message: fmt.Sprintf("newest snapshot is %s old (>24h)", age),
		}}
	default:
		return []result.Finding{okFinding(c.ID(),
			fmt.Sprintf("%d snapshots; newest %s old", len(list.Items), age))}
	}
}
