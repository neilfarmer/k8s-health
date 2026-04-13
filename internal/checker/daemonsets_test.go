package checker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDaemonSetChecker_NotReady(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "fluentd", Namespace: "kube-system"},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 5,
			NumberReady:            3,
		},
	}

	client := fake.NewSimpleClientset(ds)
	c := &DaemonSetChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "3/5")
}

func TestDaemonSetChecker_Misscheduled(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "node-exporter", Namespace: "monitoring"},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 3,
			NumberReady:            3,
			NumberMisscheduled:     2,
		},
	}

	client := fake.NewSimpleClientset(ds)
	c := &DaemonSetChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Contains(t, result.Findings[0].Message, "misscheduled")
}

func TestDaemonSetChecker_Healthy(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "healthy-ds", Namespace: "kube-system"},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 3,
			NumberReady:            3,
		},
	}

	client := fake.NewSimpleClientset(ds)
	c := &DaemonSetChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}
