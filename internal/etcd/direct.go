package etcd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func collectDirect(ctx context.Context, opts Options) (*Status, error) {
	if len(opts.Endpoints) == 0 {
		return nil, fmt.Errorf("%w: direct mode requires --etcd-endpoints", ErrUnavailable)
	}

	tlsCfg, err := buildTLS(opts)
	if err != nil {
		return nil, fmt.Errorf("etcd: tls: %w", err)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   opts.Endpoints,
		DialTimeout: opts.DialTimeout,
		TLS:         tlsCfg,
		Context:     ctx,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer func() { _ = cli.Close() }()

	st := &Status{
		Mode:        ModeDirect,
		CollectedAt: time.Now(),
		QuotaBytes:  opts.QuotaBytes,
	}

	// Smoke that the cluster responds; we don't need the membership detail
	// itself for Phase-2 checks (size + alarms + has-leader).
	listCtx, cancel := context.WithTimeout(ctx, opts.DialTimeout)
	if _, err := cli.MemberList(listCtx); err != nil {
		cancel()
		return nil, fmt.Errorf("%w: member list: %w", ErrUnavailable, err)
	}
	cancel()

	// Per-endpoint Status() so we get DB size per member.
	for _, ep := range opts.Endpoints {
		ms := MemberStatus{Endpoint: ep}
		stCtx, c := context.WithTimeout(ctx, opts.DialTimeout)
		s, err := cli.Status(stCtx, ep)
		c()
		if err != nil {
			ms.Errors = append(ms.Errors, err.Error())
			st.Members = append(st.Members, ms)
			continue
		}
		ms.Reachable = true
		ms.Version = s.Version
		ms.DBSize = s.DbSize
		ms.DBSizeInUse = s.DbSizeInUse
		ms.LeaderID = s.Leader
		ms.RaftIndex = s.RaftIndex
		st.SizeBytes += s.DbSize
		st.SizeInUseBytes += s.DbSizeInUse
		if s.Leader != 0 {
			st.HasLeader = true
		}
		st.Reachable = true
		st.Members = append(st.Members, ms)
	}

	// Alarms — single call against the cluster.
	alCtx, alCancel := context.WithTimeout(ctx, opts.DialTimeout)
	if al, err := cli.AlarmList(alCtx); err == nil {
		for _, a := range al.Alarms {
			st.Alarms = append(st.Alarms, Alarm{MemberID: a.MemberID, Type: a.Alarm.String()})
		}
	}
	alCancel()

	if !st.Reachable {
		return st, fmt.Errorf("%w: no member responded to Status()", ErrUnavailable)
	}
	return st, nil
}

func buildTLS(opts Options) (*tls.Config, error) {
	if opts.CAFile == "" && opts.CertFile == "" && opts.KeyFile == "" {
		return nil, nil
	}
	if opts.CAFile == "" || opts.CertFile == "" || opts.KeyFile == "" {
		return nil, errors.New("--etcd-cacert, --etcd-cert and --etcd-key must all be set together")
	}
	info := transport.TLSInfo{
		CertFile:      opts.CertFile,
		KeyFile:       opts.KeyFile,
		TrustedCAFile: opts.CAFile,
	}
	cfg, err := info.ClientConfig()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
