package checks

import (
	"context"
	"testing"

	"github.com/neilfarmer/k8s-health/internal/etcd"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestFormatAlarms(t *testing.T) {
	t.Parallel()
	if got := formatAlarms(nil); got != "" {
		t.Errorf("nil alarms: %q", got)
	}
	got := formatAlarms([]etcd.Alarm{{Type: "NOSPACE"}, {Type: "CORRUPT"}})
	if got != "NOSPACE,CORRUPT" {
		t.Errorf("got %q", got)
	}
}

func TestHumanBytes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{2048, "2.0 KiB"},
		{2 * 1024 * 1024, "2.0 MiB"},
		{3 * 1024 * 1024 * 1024, "3.0 GiB"},
	}
	for _, tc := range cases {
		if got := humanBytes(tc.in); got != tc.want {
			t.Errorf("humanBytes(%d) = %q want %q", tc.in, got, tc.want)
		}
	}
}

// Etcd checks against fake clientset will hit collectEtcdFor which calls
// etcd.Collect → ViaAPIServer (the only mode that fake supports). Fake's
// /readyz proxy returns 404, so we expect a SKIP finding.
func TestEtcdChecksSkipAgainstFakeClient(t *testing.T) {
	t.Parallel()
	for _, c := range []Check{&etcdHealth{}, &etcdSize{}, &etcdDefrag{}, &etcdMembers{}} {
		got := runCheck(t, c, envWithObjects())
		// Either SKIP (mode unavailable) or UNKNOWN (parse error). Both
		// non-blocking; we just want to make sure they don't panic and
		// don't claim CRIT against the fake.
		if statusCounts(got)[result.StatusCritical] > 0 {
			t.Errorf("%s: unexpected CRIT against fake client: %+v", c.ID(), got)
		}
	}
}

func TestEtcdHealthHappyPath(t *testing.T) {
	orig := collectEtcdFn
	t.Cleanup(func() { collectEtcdFn = orig })
	collectEtcdFn = func(_ context.Context, _ etcd.Options, _ etcd.Reachers) (*etcd.Status, error) {
		return &etcd.Status{Mode: etcd.ModeViaAPIServer, Reachable: true, HasLeader: true}, nil
	}
	got := runCheck(t, &etcdHealth{}, envWithObjects())
	if statusCounts(got)[result.StatusOK] != 1 {
		t.Fatalf("want OK, got %+v", got)
	}
}

func TestEtcdHealthAlarms(t *testing.T) {
	orig := collectEtcdFn
	t.Cleanup(func() { collectEtcdFn = orig })
	collectEtcdFn = func(_ context.Context, _ etcd.Options, _ etcd.Reachers) (*etcd.Status, error) {
		return &etcd.Status{
			Mode: etcd.ModeDirect, Reachable: true, HasLeader: true,
			Alarms: []etcd.Alarm{{Type: "NOSPACE", MemberID: 1}},
		}, nil
	}
	got := runCheck(t, &etcdHealth{}, envWithObjects())
	if statusCounts(got)[result.StatusCritical] != 1 {
		t.Fatalf("want CRIT for alarm, got %+v", got)
	}
}

func TestEtcdHealthNoLeader(t *testing.T) {
	orig := collectEtcdFn
	t.Cleanup(func() { collectEtcdFn = orig })
	collectEtcdFn = func(_ context.Context, _ etcd.Options, _ etcd.Reachers) (*etcd.Status, error) {
		return &etcd.Status{Mode: etcd.ModeDirect, Reachable: true, HasLeader: false}, nil
	}
	got := runCheck(t, &etcdHealth{}, envWithObjects())
	if statusCounts(got)[result.StatusCritical] != 1 {
		t.Fatalf("want CRIT, got %+v", got)
	}
}

