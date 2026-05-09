package checks

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http/httptest"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestCertFinding(t *testing.T) {
	t.Parallel()
	now := time.Now()
	cases := []struct {
		name string
		na   time.Time
		want result.Status
	}{
		{"healthy", now.Add(120 * 24 * time.Hour), result.StatusOK},
		{"warn window", now.Add(15 * 24 * time.Hour), result.StatusWarning},
		{"crit window", now.Add(5 * 24 * time.Hour), result.StatusCritical},
		{"expired", now.Add(-1 * time.Hour), result.StatusCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cert := &x509.Certificate{
				NotAfter: tc.na,
				Subject:  pkix.Name{CommonName: "kube-apiserver"},
			}
			f := certFinding("apiserver.certExpiry", "1.2.3.4:443", cert)
			if f.Status != tc.want {
				t.Errorf("got %s want %s", f.Status, tc.want)
			}
		})
	}
}

// Spin up an httptest TLS server with a freshly generated cert and verify
// the check parses the leaf via dialAndReadLeaf.
func TestApiserverCertExpiryEndToEnd(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(nil)
	t.Cleanup(srv.Close)

	env := &kube.Env{
		Config:    &rest.Config{Host: srv.URL},
		Clientset: fake.NewSimpleClientset(),
	}
	got := runCheck(t, &apiserverCertExpiry{}, env)
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %+v", got)
	}
	// httptest cert is valid for the lifetime of the test (months/years
	// depending on Go version), so OK is expected.
	if got[0].Status != result.StatusOK && got[0].Status != result.StatusWarning {
		t.Fatalf("want OK or WARN, got %s (%s)", got[0].Status, got[0].Message)
	}
}

// helper to silence unused-import warning if x509-related imports drift.
var _ = func() *x509.Certificate {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	cert, _ := x509.ParseCertificate(der)
	return cert
}
