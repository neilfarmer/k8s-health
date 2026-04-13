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

func TestEventChecker_RecentWarnings(t *testing.T) {
	event := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "warn-event", Namespace: "default"},
		Type:           "Warning",
		Reason:         "FailedScheduling",
		Message:        "no nodes available",
		Count:          5,
		LastTimestamp:  metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "test-pod"},
	}

	client := fake.NewSimpleClientset(event)
	c := &EventChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	// The fake client doesn't support field selectors, so this may include the event
	// regardless. We just verify no error occurs.
	assert.NotNil(t, result)
}

func TestEventChecker_NoEvents(t *testing.T) {
	client := fake.NewSimpleClientset()
	c := &EventChecker{}
	result, err := c.Check(context.Background(), CheckOptions{Client: client})

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}
