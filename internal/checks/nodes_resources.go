package checks

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() {
	Register(&nodesCPU{})
	Register(&nodesMemory{})
}

// nodeUsageWarnPct / nodeUsageCritPct are shared by the cpu and memory
// checks. Same band shape as etcd.size so reports stay predictable.
const (
	nodeUsageWarnPct = 75.0
	nodeUsageCritPct = 90.0
)

// nodeMetricsList mirrors metrics.k8s.io/v1beta1 NodeMetricsList. We
// hand-roll the type to avoid pulling in k8s.io/metrics; the apiserver
// proxies these as plain JSON.
type nodeMetricsList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Usage struct {
			CPU    string `json:"cpu"`
			Memory string `json:"memory"`
		} `json:"usage"`
	} `json:"items"`
}

// fetchNodeMetricsFn is the package-level indirection for tests.
var fetchNodeMetricsFn = fetchNodeMetrics

func fetchNodeMetrics(ctx context.Context, env *kube.Env) (*nodeMetricsList, error) {
	body, err := env.Clientset.Discovery().RESTClient().
		Get().
		AbsPath("/apis/metrics.k8s.io/v1beta1/nodes").
		DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	var list nodeMetricsList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse node metrics: %w", err)
	}
	return &list, nil
}

// nodeUsageData pairs a node with its parsed metrics + capacity, ready
// for percent computation.
type nodeUsageData struct {
	Name            string
	CPUUsageMilli   int64
	CPUCapMilli     int64
	MemUsageBytes   int64
	MemCapBytes     int64
}

func collectNodeUsage(ctx context.Context, env *kube.Env) ([]nodeUsageData, error) {
	metrics, err := fetchNodeMetricsFn(ctx, env)
	if err != nil {
		return nil, err
	}
	nodes, err := env.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	capByName := map[string]corev1.ResourceList{}
	for i := range nodes.Items {
		n := &nodes.Items[i]
		capByName[n.Name] = n.Status.Capacity
	}

	out := make([]nodeUsageData, 0, len(metrics.Items))
	for _, m := range metrics.Items {
		cap, ok := capByName[m.Metadata.Name]
		if !ok {
			continue
		}
		cpuU, _ := resource.ParseQuantity(m.Usage.CPU)
		memU, _ := resource.ParseQuantity(m.Usage.Memory)
		cpuC := cap[corev1.ResourceCPU]
		memC := cap[corev1.ResourceMemory]
		out = append(out, nodeUsageData{
			Name:          m.Metadata.Name,
			CPUUsageMilli: cpuU.MilliValue(),
			CPUCapMilli:   cpuC.MilliValue(),
			MemUsageBytes: memU.Value(),
			MemCapBytes:   memC.Value(),
		})
	}
	return out, nil
}

// classifyPct buckets a percent value into a Status.
func classifyPct(pct float64) result.Status {
	switch {
	case pct >= nodeUsageCritPct:
		return result.StatusCritical
	case pct >= nodeUsageWarnPct:
		return result.StatusWarning
	default:
		return result.StatusOK
	}
}

func nodeUsageSkip(id string, err error) []result.Finding {
	if apierrors.IsNotFound(err) || apierrors.IsServiceUnavailable(err) {
		return []result.Finding{{
			Check:   id,
			Status:  result.StatusSkipped,
			Message: "metrics.k8s.io API unavailable (metrics-server not installed?)",
		}}
	}
	return unknownFromErr(id, "node metrics", err)
}

// --- nodes.cpu -------------------------------------------------------------

type nodesCPU struct{}

func (nodesCPU) ID() string             { return "nodes.cpu" }
func (nodesCPU) Description() string    { return "per-node CPU usage vs capacity (via metrics-server)" }
func (nodesCPU) Categories() []Category { return []Category{CategoryNode} }
func (nodesCPU) Requires() Capabilities { return CapAPIServer }

func (c nodesCPU) Run(ctx context.Context, env *kube.Env) []result.Finding {
	usage, err := collectNodeUsage(ctx, env)
	if err != nil {
		return nodeUsageSkip(c.ID(), err)
	}
	if len(usage) == 0 {
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusSkipped,
			Message: "no node metrics returned",
		}}
	}
	out := make([]result.Finding, 0, len(usage))
	for _, u := range usage {
		if u.CPUCapMilli == 0 {
			continue
		}
		pct := float64(u.CPUUsageMilli) / float64(u.CPUCapMilli) * 100
		out = append(out, result.Finding{
			Check:    c.ID(),
			Status:   classifyPct(pct),
			Resource: "node/" + u.Name,
			Message: fmt.Sprintf("CPU %.1f%% (%dm / %dm)",
				pct, u.CPUUsageMilli, u.CPUCapMilli),
			Detail: map[string]string{
				"usageMilli": fmt.Sprintf("%d", u.CPUUsageMilli),
				"capMilli":   fmt.Sprintf("%d", u.CPUCapMilli),
				"pct":        fmt.Sprintf("%.1f", pct),
			},
		})
	}
	return out
}

// --- nodes.memory ----------------------------------------------------------

type nodesMemory struct{}

func (nodesMemory) ID() string             { return "nodes.memory" }
func (nodesMemory) Description() string    { return "per-node memory usage vs capacity (via metrics-server)" }
func (nodesMemory) Categories() []Category { return []Category{CategoryNode} }
func (nodesMemory) Requires() Capabilities { return CapAPIServer }

func (c nodesMemory) Run(ctx context.Context, env *kube.Env) []result.Finding {
	usage, err := collectNodeUsage(ctx, env)
	if err != nil {
		return nodeUsageSkip(c.ID(), err)
	}
	if len(usage) == 0 {
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusSkipped,
			Message: "no node metrics returned",
		}}
	}
	out := make([]result.Finding, 0, len(usage))
	for _, u := range usage {
		if u.MemCapBytes == 0 {
			continue
		}
		pct := float64(u.MemUsageBytes) / float64(u.MemCapBytes) * 100
		out = append(out, result.Finding{
			Check:    c.ID(),
			Status:   classifyPct(pct),
			Resource: "node/" + u.Name,
			Message: fmt.Sprintf("memory %.1f%% (%s / %s)",
				pct, humanBytes(u.MemUsageBytes), humanBytes(u.MemCapBytes)),
			Detail: map[string]string{
				"usageBytes": fmt.Sprintf("%d", u.MemUsageBytes),
				"capBytes":   fmt.Sprintf("%d", u.MemCapBytes),
				"pct":        fmt.Sprintf("%.1f", pct),
			},
		})
	}
	return out
}
