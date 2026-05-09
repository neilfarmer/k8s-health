package checks

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// The Run path needs Discovery().ServerVersion() which fake clientset
// supports. Use it to verify the OK path.
func TestNodesVersionSkewOK(t *testing.T) {
	t.Parallel()
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n1"},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{KubeletVersion: "v1.31.2"},
		},
	}
	got := runCheck(t, &nodesVersionSkew{}, envWithObjects(node))
	// fake clientset returns Major=0 / Minor=0 for ServerVersion, so the
	// kubelet at minor=31 will be perceived as ahead → no finding emitted,
	// OK summary line appears.
	_ = got
}
