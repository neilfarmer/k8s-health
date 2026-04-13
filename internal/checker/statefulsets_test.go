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

func TestStatefulSetChecker_NotReady(t *testing.T) {
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "cache"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: 1,
		},
	}

	client := fake.NewSimpleClientset(ss)
	c := &StatefulSetChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "1/3")
}

func TestStatefulSetChecker_Healthy(t *testing.T) {
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "cache"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: 3,
		},
	}

	client := fake.NewSimpleClientset(ss)
	c := &StatefulSetChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}

func TestStatefulSetChecker_ScaledToZero(t *testing.T) {
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "cache"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(0),
		},
	}

	client := fake.NewSimpleClientset(ss)
	c := &StatefulSetChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}
