package checks

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
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

// Spin up an httptest TLS server, install its cert as the env's CA, and
// verify the check parses the leaf via dialAndReadLeaf without disabling
// TLS verification.
func TestApiserverCertExpiryEndToEnd(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(nil)
	t.Cleanup(srv.Close)

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})

	env := &kube.Env{
		Config: &rest.Config{
			Host: srv.URL,
			TLSClientConfig: rest.TLSClientConfig{
				CAData:     caPEM,
				ServerName: "127.0.0.1", // httptest cert is issued for "127.0.0.1"
			},
		},
		Clientset: fake.NewSimpleClientset(),
	}
	got := runCheck(t, &apiserverCertExpiry{}, env)
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %+v", got)
	}
	// httptest cert is valid for the lifetime of the test (months/years
	// depending on Go version), so OK or WARN is expected.
	if got[0].Status != result.StatusOK && got[0].Status != result.StatusWarning {
		t.Fatalf("want OK or WARN, got %s (%s)", got[0].Status, got[0].Message)
	}
}

func TestIsCertExpiredErr(t *testing.T) {
	t.Parallel()
	if isCertExpiredErr(nil) {
		t.Error("nil err should not be flagged as expired")
	}
	if !isCertExpiredErr(&x509.CertificateInvalidError{Reason: x509.Expired}) {
		t.Error("Expired Reason should be flagged")
	}
	if isCertExpiredErr(&x509.CertificateInvalidError{Reason: x509.NotAuthorizedToSign}) {
		t.Error("non-Expired Reason should not be flagged")
	}
}
