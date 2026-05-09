package checks

import (
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
	for _, c := range []Check{&etcdHealth{}, &etcdSize{}, &etcdDefrag{}} {
		got := runCheck(t, c, envWithObjects())
		// Either SKIP (mode unavailable) or UNKNOWN (parse error). Both
		// non-blocking; we just want to make sure they don't panic and
		// don't claim CRIT against the fake.
		if statusCounts(got)[result.StatusCritical] > 0 {
			t.Errorf("%s: unexpected CRIT against fake client: %+v", c.ID(), got)
		}
	}
}
