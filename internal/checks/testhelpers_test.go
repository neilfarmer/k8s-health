package checks

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

// envWithObjects builds a *kube.Env backed by a fake clientset preloaded
// with objs. AllNamespaces is true by default so list-everything checks
// see every fixture.
func envWithObjects(objs ...runtime.Object) *kube.Env {
	cs := fake.NewSimpleClientset(objs...)
	return &kube.Env{
		Clientset:     cs,
		Discovery:     cs.Discovery(),
		AllNamespaces: true,
	}
}

// envWithDynamic builds a *kube.Env with an empty typed clientset and a
// dynamic fake clientset preloaded with dynObjs. gvrToListKind teaches
// the dynamic fake about each GVR it must serve so List() returns the
// right list-kind. Tests that need both typed and dynamic objects should
// drop a typed object into the typed fake separately.
func envWithDynamic(gvrToListKind map[schema.GroupVersionResource]string, dynObjs []runtime.Object) *kube.Env {
	cs := fake.NewSimpleClientset()
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, dynObjs...)
	return &kube.Env{
		Clientset:     cs,
		Dynamic:       dyn,
		Discovery:     cs.Discovery(),
		AllNamespaces: true,
	}
}

func runCheck(t *testing.T, c Check, env *kube.Env) []result.Finding {
	t.Helper()
	return c.Run(context.Background(), env)
}

// statusCounts tallies findings by status, useful for asserting the shape
// of a result without locking tests to specific Resource strings.
func statusCounts(fs []result.Finding) map[result.Status]int {
	m := map[result.Status]int{}
	for _, f := range fs {
		m[f.Status]++
	}
	return m
}
