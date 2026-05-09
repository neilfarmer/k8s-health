package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/neilfarmer/k8s-health/internal/checks"
	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestParseTimeout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", 2 * time.Minute, false},
		{"30s", 30 * time.Second, false},
		{"5m", 5 * time.Minute, false},
		{"junk", 0, true},
	}
	for _, tc := range cases {
		got, err := parseTimeout(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseTimeout(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseTimeout(%q) = %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("parseTimeout(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestCapabilitiesFor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		env  *kube.Env
		want checks.Capabilities
	}{
		{"nil env", nil, checks.CapAPIServer},
		{"out-of-cluster", &kube.Env{Mode: kube.ModeOutOfCluster}, checks.CapAPIServer},
		{"in-cluster adds cap", &kube.Env{Mode: kube.ModeInCluster}, checks.CapAPIServer | checks.CapInCluster},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := capabilitiesFor(tc.env); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestApplyOnlyUnhealthy(t *testing.T) {
	t.Parallel()
	rep := result.Report{Findings: []result.Finding{
		{Status: result.StatusOK},
		{Status: result.StatusWarning},
		{Status: result.StatusCritical},
		{Status: result.StatusSkipped},
	}}
	if got := applyOnlyUnhealthy(rep, false); len(got.Findings) != 4 {
		t.Errorf("flag false should be no-op; got %d", len(got.Findings))
	}
	got := applyOnlyUnhealthy(rep, true)
	if len(got.Findings) != 2 {
		t.Errorf("expected 2 findings (WARN+CRIT), got %d", len(got.Findings))
	}
}

func TestCheckExitContract(t *testing.T) {
	t.Parallel()
	clean := result.Report{Findings: []result.Finding{{Status: result.StatusOK}}}
	if err := checkExit(clean, false); err != nil {
		t.Errorf("clean report should not produce error, got %v", err)
	}

	crit := result.Report{Findings: []result.Finding{{Status: result.StatusCritical}}}
	err := checkExit(crit, false)
	var ec ExitCoder
	if !errors.As(err, &ec) {
		t.Fatalf("expected ExitCoder, got %T", err)
	}
	if ec.Code() != 1 {
		t.Errorf("expected code 1, got %d", ec.Code())
	}
	if ec.Error() == "" {
		t.Error("ExitCoder error message should not be empty")
	}
}

func TestWriteReportJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rep := result.Report{Findings: []result.Finding{{Status: result.StatusOK, Check: "x"}}}
	if err := writeReport(&buf, rep, &GlobalFlags{Output: "json"}); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	if !strings.Contains(buf.String(), `"x"`) {
		t.Errorf("want check id in body: %q", buf.String())
	}
}

func TestWriteReportUnknownFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := writeReport(&buf, result.Report{}, &GlobalFlags{Output: "xml"}); err == nil {
		t.Fatal("expected error for unknown output format")
	}
}

func TestEtcdProbeRunWithBadEndpoint(t *testing.T) {
	t.Setenv("KHEALTH_ETCD_ENDPOINTS", "http://127.0.0.1:1") // closed port → fast fail
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"etcd-probe"})
	// We don't care about the exit value — the goal is to exercise the
	// hidden subcommand's RunE for coverage. JSON output is still produced
	// (even if empty/error-marked).
	_ = cmd.Execute()
}

func TestApplyBaseline(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/base.json"
	base := result.Report{Findings: []result.Finding{
		{Check: "a", Status: result.StatusWarning, Resource: "r1"},
	}}
	if err := result.SaveBaseline(path, base); err != nil {
		t.Fatal(err)
	}
	cur := result.Report{Findings: []result.Finding{
		{Check: "a", Status: result.StatusWarning, Resource: "r1"},
		{Check: "b", Status: result.StatusCritical, Resource: "r2"},
	}}
	out, err := applyBaseline(cur, path)
	if err != nil {
		t.Fatal(err)
	}
	tagged := 0
	for _, f := range out.Findings {
		if len(f.Resource) > 13 && f.Resource[:13] == "[seen-before]" {
			tagged++
		}
	}
	if tagged != 1 {
		t.Errorf("want 1 seen-before, got %d", tagged)
	}
}

func TestApplyBaselineEmptyPath(t *testing.T) {
	rep := result.Report{Findings: []result.Finding{{Check: "a", Status: result.StatusOK}}}
	out, err := applyBaseline(rep, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Findings) != 1 {
		t.Errorf("want 1, got %d", len(out.Findings))
	}
}
