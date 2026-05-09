package result_test

import (
	"testing"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestReportWorst(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		statuses []result.Status
		want     result.Status
	}{
		{"empty report is OK", nil, result.StatusOK},
		{"single OK", []result.Status{result.StatusOK}, result.StatusOK},
		{"WARN beats OK", []result.Status{result.StatusOK, result.StatusWarning}, result.StatusWarning},
		{"CRIT beats WARN", []result.Status{result.StatusWarning, result.StatusCritical}, result.StatusCritical},
		{"SKIP does not raise severity", []result.Status{result.StatusOK, result.StatusSkipped}, result.StatusOK},
		{"UNKNOWN below WARN", []result.Status{result.StatusUnknown, result.StatusWarning}, result.StatusWarning},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := result.Report{}
			for _, s := range tc.statuses {
				r.Findings = append(r.Findings, result.Finding{Status: s})
			}
			if got := r.Worst(); got != tc.want {
				t.Fatalf("Worst() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReportExitCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		worst    result.Status
		strict   bool
		wantCode int
	}{
		{"all clean", result.StatusOK, false, 0},
		{"warn lenient", result.StatusWarning, false, 0},
		{"warn strict", result.StatusWarning, true, 2},
		{"crit lenient", result.StatusCritical, false, 1},
		{"crit strict", result.StatusCritical, true, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := result.Report{Findings: []result.Finding{{Status: tc.worst}}}
			if got := r.ExitCode(tc.strict); got != tc.wantCode {
				t.Fatalf("ExitCode(strict=%v) = %d, want %d", tc.strict, got, tc.wantCode)
			}
		})
	}
}
