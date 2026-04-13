package checker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNodeChecker_NotReady(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:    corev1.NodeReady,
					Status:  corev1.ConditionFalse,
					Reason:  "KubeletNotReady",
					Message: "container runtime not ready",
				},
			},
		},
	}

	client := fake.NewSimpleClientset(node)
	c := &NodeChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityCritical, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "NotReady")
}

func TestNodeChecker_DiskPressure(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-2"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionTrue, Reason: "DiskPressure"},
			},
		},
	}

	client := fake.NewSimpleClientset(node)
	c := &NodeChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "DiskPressure")
}

func TestNodeChecker_MemoryPressure(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-3"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue, Reason: "MemoryPressure"},
			},
		},
	}

	client := fake.NewSimpleClientset(node)
	c := &NodeChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
}

func TestNodeChecker_Cordoned(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-4"},
		Spec: corev1.NodeSpec{
			Unschedulable: true,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	client := fake.NewSimpleClientset(node)
	c := &NodeChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "cordoned")
}

func TestNodeChecker_Healthy(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "healthy-node"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
				{Type: corev1.NodePIDPressure, Status: corev1.ConditionFalse},
			},
		},
	}

	client := fake.NewSimpleClientset(node)
	c := &NodeChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}

func TestNodeChecker_NetworkUnavailable(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "net-node"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionTrue, Reason: "NoRouteCreated"},
			},
		},
	}

	client := fake.NewSimpleClientset(node)
	c := &NodeChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityCritical, result.Findings[0].Severity)
}
