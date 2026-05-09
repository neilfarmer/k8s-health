package checks

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&nodesDisk{}) }

// kubeletStatsSummary mirrors the parts of kubelet's /stats/summary
// shape we care about. Hand-rolled to avoid pulling kubelet API deps.
type kubeletStatsSummary struct {
	Node struct {
		NodeName string   `json:"nodeName"`
		FS       *fsStats `json:"fs,omitempty"`
		Runtime  struct {
			ImageFS *fsStats `json:"imageFs,omitempty"`
		} `json:"runtime,omitempty"`
	} `json:"node"`
}

type fsStats struct {
	AvailableBytes int64 `json:"availableBytes"`
	CapacityBytes  int64 `json:"capacityBytes"`
	UsedBytes      int64 `json:"usedBytes"`
}

// fetchNodeStatsFn is the package-level indirection for tests.
var fetchNodeStatsFn = fetchNodeStats

func fetchNodeStats(ctx context.Context, env *kube.Env, nodeName string) (*kubeletStatsSummary, error) {
	body, err := env.Clientset.Discovery().RESTClient().
		Get().
		AbsPath(fmt.Sprintf("/api/v1/nodes/%s/proxy/stats/summary", nodeName)).
		DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	var s kubeletStatsSummary
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("parse kubelet stats: %w", err)
	}
	return &s, nil
}

type nodesDisk struct{}

func (nodesDisk) ID() string { return "nodes.disk" }
func (nodesDisk) Description() string {
	return "per-node root filesystem and image filesystem usage (via kubelet stats)"
}
func (nodesDisk) Categories() []Category { return []Category{CategoryNode} }
func (nodesDisk) Requires() Capabilities { return CapAPIServer }

func (c nodesDisk) Run(ctx context.Context, env *kube.Env) []result.Finding {
	nodes, err := env.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list nodes", err)
	}
	out := []result.Finding{}
	for i := range nodes.Items {
		n := &nodes.Items[i]
		if !nodeReady(n) {
			continue
		}
		stats, err := fetchNodeStatsFn(ctx, env, n.Name)
		if err != nil {
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusUnknown,
				Resource: "node/" + n.Name,
				Message:  fmt.Sprintf("kubelet stats unavailable: %v", err),
			})
			continue
		}
		out = append(out, classifyFS(c.ID(), n.Name, "rootfs", stats.Node.FS)...)
		out = append(out, classifyFS(c.ID(), n.Name, "imagefs", stats.Node.Runtime.ImageFS)...)
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no node disk pressure")}
	}
	healthy := true
	for _, f := range out {
		if f.Status != result.StatusOK {
			healthy = false
			break
		}
	}
	if healthy {
		return []result.Finding{okFinding(c.ID(),
			fmt.Sprintf("all %d node fs measurements OK", len(out)))}
	}
	return out
}

func classifyFS(id, nodeName, label string, fs *fsStats) []result.Finding {
	if fs == nil || fs.CapacityBytes == 0 {
		return nil
	}
	pct := float64(fs.UsedBytes) / float64(fs.CapacityBytes) * 100
	status := classifyPct(pct)
	if status == result.StatusOK {
		return nil
	}
	return []result.Finding{{
		Check: id, Status: status,
		Resource: fmt.Sprintf("node/%s/%s", nodeName, label),
		Message: fmt.Sprintf("%s %.1f%% used (%s / %s)",
			label, pct, humanBytes(fs.UsedBytes), humanBytes(fs.CapacityBytes)),
		Detail: map[string]string{
			"used":     fmt.Sprintf("%d", fs.UsedBytes),
			"capacity": fmt.Sprintf("%d", fs.CapacityBytes),
			"pct":      fmt.Sprintf("%.1f", pct),
		},
	}}
}

func nodeReady(n *corev1.Node) bool {
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
