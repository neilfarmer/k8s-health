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

func TestServiceChecker_LoadBalancerNoIP(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "my-lb",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{},
	}

	client := fake.NewSimpleClientset(svc)
	c := &ServiceChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "no external IP")
}

func TestServiceChecker_NoReadyEndpoints(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "my-svc",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": "test"},
		},
	}
	ep := &corev1.Endpoints{ //nolint:staticcheck // Using Endpoints API to match checker implementation
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "default"},
		Subsets:    []corev1.EndpointSubset{}, //nolint:staticcheck
	}

	client := fake.NewSimpleClientset(svc, ep)
	c := &ServiceChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Contains(t, result.Findings[0].Message, "no ready endpoints")
}

func TestServiceChecker_Healthy(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "healthy-svc",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": "test"},
		},
	}
	ep := &corev1.Endpoints{ //nolint:staticcheck // Using Endpoints API to match checker implementation
		ObjectMeta: metav1.ObjectMeta{Name: "healthy-svc", Namespace: "default"},
		Subsets: []corev1.EndpointSubset{ //nolint:staticcheck
			{Addresses: []corev1.EndpointAddress{{IP: "10.0.0.1"}}},
		},
	}

	client := fake.NewSimpleClientset(svc, ep)
	c := &ServiceChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}
