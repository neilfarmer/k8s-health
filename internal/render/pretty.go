package render

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"

	"github.com/neilfarmer/k8s-health/internal/result"
)

type prettyRenderer struct{}

// ANSI SGR codes. Empty when color is disabled.
type palette struct {
	reset, bold, dim                  string
	red, yellow, green, cyan, magenta string
}

func newPalette(enabled bool) palette {
	if !enabled {
		return palette{}
	}
	return palette{
		reset:   "\x1b[0m",
		bold:    "\x1b[1m",
		dim:     "\x1b[2m",
		red:     "\x1b[31m",
		yellow:  "\x1b[33m",
		green:   "\x1b[32m",
		cyan:    "\x1b[36m",
		magenta: "\x1b[35m",
	}
}

// colorEnabled reports whether ANSI escapes should be emitted on w. It
// honors NO_COLOR (https://no-color.org/) unconditionally and otherwise
// only colorizes when w is an interactive terminal.
func colorEnabled(w io.Writer) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func (prettyRenderer) Render(w io.Writer, r result.Report) error {
	p := newPalette(colorEnabled(w))

	if err := writePrettyHeader(w, r, p); err != nil {
		return err
	}

	if len(r.Findings) == 0 {
		_, err := fmt.Fprintln(w, "no findings")
		return err
	}

	bucket := bucketByStatus(r.Findings)

	// Severity sections: print in worst-first order.
	for _, s := range []result.Status{
		result.StatusCritical,
		result.StatusUnknown,
		result.StatusWarning,
	} {
		if len(bucket[s]) == 0 {
			continue
		}
		if err := writeSection(w, s, bucket[s], p); err != nil {
			return err
		}
	}

	if err := writeCollapsed(w, bucket[result.StatusOK], bucket[result.StatusSkipped], p); err != nil {
		return err
	}

	return writeSummary(w, r, bucket, p)
}

func writePrettyHeader(w io.Writer, r result.Report, p palette) error {
	parts := []string{}
	if r.Cluster != "" {
		parts = append(parts, fmt.Sprintf("%s%s%s", p.bold, r.Cluster, p.reset))
	}
	if !r.GeneratedAt.IsZero() {
		parts = append(parts, r.GeneratedAt.Format("2006-01-02 15:04 MST"))
	}
	if r.Distro != "" {
		parts = append(parts, fmt.Sprintf("%sdistro=%s%s", p.dim, r.Distro, p.reset))
	}
	if len(parts) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, strings.Join(parts, "  ·  ")); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writeSection(w io.Writer, s result.Status, fs []result.Finding, p palette) error {
	icon, color := iconColor(s, p)
	header := fmt.Sprintf("%s%s%s  %s%s (%d)%s",
		color, icon, p.reset,
		p.bold, s, len(fs), p.reset)
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}

	groups := groupByCheck(fs)
	for _, g := range groups {
		if _, err := fmt.Fprintf(w, "  %s%s%s\n", p.cyan, g.check, p.reset); err != nil {
			return err
		}
		for _, f := range g.findings {
			if f.Resource != "" {
				if _, err := fmt.Fprintf(w, "    %s%s%s\n", p.dim, f.Resource, p.reset); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(w, "      %s\n", f.Message); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(w, "    %s\n", f.Message); err != nil {
					return err
				}
			}
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writeCollapsed(w io.Writer, oks, skips []result.Finding, p palette) error {
	if len(oks) == 0 && len(skips) == 0 {
		return nil
	}
	parts := []string{}
	if len(oks) > 0 {
		parts = append(parts, fmt.Sprintf("%s✓ %d OK%s", p.green, len(oks), p.reset))
	}
	if len(skips) > 0 {
		parts = append(parts, fmt.Sprintf("%s⏭  %d SKIP%s", p.dim, len(skips), p.reset))
	}
	if _, err := fmt.Fprintf(w, "%s   %s(use -o table to expand)%s\n\n",
		strings.Join(parts, "   "), p.dim, p.reset); err != nil {
		return err
	}
	return nil
}

func writeSummary(w io.Writer, r result.Report, bucket map[result.Status][]result.Finding, p palette) error {
	counts := []struct {
		s     result.Status
		color string
	}{
		{result.StatusCritical, p.red},
		{result.StatusWarning, p.yellow},
		{result.StatusUnknown, p.magenta},
		{result.StatusSkipped, p.dim},
		{result.StatusOK, p.green},
	}
	parts := make([]string, 0, len(counts))
	for _, c := range counts {
		parts = append(parts, fmt.Sprintf("%s%d %s%s", c.color, len(bucket[c.s]), c.s, p.reset))
	}
	worst := r.Worst()
	exit := r.ExitCode(false)
	_, color := iconColor(worst, p)
	_, err := fmt.Fprintf(
		w,
		"Summary: %s\n%sWorst: %s%s%s   Exit: %d\n",
		strings.Join(parts, "  ·  "),
		p.bold, color, worst, p.reset+p.bold, exit,
	)
	if err == nil && p.bold != "" {
		_, err = fmt.Fprint(w, p.reset)
	}
	return err
}

type checkGroup struct {
	check    string
	findings []result.Finding
}

func groupByCheck(fs []result.Finding) []checkGroup {
	idx := map[string]int{}
	out := []checkGroup{}
	for _, f := range fs {
		i, ok := idx[f.Check]
		if !ok {
			idx[f.Check] = len(out)
			out = append(out, checkGroup{check: f.Check})
			i = len(out) - 1
		}
		out[i].findings = append(out[i].findings, f)
	}
	for i := range out {
		sort.SliceStable(out[i].findings, func(a, b int) bool {
			return out[i].findings[a].Resource < out[i].findings[b].Resource
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].check < out[j].check })
	return out
}

func bucketByStatus(fs []result.Finding) map[result.Status][]result.Finding {
	m := map[result.Status][]result.Finding{}
	for _, f := range fs {
		m[f.Status] = append(m[f.Status], f)
	}
	return m
}

func iconColor(s result.Status, p palette) (icon, color string) {
	switch s {
	case result.StatusCritical:
		return "✖", p.red
	case result.StatusWarning:
		return "⚠", p.yellow
	case result.StatusUnknown:
		return "?", p.magenta
	case result.StatusSkipped:
		return "⏭", p.dim
	case result.StatusOK:
		return "✓", p.green
	}
	return "·", ""
}
