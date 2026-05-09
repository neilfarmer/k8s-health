package kube

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespaceForList(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  Env
		want string
	}{
		{"single namespace", Env{Namespace: "foo"}, "foo"},
		{"all namespaces wins", Env{Namespace: "foo", AllNamespaces: true}, metav1.NamespaceAll},
		{"empty when all-namespaces", Env{AllNamespaces: true}, metav1.NamespaceAll},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.env.NamespaceForList(); got != tc.want {
				t.Fatalf("NamespaceForList() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The remaining tests use t.Setenv, which is incompatible with t.Parallel().
// They run serially.

func TestDefaultKubeconfigPath(t *testing.T) {
	t.Setenv("KUBECONFIG", "/tmp/foo")
	if got := DefaultKubeconfigPath(); got != "/tmp/foo" {
		t.Fatalf("got %q want /tmp/foo", got)
	}
}

func TestResolveConfigOutOfClusterMissing(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBECONFIG", "/nonexistent/kubeconfig.yaml")
	t.Setenv("HOME", t.TempDir())

	_, _, _, err := resolveConfig(Options{LaunchMode: ModeOutOfCluster, Kubeconfig: "/nonexistent/kubeconfig.yaml"})
	if err == nil {
		t.Fatal("expected error when kubeconfig missing")
	}
}

func TestResolveConfigUnknownMode(t *testing.T) {
	if _, _, _, err := resolveConfig(Options{LaunchMode: "wat"}); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestIsInClusterFalseWhenNoEnv(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	if isInCluster() {
		t.Fatal("expected false when env var missing")
	}
}
