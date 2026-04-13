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

func TestPVCChecker_Pending(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "data-pvc",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimPending,
		},
	}

	client := fake.NewSimpleClientset(pvc)
	c := &PVCChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "Pending")
}

func TestPVCChecker_Lost(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "lost-pvc", Namespace: "default"},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimLost,
		},
	}

	client := fake.NewSimpleClientset(pvc)
	c := &PVCChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityCritical, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "Lost")
}

func TestPVCChecker_Bound(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "ok-pvc", Namespace: "default"},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}

	client := fake.NewSimpleClientset(pvc)
	c := &PVCChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}
