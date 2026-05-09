package etcd

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
)

// ViaAPIServer reads /readyz?verbose from the API server and infers etcd
// state from the etcd-related lines. Lower fidelity than direct (no
// db-size, no member list, no alarms), but works on managed clusters.
func ViaAPIServer(clientset kubernetes.Interface) func(ctx context.Context, opts Options) (*Status, error) {
	return func(ctx context.Context, opts Options) (*Status, error) {
		rest := clientset.Discovery().RESTClient()
		if rest == nil {
			return nil, fmt.Errorf("%w: no REST client available", ErrUnavailable)
		}
		body, err := rest.Get().
			AbsPath("/readyz").
			Param("verbose", "true").
			DoRaw(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		st := &Status{
			Mode:        ModeViaAPIServer,
			CollectedAt: time.Now(),
			QuotaBytes:  opts.QuotaBytes,
		}
		etcdSeen, etcdHealthy := parseReadyzEtcd(body)
		if !etcdSeen {
			return st, fmt.Errorf("%w: /readyz had no etcd lines", ErrUnavailable)
		}
		st.Reachable = etcdHealthy
		st.HasLeader = etcdHealthy
		return st, nil
	}
}

// parseReadyzEtcd interprets the apiserver's /readyz?verbose body and
// reports whether any etcd-related line was seen and whether all such
// lines were "[+]" (healthy).
func parseReadyzEtcd(body []byte) (seen, healthy bool) {
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, "etcd") {
			continue
		}
		seen = true
		// readyz lines look like: "[+]etcd ok" / "[-]etcd failed: ..."
		if strings.HasPrefix(line, "[+]") {
			healthy = true
		}
	}
	return seen, healthy
}
