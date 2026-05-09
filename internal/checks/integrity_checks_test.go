package checks

import (
	"testing"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestPodsMissingRefs(t *testing.T) {
	t.Parallel()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
		Spec: corev1.PodSpec{
			ServiceAccountName: "missing-sa",
			Volumes: []corev1.Volume{
				{Name: "cm-vol", VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "missing-cm"},
					},
				}},
			},
		},
	}
	env := envWithObjects(pod)
	got := runCheck(t, &podsMissingRefs{}, env)
	if statusCounts(got)[result.StatusCritical] < 2 {
		t.Fatalf("want CRITs for missing SA + CM, got %+v", got)
	}
}

func TestPodsMissingRefsAllResolve(t *testing.T) {
	t.Parallel()
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "ns"}}
	env := envWithObjects(sa, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
	})
	got := runCheck(t, &podsMissingRefs{}, env)
	if statusCounts(got)[result.StatusOK] != 1 {
		t.Fatalf("want OK, got %+v", got)
	}
}

func TestNamespacesStuckTerminating(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	withNow(t, now)

	makeNS := func(name string, ageMin int) *corev1.Namespace {
		dt := metav1.NewTime(now.Add(-time.Duration(ageMin) * time.Minute))
		return &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: name, DeletionTimestamp: &dt},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating},
		}
	}
	ok := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}

	t.Run("warn", func(t *testing.T) {
		env := envWithObjects(makeNS("warn-ns", 10), ok)
		got := runCheck(t, &namespacesTerminating{}, env)
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
	t.Run("crit", func(t *testing.T) {
		env := envWithObjects(makeNS("crit-ns", 120), ok)
		got := runCheck(t, &namespacesTerminating{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
	t.Run("none", func(t *testing.T) {
		env := envWithObjects(ok)
		got := runCheck(t, &namespacesTerminating{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
}

func TestIngressesMissingClass(t *testing.T) {
	t.Parallel()
	className := "nginx"
	missing := "ghost"
	withClass := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ok-ing", Namespace: "ns"},
		Spec:       networkingv1.IngressSpec{IngressClassName: &className},
	}
	withMissing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-ing", Namespace: "ns"},
		Spec:       networkingv1.IngressSpec{IngressClassName: &missing},
	}
	class := &networkingv1.IngressClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}}

	t.Run("flags missing class", func(t *testing.T) {
		env := envWithObjects(withClass, withMissing, class)
		got := runCheck(t, &ingressesMissingClass{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
	t.Run("ok when all resolve", func(t *testing.T) {
		env := envWithObjects(withClass, class)
		got := runCheck(t, &ingressesMissingClass{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
}

func TestIngressesDanglingService(t *testing.T) {
	t.Parallel()
	mkIngress := func(name, svcName string, port int32) *networkingv1.Ingress {
		pathType := networkingv1.PathTypePrefix
		return &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: svcName,
										Port: networkingv1.ServiceBackendPort{Number: port},
									},
								},
							}},
						},
					},
				}},
			},
		}
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "ok-svc", Namespace: "ns"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 80}},
		},
	}
	env := envWithObjects(svc, mkIngress("ok", "ok-svc", 80), mkIngress("bad", "ghost", 80))
	got := runCheck(t, &ingressesDanglingService{}, env)
	if statusCounts(got)[result.StatusCritical] < 1 {
		t.Fatalf("want CRIT for dangling service, got %+v", got)
	}
}

func TestHPAsMissingTarget(t *testing.T) {
	t.Parallel()
	hpa := func(targetName string) *autoscalingv2.HorizontalPodAutoscaler {
		return &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: "h-" + targetName, Namespace: "ns"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind: "Deployment", Name: targetName, APIVersion: "apps/v1",
				},
			},
		}
	}
	env := envWithObjects(hpa("ghost"))
	got := runCheck(t, &hpasMissingTarget{}, env)
	if statusCounts(got)[result.StatusCritical] != 1 {
		t.Fatalf("want CRIT, got %+v", got)
	}
}

