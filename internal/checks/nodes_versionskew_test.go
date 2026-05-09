package checks

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestParseMinor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want int
	}{
		{"31", 31},
		{"31+", 31},
		{" 26 ", 26},
	}
	for _, tc := range cases {
		got, err := parseMinor(tc.in)
		if err != nil {
			t.Errorf("parseMinor(%q) = %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseMinor(%q) = %d want %d", tc.in, got, tc.want)
		}
	}
	if _, err := parseMinor(""); err == nil {
		t.Error("parseMinor(\"\") expected error")
	}
}

func TestParseSemverMinor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want int
	}{
		{"v1.31.2", 31},
		{"1.30.0", 30},
		{"v1.26.0-eks-abc", 26},
	}
	for _, tc := range cases {
		got, err := parseSemverMinor(tc.in)
		if err != nil {
			t.Errorf("parseSemverMinor(%q) = %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseSemverMinor(%q) = %d want %d", tc.in, got, tc.want)
		}
	}
	if _, err := parseSemverMinor("garbage"); err == nil {
		t.Error("parseSemverMinor expected error")
	}
}

// envWithApiserverMinor builds a fake env where Discovery reports apiserver
// minor M, and the cluster contains a single node running kubeletVer.
func envWithApiserverMinor(apiserverMinor, kubeletVer string) *kube.Env {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n1"},
		Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{KubeletVersion: kubeletVer}},
	}
	cs := fake.NewSimpleClientset(node)
	disc := cs.Discovery().(*fakediscovery.FakeDiscovery)
	disc.FakedServerVersion = &version.Info{Major: "1", Minor: apiserverMinor}
	return &kube.Env{Clientset: cs, Discovery: disc, AllNamespaces: true}
}

func TestNodesVersionSkewBands(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		api     string
		kubelet string
		want    result.Status
	}{
		{"matched", "31", "v1.31.2", result.StatusOK},
		{"warn skew", "31", "v1.29.0", result.StatusWarning},
		{"crit skew", "33", "v1.29.0", result.StatusCritical},
		{"kubelet ahead is OK", "30", "v1.31.2", result.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := envWithApiserverMinor(tc.api, tc.kubelet)
			got := runCheck(t, &nodesVersionSkew{}, env)
			if statusCounts(got)[tc.want] != 1 {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
		})
	}
}

func TestNodesVersionSkewBadKubeletVersion(t *testing.T) {
	t.Parallel()
	env := envWithApiserverMinor("31", "garbage")
	got := runCheck(t, &nodesVersionSkew{}, env)
	if statusCounts(got)[result.StatusUnknown] != 1 {
		t.Fatalf("want UNKNOWN, got %+v", got)
	}
}
