// Package etcd provides three ways to read etcd health and size, per
// ADR-0005:
//
//  1. Direct gRPC: caller supplies endpoints + client TLS material.
//  2. via-apiserver: parse the API server's /readyz?verbose. Lower
//     fidelity but works everywhere, including managed clusters.
//  3. In-cluster Job: the runner schedules a privileged Job in
//     kube-system that mounts /etc/kubernetes/pki/etcd from the host
//     and runs `khealth etcd-probe`, which itself uses the direct mode.
//
// Mode "auto" tries direct first (if endpoints are configured), then
// in-cluster Job (if launch-mode=in-cluster), falling back to
// via-apiserver. The mode that produced a Status is reported in
// Status.Mode so checks can include it in the finding detail.
package etcd

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Mode names match the --etcd-mode CLI flag values.
type Mode string

// Defined etcd collection modes; see the package doc comment.
const (
	ModeAuto         Mode = "auto"
	ModeDirect       Mode = "direct"
	ModeInClusterJob Mode = "in-cluster"
	ModeViaAPIServer Mode = "via-apiserver"
	ModePodExec      Mode = "pod-exec"
)

// MemberStatus is one etcd member's view from Maintenance.Status.
type MemberStatus struct {
	Endpoint    string   `json:"endpoint"`
	Version     string   `json:"version,omitempty"`
	DBSize      int64    `json:"dbSize,omitempty"`      // bytes
	DBSizeInUse int64    `json:"dbSizeInUse,omitempty"` // bytes
	LeaderID    uint64   `json:"leaderID,omitempty"`
	RaftIndex   uint64   `json:"raftIndex,omitempty"`
	Errors      []string `json:"errors,omitempty"`
	Reachable   bool     `json:"reachable"`
}

// Alarm is a server-side etcd alarm (NOSPACE, CORRUPT, etc.).
type Alarm struct {
	MemberID uint64 `json:"memberID"`
	Type     string `json:"type"`
}

// Status is what every collection mode returns. Modes that can't fill a
// field (e.g. via-apiserver has no DBSize) leave it zero.
type Status struct {
	Mode        Mode      `json:"mode"`
	CollectedAt time.Time `json:"collectedAt"`

	// Reachable is true iff at least one signal indicates etcd is up.
	Reachable bool `json:"reachable"`

	// Members and Alarms are only populated by direct/in-cluster modes.
	Members []MemberStatus `json:"members,omitempty"`
	Alarms  []Alarm        `json:"alarms,omitempty"`

	// HasLeader is true if at least one member reports a leader. Set by
	// every mode (via-apiserver infers from /readyz).
	HasLeader bool `json:"hasLeader"`

	// QuotaBytes is the configured backend quota (bytes). Direct/in-cluster
	// only — read from /etc/etcd or via Maintenance.Status not, today, set
	// to a configurable default by the runner.
	QuotaBytes int64 `json:"quotaBytes,omitempty"`

	// SizeBytes / SizeInUseBytes summed across members (direct/in-cluster).
	SizeBytes      int64 `json:"sizeBytes,omitempty"`
	SizeInUseBytes int64 `json:"sizeInUseBytes,omitempty"`
}

// ErrUnavailable signals a particular mode could not reach etcd. Callers
// can use this with errors.Is to fall back to the next mode.
var ErrUnavailable = errors.New("etcd: not reachable in this mode")

// Options configure how Collect resolves a mode.
type Options struct {
	Mode      Mode
	Endpoints []string // direct mode
	CAFile    string   // direct mode
	CertFile  string   // direct mode
	KeyFile   string   // direct mode

	// QuotaBytes is the backend quota threshold to compare against.
	// Defaults to 8 GiB (the etcd default).
	QuotaBytes int64

	// DialTimeout caps the per-mode attempt.
	DialTimeout time.Duration

	// JobImage is the container image used by the in-cluster Job mode.
	JobImage string

	// JobNamespace is where the Job is scheduled (default kube-system).
	JobNamespace string
}

// Collect reads etcd state using the configured mode. The caller supplies a
// reachers value to plug in Kubernetes-aware modes (in-cluster Job, via-apiserver)
// without making this package import client-go directly.
func Collect(ctx context.Context, opts Options, reachers Reachers) (*Status, error) {
	if opts.QuotaBytes == 0 {
		opts.QuotaBytes = 8 * 1024 * 1024 * 1024 // etcd default
	}
	if opts.DialTimeout == 0 {
		opts.DialTimeout = 10 * time.Second
	}

	switch opts.Mode {
	case ModeDirect:
		return collectDirect(ctx, opts)
	case ModeInClusterJob:
		return reachers.InClusterJob(ctx, opts)
	case ModeViaAPIServer:
		return reachers.ViaAPIServer(ctx, opts)
	case ModePodExec:
		if reachers.PodExec == nil {
			return nil, fmt.Errorf("%w: pod-exec reacher not configured", ErrUnavailable)
		}
		return reachers.PodExec(ctx, opts)
	case ModeAuto, "":
		return collectAuto(ctx, opts, reachers)
	}
	return nil, errors.New("etcd: unknown mode " + string(opts.Mode))
}

// Reachers are the Kubernetes-aware collection paths. They're injected by
// the caller so this package stays test-friendly without a fake clientset.
type Reachers struct {
	InClusterJob func(ctx context.Context, opts Options) (*Status, error)
	ViaAPIServer func(ctx context.Context, opts Options) (*Status, error)
	PodExec      func(ctx context.Context, opts Options) (*Status, error)
}

func collectAuto(ctx context.Context, opts Options, reachers Reachers) (*Status, error) {
	if len(opts.Endpoints) > 0 {
		s, err := collectDirect(ctx, opts)
		if err == nil {
			return s, nil
		}
		if !errors.Is(err, ErrUnavailable) {
			return s, err
		}
	}
	if reachers.PodExec != nil {
		s, err := reachers.PodExec(ctx, opts)
		if err == nil {
			return s, nil
		}
		if !errors.Is(err, ErrUnavailable) {
			return s, err
		}
	}
	if reachers.InClusterJob != nil {
		s, err := reachers.InClusterJob(ctx, opts)
		if err == nil {
			return s, nil
		}
		if !errors.Is(err, ErrUnavailable) {
			return s, err
		}
	}
	if reachers.ViaAPIServer != nil {
		return reachers.ViaAPIServer(ctx, opts)
	}
	return nil, ErrUnavailable
}
