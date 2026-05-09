package checks

import (
	"context"
	"testing"

	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseDistro(t *testing.T) {
	t.Parallel()
	cases := map[string]Distro{
		"":        DistroAuto,
		"auto":    DistroAuto,
		"rke2":    DistroRKE2,
		"k3s":     DistroK3s,
		"kubeadm": DistroKubeadm,
		"eks":     DistroEKS,
	}
	for in, want := range cases {
		got, err := ParseDistro(in)
		if err != nil {
			t.Errorf("ParseDistro(%q) err: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseDistro(%q) = %s, want %s", in, got, want)
		}
	}
	if _, err := ParseDistro("openshift"); err == nil {
		t.Error("want error for unknown distro, got nil")
	}
}

func TestAppliesToDistro(t *testing.T) {
	t.Parallel()
	rke2 := rke2HelmInstallJobs{}
	if !AppliesToDistro(rke2, DistroRKE2) {
		t.Error("rke2 check should apply to rke2")
	}
	if AppliesToDistro(rke2, DistroEKS) {
		t.Error("rke2 check should not apply to eks")
	}
	generic := corednsReplicas{}
	for _, d := range AllDistros {
		if !AppliesToDistro(generic, d) {
			t.Errorf("generic check should apply to %s", d)
		}
	}
}

func TestFilterGatesByDistro(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(corednsReplicas{})
	r.Register(rke2HelmInstallJobs{})

	rke2List := r.Filter(nil, nil, nil, DistroRKE2)
	if len(rke2List) != 2 {
		t.Errorf("rke2: want 2 checks, got %d", len(rke2List))
	}
	eksList := r.Filter(nil, nil, nil, DistroEKS)
	if len(eksList) != 1 {
		t.Errorf("eks: want 1 check (generic only), got %d", len(eksList))
	}
	autoList := r.Filter(nil, nil, nil, DistroAuto)
	if len(autoList) != 2 {
		t.Errorf("auto: want all checks (no gating), got %d", len(autoList))
	}
}

func TestDetectDistroRKE2(t *testing.T) {
	t.Parallel()
	env := envWithObjects(&coordv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "rke2"},
	})
	if got := DetectDistro(context.Background(), env); got != DistroRKE2 {
		t.Errorf("want rke2, got %s", got)
	}
}

func TestDetectDistroK3s(t *testing.T) {
	t.Parallel()
	env := envWithObjects(&coordv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "k3s"},
	})
	if got := DetectDistro(context.Background(), env); got != DistroK3s {
		t.Errorf("want k3s, got %s", got)
	}
}

func TestDetectDistroEKS(t *testing.T) {
	t.Parallel()
	env := envWithObjects(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ip-10-0-0-1.ec2.internal",
			Labels: map[string]string{"eks.amazonaws.com/nodegroup": "default"},
		},
	})
	if got := DetectDistro(context.Background(), env); got != DistroEKS {
		t.Errorf("want eks, got %s", got)
	}
}

func TestDetectDistroDefaultsToKubeadm(t *testing.T) {
	t.Parallel()
	env := envWithObjects()
	if got := DetectDistro(context.Background(), env); got != DistroKubeadm {
		t.Errorf("want kubeadm default, got %s", got)
	}
}
