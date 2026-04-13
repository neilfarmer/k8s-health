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

func int32Ptr(i int32) *int32 { return &i }

func TestDeploymentChecker_Unavailable(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas:   1,
			UnavailableReplicas: 2,
			ReadyReplicas:       1,
		},
	}

	client := fake.NewSimpleClientset(dep)
	c := &DeploymentChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "1/3")
}

func TestDeploymentChecker_ProgressDeadlineExceeded(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "stuck", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
		},
		Status: appsv1.DeploymentStatus{
			UnavailableReplicas: 3,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:    appsv1.DeploymentProgressing,
					Reason:  "ProgressDeadlineExceeded",
					Message: "deployment exceeded its progress deadline",
				},
			},
		},
	}

	client := fake.NewSimpleClientset(dep)
	c := &DeploymentChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityCritical, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "stalled")
}

func TestDeploymentChecker_Healthy(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "healthy", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 3,
			ReadyReplicas:     3,
		},
	}

	client := fake.NewSimpleClientset(dep)
	c := &DeploymentChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}

func TestDeploymentChecker_ScaledToZero(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "scaled-down", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(0),
		},
	}

	client := fake.NewSimpleClientset(dep)
	c := &DeploymentChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}
