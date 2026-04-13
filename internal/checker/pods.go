package checker

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodChecker inspects pods for common failure states.
type PodChecker struct{}

func (c *PodChecker) Name() string        { return "pods" }
func (c *PodChecker) Description() string { return "Pods" }

func (c *PodChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	var pods []corev1.Pod
	if opts.AllNamespaces() {
		podList, err := opts.Client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing pods: %w", err)
		}
		pods = podList.Items
	} else {
		for _, ns := range opts.Namespaces {
			podList, err := opts.Client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("listing pods in namespace %s: %w", ns, err)
			}
			pods = append(pods, podList.Items...)
		}
	}

	for i := range pods {
		c.checkPod(&pods[i], result)
	}

	return result, nil
}

var criticalWaitingReasons = map[string]bool{
	"CrashLoopBackOff":           true,
	"CreateContainerConfigError": true,
	"RunContainerError":          true,
	"CreateContainerError":       true,
}

var warningWaitingReasons = map[string]bool{
	"ImagePullBackOff":  true,
	"ErrImagePull":      true,
	"ErrImageNeverPull": true,
}

func (c *PodChecker) checkPod(pod *corev1.Pod, result *Result) {
	// Skip completed pods that are owned by jobs
	if pod.Status.Phase == corev1.PodSucceeded {
		return
	}

	// Check for pending pods (stuck for > 2 minutes)
	if pod.Status.Phase == corev1.PodPending {
		age := time.Since(pod.CreationTimestamp.Time)
		if age > 2*time.Minute {
			result.Findings = append(result.Findings, Finding{
				Checker:   c.Name(),
				Severity:  SeverityWarning,
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Name:      pod.Name,
				Message:   fmt.Sprintf("Pod has been Pending for %s", formatDuration(age)),
				Details: map[string]string{
					"phase": string(pod.Status.Phase),
					"age":   formatDuration(age),
				},
			})
		}
		return
	}

	// Check for unknown phase
	if pod.Status.Phase == corev1.PodUnknown {
		result.Findings = append(result.Findings, Finding{
			Checker:   c.Name(),
			Severity:  SeverityCritical,
			Namespace: pod.Namespace,
			Kind:      "Pod",
			Name:      pod.Name,
			Message:   "Pod is in Unknown phase",
		})
		return
	}

	// Check container statuses
	allStatuses := make([]corev1.ContainerStatus, 0, len(pod.Status.InitContainerStatuses)+len(pod.Status.ContainerStatuses))
	allStatuses = append(allStatuses, pod.Status.InitContainerStatuses...)
	allStatuses = append(allStatuses, pod.Status.ContainerStatuses...)
	for _, cs := range allStatuses {
		c.checkContainerStatus(pod, cs, result)
	}
}

func (c *PodChecker) checkContainerStatus(pod *corev1.Pod, cs corev1.ContainerStatus, result *Result) {
	if cs.State.Waiting != nil {
		reason := cs.State.Waiting.Reason
		if criticalWaitingReasons[reason] {
			result.Findings = append(result.Findings, Finding{
				Checker:   c.Name(),
				Severity:  SeverityCritical,
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Name:      pod.Name,
				Message:   fmt.Sprintf("Container %q is in %s (%d restarts)", cs.Name, reason, cs.RestartCount),
				Details: map[string]string{
					"container": cs.Name,
					"reason":    reason,
					"restarts":  fmt.Sprintf("%d", cs.RestartCount),
				},
			})
		} else if warningWaitingReasons[reason] {
			result.Findings = append(result.Findings, Finding{
				Checker:   c.Name(),
				Severity:  SeverityWarning,
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Name:      pod.Name,
				Message:   fmt.Sprintf("Container %q is in %s", cs.Name, reason),
				Details: map[string]string{
					"container": cs.Name,
					"reason":    reason,
				},
			})
		}
	}

	// Check for OOMKilled in last termination state
	if cs.LastTerminationState.Terminated != nil {
		term := cs.LastTerminationState.Terminated
		if term.Reason == "OOMKilled" {
			result.Findings = append(result.Findings, Finding{
				Checker:   c.Name(),
				Severity:  SeverityCritical,
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Name:      pod.Name,
				Message:   fmt.Sprintf("Container %q was OOMKilled (exit code %d, %d restarts)", cs.Name, term.ExitCode, cs.RestartCount),
				Details: map[string]string{
					"container": cs.Name,
					"reason":    "OOMKilled",
					"exitCode":  fmt.Sprintf("%d", term.ExitCode),
					"restarts":  fmt.Sprintf("%d", cs.RestartCount),
				},
			})
		}
	}

	// Check for currently terminated with error
	if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
		term := cs.State.Terminated
		if term.Reason != "" && term.Reason != "Completed" {
			result.Findings = append(result.Findings, Finding{
				Checker:   c.Name(),
				Severity:  SeverityWarning,
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Name:      pod.Name,
				Message:   fmt.Sprintf("Container %q terminated with %s (exit code %d)", cs.Name, term.Reason, term.ExitCode),
				Details: map[string]string{
					"container": cs.Name,
					"reason":    term.Reason,
					"exitCode":  fmt.Sprintf("%d", term.ExitCode),
				},
			})
		}
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}
