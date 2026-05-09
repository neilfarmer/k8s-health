package checks

import (
	"testing"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/neilfarmer/k8s-health/internal/result"
)


func TestStorageClassDefault(t *testing.T) {
	t.Parallel()
	defaultSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-sc",
			Annotations: map[string]string{
				defaultSCAnnotation: "true",
			},
		},
	}
	otherSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "other-sc"},
	}
	cases := []struct {
		name string
		objs []*storagev1.StorageClass
		want result.Status
	}{
		{"exactly one", []*storagev1.StorageClass{defaultSC, otherSC}, result.StatusOK},
		{"none", []*storagev1.StorageClass{otherSC}, result.StatusWarning},
		{"two defaults", []*storagev1.StorageClass{
			defaultSC,
			func() *storagev1.StorageClass {
				s := defaultSC.DeepCopy()
				s.Name = "default-sc-2"
				return s
			}(),
		}, result.StatusCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			objs := make([]runtime.Object, len(tc.objs))
			for i, o := range tc.objs {
				objs[i] = o
			}
			env := envWithObjects(objs...)
			got := runCheck(t, &storageclassDefault{}, env)
			if statusCounts(got)[tc.want] != 1 {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
		})
	}
}

func TestPodsRestarts(t *testing.T) {
	t.Parallel()
	pod := func(name string, restarts int32) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "c", RestartCount: restarts},
				},
			},
		}
	}
	cases := []struct {
		name     string
		restarts int32
		want     result.Status
	}{
		{"none", 0, result.StatusOK},
		{"warn", restartsWarnAt, result.StatusWarning},
		{"crit", restartsCritAt, result.StatusCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := envWithObjects(pod("p", tc.restarts))
			env.AllNamespaces = true
			got := runCheck(t, &podsRestarts{}, env)
			if statusCounts(got)[tc.want] != 1 {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
		})
	}
}

func TestWebhooksTimeout(t *testing.T) {
	t.Parallel()
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore

	tooLong := int32(30)
	ok := int32(5)

	cases := []struct {
		name    string
		policy  *admissionregistrationv1.FailurePolicyType
		timeout *int32
		want    result.Status
	}{
		{"fail+30s", &fail, &tooLong, result.StatusWarning},
		{"fail+nil", &fail, nil, result.StatusWarning},
		{"fail+ok", &fail, &ok, result.StatusOK},
		{"ignore+30s", &ignore, &tooLong, result.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "vw"},
				Webhooks: []admissionregistrationv1.ValidatingWebhook{
					{Name: "h", FailurePolicy: tc.policy, TimeoutSeconds: tc.timeout},
				},
			}
			env := envWithObjects(cfg)
			got := runCheck(t, &webhooksTimeout{}, env)
			if statusCounts(got)[tc.want] != 1 {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
		})
	}
}

func TestRBACWildcards(t *testing.T) {
	t.Parallel()
	wild := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "user-wide"},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"*"}, APIGroups: []string{"*"}, Resources: []string{"*"}},
		},
	}
	system := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "system:masters"},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"*"}, APIGroups: []string{"*"}, Resources: []string{"*"}},
		},
	}
	narrow := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "narrow"},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods"}},
		},
	}

	t.Run("flags non-system wildcard", func(t *testing.T) {
		t.Parallel()
		env := envWithObjects(wild, system, narrow)
		got := runCheck(t, &rbacWildcards{}, env)
		warns := statusCounts(got)[result.StatusWarning]
		if warns != 1 {
			t.Fatalf("want exactly 1 WARN (skip system + narrow), got %+v", got)
		}
	})

	t.Run("ok when no wildcards", func(t *testing.T) {
		t.Parallel()
		env := envWithObjects(narrow)
		got := runCheck(t, &rbacWildcards{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
}

func TestPDBsMisconfigured(t *testing.T) {
	t.Parallel()
	bad := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "ns"},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: intOrStringPtr(1),
		},
		Status: policyv1.PodDisruptionBudgetStatus{ExpectedPods: 1},
	}
	good := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "good", Namespace: "ns"},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: intOrStringPtr(1),
		},
		Status: policyv1.PodDisruptionBudgetStatus{ExpectedPods: 3},
	}
	t.Run("flags blocking", func(t *testing.T) {
		t.Parallel()
		env := envWithObjects(bad, good)
		env.AllNamespaces = true
		got := runCheck(t, &pdbsMisconfigured{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want 1 CRIT, got %+v", got)
		}
	})
}

