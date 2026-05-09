package checks

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() {
	Register(&pvcLostBoundPV{})
	Register(&pvReleasedNotReclaimed{})
	Register(&volumeAttachmentsFailed{})
}

const pvReleasedStaleAfter = 24 * time.Hour

// --- pvc.lostBoundPV -------------------------------------------------------

type pvcLostBoundPV struct{}

func (pvcLostBoundPV) ID() string { return "pvc.lostBoundPV" }
func (pvcLostBoundPV) Description() string {
	return "PVCs that reference a PersistentVolume that no longer exists"
}
func (pvcLostBoundPV) Categories() []Category { return []Category{CategoryStorage} }
func (pvcLostBoundPV) Requires() Capabilities { return CapAPIServer }

func (c pvcLostBoundPV) Run(ctx context.Context, env *kube.Env) []result.Finding {
	pvcs, err := env.Clientset.CoreV1().PersistentVolumeClaims(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pvcs", err)
	}
	pvs, err := env.Clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pvs", err)
	}
	pvSet := map[string]struct{}{}
	for i := range pvs.Items {
		pvSet[pvs.Items[i].Name] = struct{}{}
	}
	out := []result.Finding{}
	for i := range pvcs.Items {
		p := &pvcs.Items[i]
		if p.Spec.VolumeName == "" {
			continue
		}
		if _, ok := pvSet[p.Spec.VolumeName]; ok {
			continue
		}
		out = append(out, result.Finding{
			Check: c.ID(), Status: result.StatusCritical,
			Resource: resourceID("persistentvolumeclaim", p.Namespace, p.Name),
			Message:  fmt.Sprintf("bound to volumeName=%s but PV does not exist", p.Spec.VolumeName),
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all bound PVCs reference an existing PV")}
	}
	return out
}

// --- pv.releasedNotReclaimed ----------------------------------------------

type pvReleasedNotReclaimed struct{}

func (pvReleasedNotReclaimed) ID() string { return "pv.releasedNotReclaimed" }
func (pvReleasedNotReclaimed) Description() string {
	return "PVs in Released phase past their reclaim window"
}
func (pvReleasedNotReclaimed) Categories() []Category { return []Category{CategoryStorage} }
func (pvReleasedNotReclaimed) Requires() Capabilities { return CapAPIServer }

func (c pvReleasedNotReclaimed) Run(ctx context.Context, env *kube.Env) []result.Finding {
	pvs, err := env.Clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pvs", err)
	}
	now := nowFunc()
	out := []result.Finding{}
	for i := range pvs.Items {
		pv := &pvs.Items[i]
		if pv.Status.Phase != corev1.VolumeReleased {
			continue
		}
		age := now.Sub(pv.CreationTimestamp.Time).Round(time.Minute)
		// Only flag if older than the stale window — newly Released
		// volumes are part of the normal reclaim flow.
		if age < pvReleasedStaleAfter {
			continue
		}
		out = append(out, result.Finding{
			Check: c.ID(), Status: result.StatusWarning,
			Resource: "persistentvolume/" + pv.Name,
			Message:  fmt.Sprintf("Released for %s (reclaim policy=%s)", age, pv.Spec.PersistentVolumeReclaimPolicy),
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no stale Released PVs")}
	}
	return out
}

// --- volumeattachments.failed ---------------------------------------------

type volumeAttachmentsFailed struct{}

func (volumeAttachmentsFailed) ID() string { return "volumeattachments.failed" }
func (volumeAttachmentsFailed) Description() string {
	return "VolumeAttachment objects with attach or detach errors"
}
func (volumeAttachmentsFailed) Categories() []Category { return []Category{CategoryStorage} }
func (volumeAttachmentsFailed) Requires() Capabilities { return CapAPIServer }

func (c volumeAttachmentsFailed) Run(ctx context.Context, env *kube.Env) []result.Finding {
	vas, err := env.Clientset.StorageV1().VolumeAttachments().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list volumeattachments", err)
	}
	out := []result.Finding{}
	for i := range vas.Items {
		va := &vas.Items[i]
		if msg := vaErrorMessage(va); msg != "" {
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusCritical,
				Resource: "volumeattachment/" + va.Name,
				Message:  msg,
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no failing VolumeAttachments")}
	}
	return out
}

func vaErrorMessage(va *storagev1.VolumeAttachment) string {
	if va.Status.AttachError != nil {
		return fmt.Sprintf("AttachError: %s", va.Status.AttachError.Message)
	}
	if va.Status.DetachError != nil {
		return fmt.Sprintf("DetachError: %s", va.Status.DetachError.Message)
	}
	return ""
}
