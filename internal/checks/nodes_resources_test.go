package checks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func nodeWithCap(name, cpu, mem string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(mem),
			},
		},
	}
}

func withMetrics(t *testing.T, list *nodeMetricsList) {
	t.Helper()
	prev := fetchNodeMetricsFn
	fetchNodeMetricsFn = func(_ context.Context, _ *kube.Env) (*nodeMetricsList, error) { return list, nil }
	t.Cleanup(func() { fetchNodeMetricsFn = prev })
}

func TestNodesCPUBands(t *testing.T) {
	cases := []struct {
		name       string
		cpuUsage   string
		cpuCap     string
		wantStatus result.Status
	}{
		{"healthy", "1", "8", result.StatusOK},
		{"warn", "6", "8", result.StatusWarning},
		{"crit", "75", "80", result.StatusCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := envWithObjects(nodeWithCap("n1", tc.cpuCap, "8Gi"))
			withMetrics(t, &nodeMetricsList{Items: []struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
				Usage struct {
					CPU    string `json:"cpu"`
					Memory string `json:"memory"`
				} `json:"usage"`
			}{
				{
					Metadata: struct {
						Name string `json:"name"`
					}{Name: "n1"},
					Usage: struct {
						CPU    string `json:"cpu"`
						Memory string `json:"memory"`
					}{CPU: tc.cpuUsage, Memory: "1Gi"},
				},
			}})
			got := runCheck(t, &nodesCPU{}, env)
			if statusCounts(got)[tc.wantStatus] != 1 {
				t.Fatalf("want %s, got %+v", tc.wantStatus, got)
			}
		})
	}
}

func TestNodesMemoryBands(t *testing.T) {
	cases := []struct {
		name    string
		memUsed string
		memCap  string
		want    result.Status
	}{
		{"healthy", "1Gi", "16Gi", result.StatusOK},
		{"warn", "13Gi", "16Gi", result.StatusWarning},
		{"crit", "15Gi", "16Gi", result.StatusCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := envWithObjects(nodeWithCap("n1", "8", tc.memCap))
			withMetrics(t, &nodeMetricsList{Items: []struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
				Usage struct {
					CPU    string `json:"cpu"`
					Memory string `json:"memory"`
				} `json:"usage"`
			}{
				{
					Metadata: struct {
						Name string `json:"name"`
					}{Name: "n1"},
					Usage: struct {
						CPU    string `json:"cpu"`
						Memory string `json:"memory"`
					}{CPU: "1", Memory: tc.memUsed},
				},
			}})
			got := runCheck(t, &nodesMemory{}, env)
			if statusCounts(got)[tc.want] != 1 {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
		})
	}
}

func TestNodesCPUSkipsWhenMetricsUnavailable(t *testing.T) {
	prev := fetchNodeMetricsFn
	fetchNodeMetricsFn = func(_ context.Context, _ *kube.Env) (*nodeMetricsList, error) {
		return nil, &fakeNotFoundErr{}
	}
	t.Cleanup(func() { fetchNodeMetricsFn = prev })
	got := runCheck(t, &nodesCPU{}, envWithObjects())
	if statusCounts(got)[result.StatusSkipped] != 1 {
		t.Fatalf("want SKIP, got %+v", got)
	}
}

type fakeNotFoundErr struct{}

func (fakeNotFoundErr) Error() string { return "not found" }
func (fakeNotFoundErr) Status() metav1.Status {
	return metav1.Status{
		Status: metav1.StatusFailure,
		Code:   404,
		Reason: metav1.StatusReasonNotFound,
	}
}
