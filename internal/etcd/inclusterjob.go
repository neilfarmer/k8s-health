package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// InClusterJob schedules a privileged Job in kube-system that runs
// `khealth etcd-probe` and reads the JSON Status off its pod logs.
//
// The launcher passes its own image (env.JobImage) so the Job runs the
// exact same khealth binary. The Job mounts /etc/kubernetes/pki/etcd from
// the host so etcdctl certs are visible.
func InClusterJob(clientset kubernetes.Interface) func(ctx context.Context, opts Options) (*Status, error) {
	return func(ctx context.Context, opts Options) (*Status, error) {
		if opts.JobImage == "" {
			return nil, fmt.Errorf("%w: --launch-mode in-cluster needs --etcd-job-image", ErrUnavailable)
		}
		ns := opts.JobNamespace
		if ns == "" {
			ns = "kube-system"
		}

		jobName := fmt.Sprintf("khealth-etcd-probe-%d", time.Now().Unix())
		job := buildEtcdProbeJob(jobName, ns, opts.JobImage)
		_, err := clientset.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("%w: create probe job: %w", ErrUnavailable, err)
		}
		defer func() {
			policy := metav1.DeletePropagationBackground
			_ = clientset.BatchV1().Jobs(ns).Delete(context.Background(), jobName, metav1.DeleteOptions{
				PropagationPolicy: &policy,
			})
		}()

		// Wait up to dialTimeout * 6 for the probe pod to finish.
		deadline := time.Now().Add(opts.DialTimeout * 6)
		var podName string
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
			pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
				LabelSelector: "job-name=" + jobName,
			})
			if err != nil || len(pods.Items) == 0 {
				continue
			}
			p := &pods.Items[0]
			podName = p.Name
			if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
				break
			}
		}
		if podName == "" {
			return nil, fmt.Errorf("%w: probe pod never appeared", ErrUnavailable)
		}

		req := clientset.CoreV1().Pods(ns).GetLogs(podName, &corev1.PodLogOptions{})
		stream, err := req.Stream(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: get probe logs: %w", ErrUnavailable, err)
		}
		defer func() { _ = stream.Close() }()
		raw, err := io.ReadAll(stream)
		if err != nil {
			return nil, fmt.Errorf("%w: read probe logs: %w", ErrUnavailable, err)
		}

		// Probe writes a single JSON line at the end. Find it.
		var st Status
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "{") {
				continue
			}
			if err := json.Unmarshal([]byte(line), &st); err == nil && !st.CollectedAt.IsZero() {
				st.Mode = ModeInClusterJob
				return &st, nil
			}
		}
		return nil, fmt.Errorf("%w: probe produced no parseable JSON line", ErrUnavailable)
	}
}

func buildEtcdProbeJob(name, ns, image string) *batchv1.Job {
	one := int32(1)
	zero := int32(0)
	hostPathDir := corev1.HostPathDirectory
	tolerateAll := []corev1.Toleration{
		{Operator: corev1.TolerationOpExists},
	}
	user := int64(0) // need root to read PKI
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "khealth",
				"app.kubernetes.io/component": "etcd-probe",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &zero,
			TTLSecondsAfterFinished: &one,
			Completions:             &one,
			Parallelism:             &one,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:   corev1.RestartPolicyNever,
					HostNetwork:     true,
					Tolerations:     tolerateAll,
					NodeSelector:    map[string]string{"node-role.kubernetes.io/control-plane": ""},
					SecurityContext: &corev1.PodSecurityContext{RunAsUser: &user},
					Containers: []corev1.Container{{
						Name:    "probe",
						Image:   image,
						Command: []string{"/usr/local/bin/khealth", "etcd-probe"},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "etcd-pki",
							MountPath: "/etc/kubernetes/pki/etcd",
							ReadOnly:  true,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "etcd-pki",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/etc/kubernetes/pki/etcd",
								Type: &hostPathDir,
							},
						},
					}},
				},
			},
		},
	}
}

// IsJobAlreadyExists is a small helper for tests.
func IsJobAlreadyExists(err error) bool { return apierrors.IsAlreadyExists(err) }
