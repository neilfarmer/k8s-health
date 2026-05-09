// Package-level helpers + the etcd.health / etcd.size / etcd.defrag checks.
//
// All three call the same etcd.Collect() under the hood (see ADR-0005); they
// just interpret different facets of the resulting Status. To avoid making
// three separate API calls per khealth run, callers should run all etcd
// checks together, but for Phase-2 simplicity we accept the duplication —
// modest cost compared to the rest of the run.

package checks

import (
	"context"
	"errors"
	"fmt"

	"github.com/neilfarmer/k8s-health/internal/etcd"
	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() {
	Register(&etcdHealth{})
	Register(&etcdSize{})
	Register(&etcdDefrag{})
}

// collectEtcdFor builds Options and Reachers from env and runs etcd.Collect.
func collectEtcdFor(ctx context.Context, env *kube.Env) (*etcd.Status, error) {
	opts := etcd.Options{
		Mode:       etcd.Mode(env.Etcd.Mode),
		Endpoints:  env.Etcd.Endpoints,
		CAFile:     env.Etcd.CAFile,
		CertFile:   env.Etcd.CertFile,
		KeyFile:    env.Etcd.KeyFile,
		JobImage:   env.Etcd.JobImage,
		QuotaBytes: env.Etcd.QuotaBytes,
	}
	reachers := etcd.Reachers{
		ViaAPIServer: etcd.ViaAPIServer(env.Clientset),
		InClusterJob: etcd.InClusterJob(env.Clientset),
	}
	return etcd.Collect(ctx, opts, reachers)
}

// --- etcd.health -----------------------------------------------------------

type etcdHealth struct{}

func (etcdHealth) ID() string             { return "etcd.health" }
func (etcdHealth) Description() string    { return "etcd reachable, has leader, and no active alarms" }
func (etcdHealth) Categories() []Category { return []Category{CategoryControlPlane} }
func (etcdHealth) Requires() Capabilities { return CapAPIServer }

func (c etcdHealth) Run(ctx context.Context, env *kube.Env) []result.Finding {
	st, err := collectEtcdFor(ctx, env)
	if err != nil {
		if errors.Is(err, etcd.ErrUnavailable) {
			return []result.Finding{{Check: c.ID(), Status: result.StatusSkipped, Message: err.Error()}}
		}
		return unknownFromErr(c.ID(), "collect etcd", err)
	}
	detail := map[string]string{"mode": string(st.Mode)}
	if !st.Reachable {
		return []result.Finding{{Check: c.ID(), Status: result.StatusCritical, Message: "etcd not reachable", Detail: detail}}
	}
	if !st.HasLeader {
		return []result.Finding{{Check: c.ID(), Status: result.StatusCritical, Message: "etcd has no leader", Detail: detail}}
	}
	if len(st.Alarms) > 0 {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusCritical,
			Message: fmt.Sprintf("etcd has %d active alarm(s): %s", len(st.Alarms), formatAlarms(st.Alarms)),
			Detail:  detail,
		}}
	}
	return []result.Finding{{
		Check: c.ID(), Status: result.StatusOK,
		Message: fmt.Sprintf("etcd reachable, has leader, no alarms (mode=%s)", st.Mode),
		Detail:  detail,
	}}
}

func formatAlarms(alarms []etcd.Alarm) string {
	if len(alarms) == 0 {
		return ""
	}
	out := alarms[0].Type
	for i := 1; i < len(alarms); i++ {
		out += "," + alarms[i].Type
	}
	return out
}

// --- etcd.size -------------------------------------------------------------

type etcdSize struct{}

func (etcdSize) ID() string             { return "etcd.size" }
func (etcdSize) Description() string    { return "etcd backend DB size vs configured quota" }
func (etcdSize) Categories() []Category { return []Category{CategoryControlPlane} }
func (etcdSize) Requires() Capabilities { return CapAPIServer }

