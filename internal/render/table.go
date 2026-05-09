package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/neilfarmer/k8s-health/internal/result"
)

type tableRenderer struct{}

func (tableRenderer) Render(w io.Writer, r result.Report) error {
	if r.Cluster != "" {
		if _, err := fmt.Fprintf(w, "CLUSTER  %s\n", r.Cluster); err != nil {
			return err
		}
	}
	if !r.GeneratedAt.IsZero() {
		if _, err := fmt.Fprintf(w, "TIME     %s\n", r.GeneratedAt.Format("2006-01-02T15:04:05Z07:00")); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if len(r.Findings) == 0 {
		_, err := fmt.Fprintln(w, "no findings")
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	sorted := append([]result.Finding(nil), r.Findings...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if rank(sorted[i].Status) != rank(sorted[j].Status) {
			return rank(sorted[i].Status) > rank(sorted[j].Status)
		}
		if sorted[i].Check != sorted[j].Check {
			return sorted[i].Check < sorted[j].Check
		}
		return sorted[i].Resource < sorted[j].Resource
	})
	for _, f := range sorted {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			string(f.Status),
			f.Check,
			truncate(f.Resource, 60),
			truncate(f.Message, 80),
		); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	worst := r.Worst()
	exit := r.ExitCode(false)
	_, err := fmt.Fprintf(w, "\n%d findings (worst=%s). Default exit code: %d\n", len(r.Findings), worst, exit)
	return err
}

func rank(s result.Status) int {
	switch s {
	case result.StatusCritical:
		return 4
	case result.StatusWarning:
		return 3
	case result.StatusUnknown:
		return 2
	case result.StatusSkipped:
		return 1
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimRight(s[:n-1], " ") + "…"
}
