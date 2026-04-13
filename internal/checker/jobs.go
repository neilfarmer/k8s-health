package checker

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobChecker inspects jobs for failures.
type JobChecker struct{}

func (c *JobChecker) Name() string        { return "jobs" }
func (c *JobChecker) Description() string { return "Jobs" }

func (c *JobChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	var jobs []batchv1.Job
	if opts.AllNamespaces() {
		list, err := opts.Client.BatchV1().Jobs("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing jobs: %w", err)
		}
		jobs = list.Items
	} else {
		for _, ns := range opts.Namespaces {
			list, err := opts.Client.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("listing jobs in namespace %s: %w", ns, err)
			}
			jobs = append(jobs, list.Items...)
		}
	}

	for i := range jobs {
		c.checkJob(&jobs[i], result)
	}

	return result, nil
}

func (c *JobChecker) checkJob(job *batchv1.Job, result *Result) {
	// Skip completed jobs
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == "True" {
			return
		}
	}

	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == "True" {
			result.Findings = append(result.Findings, Finding{
				Checker:   c.Name(),
				Severity:  SeverityCritical,
				Namespace: job.Namespace,
				Kind:      "Job",
				Name:      job.Name,
				Message:   fmt.Sprintf("Job failed: %s", cond.Message),
				Details: map[string]string{
					"reason":  cond.Reason,
					"message": cond.Message,
				},
			})
			return
		}
	}

	// Check for jobs stuck with active pods but exceeding active deadline
	if job.Spec.ActiveDeadlineSeconds != nil && job.Status.StartTime != nil {
		elapsed := time.Since(job.Status.StartTime.Time)
		deadline := time.Duration(*job.Spec.ActiveDeadlineSeconds) * time.Second
		if elapsed > deadline {
			result.Findings = append(result.Findings, Finding{
				Checker:   c.Name(),
				Severity:  SeverityWarning,
				Namespace: job.Namespace,
				Kind:      "Job",
				Name:      job.Name,
				Message:   fmt.Sprintf("Job exceeded active deadline (%s elapsed, %s deadline)", formatDuration(elapsed), formatDuration(deadline)),
			})
		}
	}

	// Check for suspended jobs
	if job.Spec.Suspend != nil && *job.Spec.Suspend {
		result.Findings = append(result.Findings, Finding{
			Checker:   c.Name(),
			Severity:  SeverityInfo,
			Namespace: job.Namespace,
			Kind:      "Job",
			Name:      job.Name,
			Message:   "Job is suspended",
		})
	}
}