func (c etcdSize) Run(ctx context.Context, env *kube.Env) []result.Finding {
	st, err := collectEtcdFor(ctx, env)
	if err != nil {
		if errors.Is(err, etcd.ErrUnavailable) {
			return []result.Finding{{Check: c.ID(), Status: result.StatusSkipped, Message: err.Error()}}
		}
		return unknownFromErr(c.ID(), "collect etcd", err)
	}
	if st.SizeBytes == 0 {
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusSkipped,
			Message: fmt.Sprintf("etcd size unavailable in mode=%s", st.Mode),
		}}
	}
	pct := float64(st.SizeBytes) / float64(st.QuotaBytes) * 100
	detail := map[string]string{
		"mode":       string(st.Mode),
		"sizeBytes":  fmt.Sprintf("%d", st.SizeBytes),
		"quotaBytes": fmt.Sprintf("%d", st.QuotaBytes),
		"pctOfQuota": fmt.Sprintf("%.1f", pct),
	}
	switch {
	case pct >= 90:
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusCritical,
			Message: fmt.Sprintf("etcd db %.1f%% of quota (%s / %s)", pct, humanBytes(st.SizeBytes), humanBytes(st.QuotaBytes)),
			Detail:  detail,
		}}
	case pct >= 75:
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusWarning,
			Message: fmt.Sprintf("etcd db %.1f%% of quota (%s / %s)", pct, humanBytes(st.SizeBytes), humanBytes(st.QuotaBytes)),
			Detail:  detail,
		}}
	default:
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusOK,
			Message: fmt.Sprintf("etcd db %.1f%% of quota (%s / %s)", pct, humanBytes(st.SizeBytes), humanBytes(st.QuotaBytes)),
			Detail:  detail,
		}}
	}
}

// --- etcd.defrag -----------------------------------------------------------

type etcdDefrag struct{}

func (etcdDefrag) ID() string             { return "etcd.defrag" }
func (etcdDefrag) Description() string    { return "etcd fragmentation ratio (db-size vs in-use)" }
func (etcdDefrag) Categories() []Category { return []Category{CategoryControlPlane} }
func (etcdDefrag) Requires() Capabilities { return CapAPIServer }

// Warn at >25% fragmentation, crit at >50%.
const (
	defragWarnPct = 25.0
	defragCritPct = 50.0
)

func (c etcdDefrag) Run(ctx context.Context, env *kube.Env) []result.Finding {
	st, err := collectEtcdFor(ctx, env)
	if err != nil {
		if errors.Is(err, etcd.ErrUnavailable) {
			return []result.Finding{{Check: c.ID(), Status: result.StatusSkipped, Message: err.Error()}}
		}
		return unknownFromErr(c.ID(), "collect etcd", err)
	}
	if st.SizeBytes == 0 || st.SizeInUseBytes == 0 {
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusSkipped,
			Message: fmt.Sprintf("etcd size data unavailable in mode=%s", st.Mode),
		}}
	}
	frag := float64(st.SizeBytes-st.SizeInUseBytes) / float64(st.SizeBytes) * 100
	detail := map[string]string{
		"mode":           string(st.Mode),
		"sizeBytes":      fmt.Sprintf("%d", st.SizeBytes),
		"sizeInUseBytes": fmt.Sprintf("%d", st.SizeInUseBytes),
		"fragPct":        fmt.Sprintf("%.1f", frag),
	}
	switch {
	case frag >= defragCritPct:
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusCritical,
			Message: fmt.Sprintf("etcd %.1f%% fragmented (consider `etcdctl defrag`)", frag),
			Detail:  detail,
		}}
	case frag >= defragWarnPct:
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusWarning,
			Message: fmt.Sprintf("etcd %.1f%% fragmented", frag),
			Detail:  detail,
		}}
	default:
		return []result.Finding{{
			Check: c.ID(), Status: result.StatusOK,
			Message: fmt.Sprintf("etcd fragmentation %.1f%%", frag),
			Detail:  detail,
		}}
	}
}

// humanBytes formats n as a short binary-prefix string (KiB/MiB/GiB).
func humanBytes(n int64) string {
	const (
		k = int64(1024)
		m = k * 1024
		g = m * 1024
	)
	switch {
	case n >= g:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(g))
	case n >= m:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(m))
	case n >= k:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(k))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
