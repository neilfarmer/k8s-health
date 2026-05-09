package checks

import (
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestRKE2HelmInstallJobs(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	job := func(name string, succeeded, failed, active int32, age time.Duration) *batchv1.Job {
		return &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:         "kube-system",
				Name:              name,
				Labels:            map[string]string{"helmcharts.helm.cattle.io/chart": name},
				CreationTimestamp: metav1.NewTime(now.Add(-age)),
			},
			Status: batchv1.JobStatus{Succeeded: succeeded, Failed: failed, Active: active},
		}
	}

	t.Run("skip when none", func(t *testing.T) {
		got := runCheck(t, &rke2HelmInstallJobs{}, envWithObjects())
		if statusCounts(got)[result.StatusSkipped] != 1 {
			t.Fatalf("want SKIP, got %+v", got)
		}
	})

	t.Run("ok when completed", func(t *testing.T) {
		withNow(t, now)
		env := envWithObjects(job("helm-install-rke2-coredns", 1, 0, 0, time.Minute))
		got := runCheck(t, &rke2HelmInstallJobs{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})

	t.Run("crit when failed", func(t *testing.T) {
		withNow(t, now)
		env := envWithObjects(job("helm-install-rke2-broken", 0, 3, 0, time.Minute))
		got := runCheck(t, &rke2HelmInstallJobs{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})

	t.Run("warn when in flight", func(t *testing.T) {
		withNow(t, now)
		env := envWithObjects(job("helm-install-rke2-young", 0, 0, 1, time.Minute))
		got := runCheck(t, &rke2HelmInstallJobs{}, env)
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})

	t.Run("crit when stuck", func(t *testing.T) {
		withNow(t, now)
		env := envWithObjects(job("helm-install-rke2-stuck", 0, 0, 1, 2*time.Hour))
		got := runCheck(t, &rke2HelmInstallJobs{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
}
