package etcd

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCollectUnknownMode(t *testing.T) {
	t.Parallel()
	_, err := Collect(context.Background(), Options{Mode: "weird"}, Reachers{})
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestCollectViaReacher(t *testing.T) {
	t.Parallel()
	called := false
	r := Reachers{
		ViaAPIServer: func(_ context.Context, _ Options) (*Status, error) {
			called = true
			return &Status{Mode: ModeViaAPIServer, Reachable: true, HasLeader: true, CollectedAt: time.Now()}, nil
		},
	}
	st, err := Collect(context.Background(), Options{Mode: ModeViaAPIServer}, r)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if !called {
		t.Fatal("ViaAPIServer not invoked")
	}
	if st.Mode != ModeViaAPIServer {
		t.Errorf("mode = %q", st.Mode)
	}
}

func TestCollectAutoTriesEachMode(t *testing.T) {
	t.Parallel()
	jobCalled := false
	apiCalled := false
	r := Reachers{
		InClusterJob: func(_ context.Context, _ Options) (*Status, error) {
			jobCalled = true
			return nil, ErrUnavailable
		},
		ViaAPIServer: func(_ context.Context, _ Options) (*Status, error) {
			apiCalled = true
			return &Status{Mode: ModeViaAPIServer, Reachable: true, HasLeader: true, CollectedAt: time.Now()}, nil
		},
	}
	_, err := Collect(context.Background(), Options{Mode: ModeAuto}, r)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if !jobCalled {
		t.Error("expected InClusterJob attempt")
	}
	if !apiCalled {
		t.Error("expected ViaAPIServer fallback")
	}
}

func TestCollectAutoNoReachers(t *testing.T) {
	t.Parallel()
	_, err := Collect(context.Background(), Options{Mode: ModeAuto}, Reachers{})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestCollectDirectMissingEndpoints(t *testing.T) {
	t.Parallel()
	_, err := Collect(context.Background(), Options{Mode: ModeDirect}, Reachers{})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestBuildTLSPartialOpts(t *testing.T) {
	t.Parallel()
	cfg, err := buildTLS(Options{})
	if err != nil || cfg != nil {
		t.Errorf("no opts: cfg=%v err=%v", cfg, err)
	}
	if _, err := buildTLS(Options{CAFile: "ca.pem"}); err == nil {
		t.Error("partial opts should error")
	}
}

func TestParseReadyzEtcd(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		body        string
		wantSeen    bool
		wantHealthy bool
	}{
		{"no etcd line", "[+]ping ok\n[+]log ok", false, false},
		{"healthy", "[+]etcd ok\n[+]other ok", true, true},
		{"unhealthy", "[-]etcd failed: dial timeout", true, false},
		{"empty body", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seen, healthy := parseReadyzEtcd([]byte(tc.body))
			if seen != tc.wantSeen || healthy != tc.wantHealthy {
				t.Errorf("got seen=%v healthy=%v want seen=%v healthy=%v",
					seen, healthy, tc.wantSeen, tc.wantHealthy)
			}
		})
	}
}

func TestBuildEtcdProbeJob(t *testing.T) {
	t.Parallel()
	j := buildEtcdProbeJob("probe", "kube-system", "ghcr.io/x:1")
	if j.Name != "probe" {
		t.Errorf("name = %q", j.Name)
	}
	if j.Namespace != "kube-system" {
		t.Errorf("ns = %q", j.Namespace)
	}
	if got := j.Spec.Template.Spec.Containers[0].Image; got != "ghcr.io/x:1" {
		t.Errorf("image = %q", got)
	}
	if !j.Spec.Template.Spec.HostNetwork {
		t.Error("expected HostNetwork=true")
	}
	if len(j.Spec.Template.Spec.Volumes) == 0 {
		t.Error("expected pki volume mount")
	}
}

func TestCollectDirectBadEndpoint(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := collectDirect(ctx, Options{
		Endpoints:   []string{"http://127.0.0.1:1"}, // closed port
		DialTimeout: 1 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error against closed endpoint")
	}
}

func TestBuildTLSAllSet(t *testing.T) {
	t.Parallel()
	// Reference nonexistent files; we expect a load error rather than a
	// "must all be set together" error.
	_, err := buildTLS(Options{
		CAFile:   "/nope/ca.pem",
		CertFile: "/nope/cert.pem",
		KeyFile:  "/nope/key.pem",
	})
	if err == nil {
		t.Fatal("expected load error")
	}
}

func TestInClusterJobNoImage(t *testing.T) {
	t.Parallel()
	r := InClusterJob(nil)
	_, err := r(context.Background(), Options{})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
}