func TestEtcdSizeBands(t *testing.T) {
	orig := collectEtcdFn
	t.Cleanup(func() { collectEtcdFn = orig })

	cases := []struct {
		name  string
		size  int64
		quota int64
		want  result.Status
	}{
		{"healthy", 1024 * 1024 * 1024, 8 * 1024 * 1024 * 1024, result.StatusOK},
		{"warn", 6*1024*1024*1024 + 200*1024*1024, 8 * 1024 * 1024 * 1024, result.StatusWarning},
		{"crit", 7500 * 1024 * 1024, 8 * 1024 * 1024 * 1024, result.StatusCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collectEtcdFn = func(_ context.Context, _ etcd.Options, _ etcd.Reachers) (*etcd.Status, error) {
				return &etcd.Status{
					Mode: etcd.ModeDirect, Reachable: true, HasLeader: true,
					SizeBytes: tc.size, SizeInUseBytes: tc.size, QuotaBytes: tc.quota,
				}, nil
			}
			got := runCheck(t, &etcdSize{}, envWithObjects())
			if statusCounts(got)[tc.want] != 1 {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
		})
	}
}

func TestEtcdSizeUnavailableSkipped(t *testing.T) {
	orig := collectEtcdFn
	t.Cleanup(func() { collectEtcdFn = orig })
	collectEtcdFn = func(_ context.Context, _ etcd.Options, _ etcd.Reachers) (*etcd.Status, error) {
		return &etcd.Status{Mode: etcd.ModeViaAPIServer, Reachable: true, HasLeader: true}, nil
	}
	got := runCheck(t, &etcdSize{}, envWithObjects())
	if statusCounts(got)[result.StatusSkipped] != 1 {
		t.Fatalf("want SKIP, got %+v", got)
	}
}

func TestEtcdDefragBands(t *testing.T) {
	orig := collectEtcdFn
	t.Cleanup(func() { collectEtcdFn = orig })

	cases := []struct {
		name string
		size int64
		used int64
		want result.Status
	}{
		{"clean", 1000, 900, result.StatusOK},
		{"warn", 1000, 700, result.StatusWarning},
		{"crit", 1000, 400, result.StatusCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collectEtcdFn = func(_ context.Context, _ etcd.Options, _ etcd.Reachers) (*etcd.Status, error) {
				return &etcd.Status{
					Mode: etcd.ModeDirect, Reachable: true, HasLeader: true,
					SizeBytes: tc.size, SizeInUseBytes: tc.used,
				}, nil
			}
			got := runCheck(t, &etcdDefrag{}, envWithObjects())
			if statusCounts(got)[tc.want] != 1 {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
		})
	}
}

func TestEtcdMembersBands(t *testing.T) {
	orig := collectEtcdFn
	t.Cleanup(func() { collectEtcdFn = orig })

	cases := []struct {
		name    string
		members []etcd.MemberStatus
		want    result.Status
	}{
		{
			"three reachable",
			[]etcd.MemberStatus{
				{Endpoint: "a", Reachable: true},
				{Endpoint: "b", Reachable: true},
				{Endpoint: "c", Reachable: true},
			},
			result.StatusOK,
		},
		{
			"one unreachable",
			[]etcd.MemberStatus{
				{Endpoint: "a", Reachable: true},
				{Endpoint: "b", Reachable: false},
				{Endpoint: "c", Reachable: true},
			},
			result.StatusWarning,
		},
		{
			"all down",
			[]etcd.MemberStatus{
				{Endpoint: "a", Reachable: false},
				{Endpoint: "b", Reachable: false},
			},
			result.StatusCritical,
		},
		{
			"too few",
			[]etcd.MemberStatus{
				{Endpoint: "a", Reachable: true},
				{Endpoint: "b", Reachable: true},
			},
			result.StatusWarning,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collectEtcdFn = func(_ context.Context, _ etcd.Options, _ etcd.Reachers) (*etcd.Status, error) {
				return &etcd.Status{
					Mode: etcd.ModePodExec, Reachable: true, HasLeader: true,
					Members: tc.members,
				}, nil
			}
			got := runCheck(t, &etcdMembers{}, envWithObjects())
			if statusCounts(got)[tc.want] != 1 {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
		})
	}
}
