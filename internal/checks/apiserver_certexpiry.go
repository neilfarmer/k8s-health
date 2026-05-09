package checks

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"time"

	"k8s.io/client-go/transport"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&apiserverCertExpiry{}) }

// Threshold defaults: warn 30 days out, crit 7 days out. Tunable later.
const (
	certWarnWindow = 30 * 24 * time.Hour
	certCritWindow = 7 * 24 * time.Hour
)

type apiserverCertExpiry struct{}

func (apiserverCertExpiry) ID() string             { return "apiserver.certExpiry" }
func (apiserverCertExpiry) Description() string    { return "API server TLS leaf cert expiry" }
func (apiserverCertExpiry) Categories() []Category { return []Category{CategoryControlPlane} }
func (apiserverCertExpiry) Requires() Capabilities { return CapAPIServer }

func (c apiserverCertExpiry) Run(ctx context.Context, env *kube.Env) []result.Finding {
	if env.Config == nil {
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusSkipped,
			Message: "no rest.Config (fake clientset)",
		}}
	}
	host, err := dialAddr(env.Config.Host)
	if err != nil {
		return []result.Finding{{Check: c.ID(), Status: result.StatusUnknown, Message: err.Error()}}
	}
	tCfg, err := env.Config.TransportConfig()
	if err != nil {
		return unknownFromErr(c.ID(), "build transport config", err)
	}
	tlsCfg, err := transport.TLSConfigFor(tCfg)
	if err != nil {
		return unknownFromErr(c.ID(), "build TLS config", err)
	}
	if tlsCfg == nil {
		tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	tlsCfg = tlsCfg.Clone()
	// We don't need verified chain for expiry; allow self-signed. The leaf
	// cert is what we read regardless.
	tlsCfg.InsecureSkipVerify = true
	tlsCfg.VerifyPeerCertificate = nil
	if tlsCfg.MinVersion == 0 {
		tlsCfg.MinVersion = tls.VersionTLS12
	}

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	leaf, err := dialAndReadLeaf(dialCtx, host, tlsCfg)
	if err != nil {
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusUnknown,
			Message: fmt.Sprintf("TLS dial %s: %v", host, err),
		}}
	}
	return []result.Finding{certFinding(c.ID(), host, leaf)}
}

func dialAddr(rawHost string) (string, error) {
	u, err := url.Parse(rawHost)
	if err != nil {
		return "", fmt.Errorf("parse host %q: %w", rawHost, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("apiserver host has no authority: %q", rawHost)
	}
	if hostPortHasPort(u.Host) {
		return u.Host, nil
	}
	return u.Host + ":443", nil
}

// hostPortHasPort reports whether s already includes an explicit port. We
// use net.SplitHostPort as the "is port present" probe.
func hostPortHasPort(s string) bool {
	_, _, err := net.SplitHostPort(s)
	return err == nil
}

func dialAndReadLeaf(ctx context.Context, addr string, tlsCfg *tls.Config) (*x509.Certificate, error) {
	d := &tls.Dialer{Config: tlsCfg}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("not a TLS connection")
	}
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, fmt.Errorf("no peer certificates")
	}
	return state.PeerCertificates[0], nil
}

func certFinding(id, host string, leaf *x509.Certificate) result.Finding {
	until := time.Until(leaf.NotAfter)
	detail := map[string]string{
		"subject":  leaf.Subject.CommonName,
		"notAfter": leaf.NotAfter.UTC().Format(time.RFC3339),
		"daysLeft": fmt.Sprintf("%.1f", until.Hours()/24),
	}
	switch {
	case until <= 0:
		return result.Finding{
			Check: id, Status: result.StatusCritical,
			Resource: host, Detail: detail,
			Message: "API server TLS cert is expired",
		}
	case until < certCritWindow:
		return result.Finding{
			Check: id, Status: result.StatusCritical,
			Resource: host, Detail: detail,
			Message: fmt.Sprintf("expires in %s", until.Round(time.Hour)),
		}
	case until < certWarnWindow:
		return result.Finding{
			Check: id, Status: result.StatusWarning,
			Resource: host, Detail: detail,
			Message: fmt.Sprintf("expires in %s", until.Round(time.Hour)),
		}
	default:
		return result.Finding{
			Check: id, Status: result.StatusOK,
			Resource: host, Detail: detail,
			Message: fmt.Sprintf("expires in %s", until.Round(24*time.Hour)),
		}
	}
}