func TestCertManagerCertificatesNotReadySkipsWhenCRDMissing(t *testing.T) {
	t.Parallel()
	gvrToList := map[schema.GroupVersionResource]string{
		certificatesGVR: "CertificateList",
	}
	env := envWithDynamic(gvrToList, nil)
	got := runCheck(t, &certmanagerCertificatesNotReady{}, env)
	if statusCounts(got)[result.StatusOK] != 1 {
		t.Fatalf("want OK (no Certificates), got %+v", got)
	}
}

func TestCertManagerCertificatesCriticalWhenNotReady(t *testing.T) {
	t.Parallel()
	gvrToList := map[schema.GroupVersionResource]string{
		certificatesGVR: "CertificateList",
	}
	notReady := makeCert("ns/bad", "False")
	ready := makeCert("ns/good", "True")
	env := envWithDynamic(gvrToList, []runtime.Object{notReady, ready})
	got := runCheck(t, &certmanagerCertificatesNotReady{}, env)
	if statusCounts(got)[result.StatusCritical] != 1 {
		t.Fatalf("want CRIT, got %+v", got)
	}
}

func makeCert(nsName, ready string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cert-manager.io", Version: "v1", Kind: "Certificate",
	})
	for i := 0; i < len(nsName); i++ {
		if nsName[i] == '/' {
			u.SetNamespace(nsName[:i])
			u.SetName(nsName[i+1:])
			break
		}
	}
	u.Object["status"] = map[string]any{
		"conditions": []any{
			map[string]any{"type": "Ready", "status": ready, "reason": "x", "message": "x"},
		},
	}
	return u
}

func TestLonghornVolumesDegraded(t *testing.T) {
	t.Parallel()
	gvrToList := map[schema.GroupVersionResource]string{
		longhornVolumesGVR: "VolumeList",
	}
	mkVol := func(name, robustness string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group: "longhorn.io", Version: "v1beta2", Kind: "Volume",
		})
		u.SetNamespace("longhorn-system")
		u.SetName(name)
		u.Object["status"] = map[string]any{
			"robustness": robustness, "state": "attached",
		}
		return u
	}
	t.Run("ok when healthy", func(t *testing.T) {
		env := envWithDynamic(gvrToList, []runtime.Object{mkVol("v1", "healthy")})
		got := runCheck(t, &longhornVolumesDegraded{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
	t.Run("warn when degraded", func(t *testing.T) {
		env := envWithDynamic(gvrToList, []runtime.Object{mkVol("v1", "degraded")})
		got := runCheck(t, &longhornVolumesDegraded{}, env)
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
	t.Run("crit when faulted", func(t *testing.T) {
		env := envWithDynamic(gvrToList, []runtime.Object{mkVol("v1", "faulted")})
		got := runCheck(t, &longhornVolumesDegraded{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
}

func TestArgoCDApps(t *testing.T) {
	t.Parallel()
	gvrToList := map[schema.GroupVersionResource]string{
		argocdApplicationsGVR: "ApplicationList",
	}
	mkApp := func(name, sync, health string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group: "argoproj.io", Version: "v1alpha1", Kind: "Application",
		})
		u.SetNamespace("argocd")
		u.SetName(name)
		u.Object["status"] = map[string]any{
			"sync":   map[string]any{"status": sync},
			"health": map[string]any{"status": health},
		}
		return u
	}
	env := envWithDynamic(gvrToList, []runtime.Object{
		mkApp("ok", "Synced", "Healthy"),
		mkApp("drift", "OutOfSync", "Healthy"),
		mkApp("sick", "Synced", "Degraded"),
	})

	t.Run("flags out of sync", func(t *testing.T) {
		got := runCheck(t, &argocdAppsOutOfSync{}, env)
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want 1 WARN, got %+v", got)
		}
	})
	t.Run("flags unhealthy", func(t *testing.T) {
		got := runCheck(t, &argocdAppsUnhealthy{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want 1 CRIT, got %+v", got)
		}
	})
}
