package checks

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&nodesVersionSkew{}) }

// kubeletMinorSkew is the maximum allowed kubelet minor lag behind the
// control plane per the Kubernetes version-skew policy. As of v1.28 this is
// 3 (was 2). We warn at 2 and crit at >3 for early signal.
const (
	kubeletWarnSkew = 2
	kubeletCritSkew = 3
)

type nodesVersionSkew struct{}

func (nodesVersionSkew) ID() string             { return "nodes.versionSkew" }
func (nodesVersionSkew) Description() string    { return "Kubelet version skew vs the API server" }
func (nodesVersionSkew) Categories() []Category { return []Category{CategoryNode} }
func (nodesVersionSkew) Requires() Capabilities { return CapAPIServer }

func (c nodesVersionSkew) Run(ctx context.Context, env *kube.Env) []result.Finding {
	apiVer, err := env.Discovery.ServerVersion()
	if err != nil {
		return unknownFromErr(c.ID(), "discover api version", err)
	}
	apiMinor, err := parseMinor(apiVer.Minor)
	if err != nil {
		return unknownFromErr(c.ID(), "parse apiserver minor", err)
	}

	nodes, err := env.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list nodes", err)
	}
	var out []result.Finding
	for i := range nodes.Items {
		n := &nodes.Items[i]
		kubeletVer := n.Status.NodeInfo.KubeletVersion
		nodeMinor, err := parseSemverMinor(kubeletVer)
		if err != nil {
			out = append(out, result.Finding{
				Check:    c.ID(),
				Status:   result.StatusUnknown,
				Resource: resourceID("node", "", n.Name),
				Message:  fmt.Sprintf("cannot parse kubelet version %q: %v", kubeletVer, err),
			})
			continue
		}
		skew := apiMinor - nodeMinor
		if skew <= 0 {
			continue
		}
		var status result.Status
		switch {
		case skew > kubeletCritSkew:
			status = result.StatusCritical
		case skew >= kubeletWarnSkew:
			status = result.StatusWarning
		default:
			continue
		}
		out = append(out, result.Finding{
			Check:    c.ID(),
			Status:   status,
			Resource: resourceID("node", "", n.Name),
			Message:  fmt.Sprintf("kubelet %s lags apiserver v1.%d by %d minor", kubeletVer, apiMinor, skew),
			Detail: map[string]string{
				"kubelet":   kubeletVer,
				"apiserver": fmt.Sprintf("v1.%d", apiMinor),
				"minorSkew": strconv.Itoa(skew),
			},
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(),
			fmt.Sprintf("all kubelets within skew policy of apiserver v1.%d", apiMinor))}
	}
	return out
}

// parseMinor reads a Discovery.ServerVersion().Minor field, which can look
// like "31" or "31+" on managed clusters.
func parseMinor(s string) (int, error) {
	s = strings.TrimRight(strings.TrimSpace(s), "+")
	if s == "" {
		return 0, fmt.Errorf("empty minor")
	}
	return strconv.Atoi(s)
}

// parseSemverMinor extracts N from "v1.N.M[-suffix]".
func parseSemverMinor(v string) (int, error) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, fmt.Errorf("not a semver: %q", v)
	}
	return strconv.Atoi(parts[1])
}