func TestPDBsCoverage(t *testing.T) {
	t.Parallel()
	twoReps := int32(2)
	uncovered := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "uncovered", Namespace: "kube-system"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &twoReps,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "u"}},
			},
		},
	}
	covered := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "covered", Namespace: "kube-system"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &twoReps,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "c"}},
			},
		},
	}
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "pdb-c", Namespace: "kube-system"},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "c"}},
		},
	}

	t.Run("flags uncovered, allows covered", func(t *testing.T) {
		t.Parallel()
		env := envWithObjects(uncovered, covered, pdb)
		got := runCheck(t, &pdbsCoverage{}, env)
		warns := statusCounts(got)[result.StatusWarning]
		if warns != 1 {
			t.Fatalf("want 1 WARN, got %+v", got)
		}
	})
}

func intOrStringPtr(i int) *intstr.IntOrString {
	v := intstr.FromInt(i)
	return &v
}

func TestServicesNoBackends(t *testing.T) {
	t.Parallel()
	svcWithMatching := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "matching", Namespace: "ns"},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": "match"},
		},
	}
	svcWithNoneMatching := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: "ns"},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": "none"},
		},
	}
	matchPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", Labels: map[string]string{"app": "match"}},
	}

	t.Run("flags empty selector match", func(t *testing.T) {
		t.Parallel()
		env := envWithObjects(svcWithMatching, svcWithNoneMatching, matchPod)
		got := runCheck(t, &servicesNoBackends{}, env)
		warns := statusCounts(got)[result.StatusWarning]
		if warns != 1 {
			t.Fatalf("want 1 WARN, got %+v", got)
		}
	})

	t.Run("ok when none have selectors", func(t *testing.T) {
		t.Parallel()
		env := envWithObjects(matchPod)
		got := runCheck(t, &servicesNoBackends{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
}

func TestAPIServicesAvailable(t *testing.T) {
	t.Parallel()

	makeAPIService := func(name, status string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group: "apiregistration.k8s.io", Version: "v1", Kind: "APIService",
		})
		u.SetName(name)
		u.Object["status"] = map[string]any{
			"conditions": []any{
				map[string]any{"type": "Available", "status": status},
			},
		}
		return u
	}
	gvrToList := map[schema.GroupVersionResource]string{
		apiServicesGVR: "APIServiceList",
	}

	t.Run("ok when all available", func(t *testing.T) {
		t.Parallel()
		env := envWithDynamic(gvrToList,
			[]runtime.Object{makeAPIService("v1.metrics.k8s.io", "True")})
		got := runCheck(t, &apiservicesAvailable{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})

	t.Run("crit when one is False", func(t *testing.T) {
		t.Parallel()
		env := envWithDynamic(gvrToList, []runtime.Object{
			makeAPIService("v1.good", "True"),
			makeAPIService("v1.bad", "False"),
		})
		got := runCheck(t, &apiservicesAvailable{}, env)
		if statusCounts(got)[result.StatusCritical] < 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
}

func TestRKE2SnapshotsRecent(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	gvrToList := map[schema.GroupVersionResource]string{
		etcdSnapshotFilesGVR: "ETCDSnapshotFileList",
	}
	mkSnap := func(name string, age time.Duration) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group: "k3s.cattle.io", Version: "v1", Kind: "ETCDSnapshotFile",
		})
		u.SetName(name)
		u.SetCreationTimestamp(metav1.NewTime(now.Add(-age)))
		return u
	}

	t.Run("ok when recent", func(t *testing.T) {
		withNow(t, now)
		env := envWithDynamic(gvrToList, []runtime.Object{mkSnap("s1", time.Hour)})
		got := runCheck(t, &rke2SnapshotsRecent{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})

	t.Run("warn when stale", func(t *testing.T) {
		withNow(t, now)
		env := envWithDynamic(gvrToList, []runtime.Object{mkSnap("s1", 30*time.Hour)})
		got := runCheck(t, &rke2SnapshotsRecent{}, env)
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})

	t.Run("crit when very stale", func(t *testing.T) {
		withNow(t, now)
		env := envWithDynamic(gvrToList, []runtime.Object{mkSnap("s1", 8*24*time.Hour)})
		got := runCheck(t, &rke2SnapshotsRecent{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})

	t.Run("crit when none", func(t *testing.T) {
		env := envWithDynamic(gvrToList, nil)
		got := runCheck(t, &rke2SnapshotsRecent{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
}
