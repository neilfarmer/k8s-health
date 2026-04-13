package checker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodChecker_CrashLoopBackOff(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "crash-pod", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "app",
					RestartCount: 42,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "CrashLoopBackOff",
						},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)
	c := &PodChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityCritical, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "CrashLoopBackOff")
	assert.Equal(t, "crash-pod", result.Findings[0].Name)
}

func TestPodChecker_ImagePullBackOff(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pull-pod", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "ImagePullBackOff",
						},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)
	c := &PodChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "ImagePullBackOff")
}

func TestPodChecker_OOMKilled(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "oom-pod", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "app",
					RestartCount: 5,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason:   "OOMKilled",
							ExitCode: 137,
						},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)
	c := &PodChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityCritical, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "OOMKilled")
}

func TestPodChecker_Pending(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pending-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	client := fake.NewSimpleClientset(pod)
	c := &PodChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "Pending")
}

func TestPodChecker_PendingRecent(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "new-pending-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Second)),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	client := fake.NewSimpleClientset(pod)
	c := &PodChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}

func TestPodChecker_Healthy(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "healthy-pod", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "app",
					Ready: true,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)
	c := &PodChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}

func TestPodChecker_Succeeded(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "completed-pod", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		},
	}

	client := fake.NewSimpleClientset(pod)
	c := &PodChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}

func TestPodChecker_NamespaceFiltering(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "crash-pod", Namespace: "target"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "crash-pod2", Namespace: "other"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod1, pod2)
	c := &PodChecker{}
	result, err := c.Check(context.Background(), CheckOptions{
		Client:     client,
		Namespaces: []string{"target"},
	})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, "target", result.Findings[0].Namespace)
}

func TestPodChecker_UnknownPhase(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "unknown-pod", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodUnknown,
		},
	}

	client := fake.NewSimpleClientset(pod)
	c := &PodChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityCritical, result.Findings[0].Severity)
}

func TestPodChecker_CreateContainerConfigError(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "config-err-pod", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "CreateContainerConfigError",
						},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)
	c := &PodChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityCritical, result.Findings[0].Severity)
}
