package checks

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&rke2HelmInstallJobs{}) }

// rke2HelmInstallStaleAfter is the age past which a still-running
// helm-install Job is treated as stuck. RKE2's bundled charts normally
// land within seconds; tens of minutes means something is wrong.
const rke2HelmInstallStaleAfter = 30 * time.Minute

type rke2HelmInstallJobs struct{}

func (rke2HelmInstallJobs) ID() string { return "rke2.helmInstallJobs" }
func (rke2HelmInstallJobs) Description() string {
	return "RKE2 helm-install Jobs in kube-system have completed"
}
func (rke2HelmInstallJobs) Categories() []Category { return []Category{CategoryControlPlane} }
func (rke2HelmInstallJobs) Requires() Capabilities { return CapAPIServer }
func (rke2HelmInstallJobs) Distros() []Distro      { return []Distro{DistroRKE2} }

func (c rke2HelmInstallJobs) Run(ctx context.Context, env *kube.Env) []result.Finding {
	jobs, err := env.Clientset.BatchV1().Jobs("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "helmcharts.helm.cattle.io/chart",
	})
	if err != nil {
		return unknownFromErr(c.ID(), "list helm-install jobs", err)
	}
	if len(jobs.Items) == 0 {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusSkipped,
			Message: "no helm-install Jobs labelled helmcharts.helm.cattle.io/chart in kube-system",
		}}
	}

	now := nowFunc()
	out := make([]result.Finding, 0, len(jobs.Items))
	for i := range jobs.Items {
		j := &jobs.Items[i]
		out = append(out, classifyHelmInstallJob(c.ID(), j, now))
	}
	return out
}

func classifyHelmInstallJob(id string, j *batchv1.Job, now time.Time) result.Finding {
	res := resourceID("job", j.Namespace, j.Name)
	if j.Status.Failed > 0 && j.Status.Succeeded == 0 {
		return result.Finding{
			Check: id, Status: result.StatusCritical, Resource: res,
			Message: fmt.Sprintf("%d failed pod(s), 0 succeeded", j.Status.Failed),
		}
	}
	if j.Status.Succeeded > 0 {
		return result.Finding{
			Check: id, Status: result.StatusOK, Resource: res,
			Message: "helm-install completed",
		}
	}
	age := now.Sub(j.CreationTimestamp.Time)
	if age > rke2HelmInstallStaleAfter {
		return result.Finding{
			Check: id, Status: result.StatusCritical, Resource: res,
			Message: fmt.Sprintf("not completed after %s (active=%d)",
				age.Round(time.Second), j.Status.Active),
		}
	}
	return result.Finding{
		Check: id, Status: result.StatusWarning, Resource: res,
		Message: fmt.Sprintf("in progress (age=%s, active=%d)",
			age.Round(time.Second), j.Status.Active),
	}
}
