package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/neilfarmer/k8s-health/internal/result"
)

type compactRenderer struct{}

const (
	compactCheckCap    = 32
	compactResourceCap = 56
	seenBeforePrefix   = "[seen-before] "
)

func (compactRenderer) Render(w io.Writer, r result.Report) error {
	p := newPalette(colorEnabled(w))

	if err := writePrettyHeader(w, r, p); err != nil {
		return err
	}

	if len(r.Findings) == 0 {
		_, err := fmt.Fprintln(w, "no findings")
		return err
	}

	bucket := bucketByStatus(r.Findings)

	for _, s := range []result.Status{
		result.StatusCritical,
		result.StatusUnknown,
		result.StatusWarning,
	} {
		if len(bucket[s]) == 0 {
			continue
		}
		if err := writeCompactSection(w, s, bucket[s], p); err != nil {
			return err
		}
	}

	if err := writeCollapsed(w, bucket[result.StatusOK], bucket[result.StatusSkipped], p); err != nil {
		return err
	}

	return writeSummary(w, r, bucket, p)
}

func writeCompactSection(w io.Writer, s result.Status, fs []result.Finding, p palette) error {
	icon, color := iconColor(s, p)
	header := fmt.Sprintf("%s%s%s  %s%s (%d)%s",
		color, icon, p.reset,
		p.bold, s, len(fs), p.reset)
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}

	sorted := append([]result.Finding(nil), fs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Check != sorted[j].Check {
			return sorted[i].Check < sorted[j].Check
		}
		return sorted[i].Resource < sorted[j].Resource
	})

	checkW, resW := compactWidths(sorted)

	for _, f := range sorted {
		isNew, res := stripSeenBefore(f.Resource)
		check := truncate(f.Check, checkW)
		resPlain := res
		if resPlain == "" {
			resPlain = "-"
		}
		resPlain = truncate(resPlain, resW)
		resPad := strings.Repeat(" ", padBy(resPlain, resW))

		emphStart, emphEnd := "", ""
		switch {
		case !isNew && p.dim != "":
			emphStart, emphEnd = p.dim, p.reset
		case isNew && p.bold != "":
			emphStart, emphEnd = p.bold, p.reset
		}

		line := fmt.Sprintf("  %s▌%s  %s%s%s  %s%s%s  %s%s%s",
			color, p.reset,
			p.cyan, padRight(check, checkW), p.reset,
			emphStart, colorizeResource(resPlain, p)+resPad, emphEnd,
			emphStart, f.Message, emphEnd)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

// colorizeResource splits a resource string of the form
// "<kind>/<name> in ns/<namespace>" into two color groups so the namespace
// stands out from the resource kind. Falls back to a single dim segment when
// the pattern doesn't match.
func colorizeResource(s string, p palette) string {
	if s == "" || s == "-" {
		return p.dim + s + p.reset
	}
	const sep = " in ns/"
	if i := strings.Index(s, sep); i >= 0 {
		head := s[:i]
		ns := s[i+len(sep):]
		return p.dim + head + p.reset + p.dim + " in ns/" + p.reset + p.magenta + ns + p.reset
	}
	return p.dim + s + p.reset
}

func stripSeenBefore(res string) (isNew bool, out string) {
	if strings.HasPrefix(res, seenBeforePrefix) {
		return false, strings.TrimPrefix(res, seenBeforePrefix)
	}
	return true, res
}

func compactWidths(fs []result.Finding) (checkW, resW int) {
	for _, f := range fs {
		if n := utf8.RuneCountInString(f.Check); n > checkW {
			checkW = n
		}
		_, res := stripSeenBefore(f.Resource)
		if res == "" {
			res = "-"
		}
		if n := utf8.RuneCountInString(res); n > resW {
			resW = n
		}
	}
	if checkW > compactCheckCap {
		checkW = compactCheckCap
	}
	if resW > compactResourceCap {
		resW = compactResourceCap
	}
	return checkW, resW
}

func padRight(s string, w int) string {
	pad := w - utf8.RuneCountInString(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func padBy(s string, w int) int {
	pad := w - utf8.RuneCountInString(s)
	if pad <= 0 {
		return 0
	}
	return pad
}

func truncate(s string, w int) string {
	if w <= 0 || utf8.RuneCountInString(s) <= w {
		return s
	}
	if w <= 1 {
		return string([]rune(s)[:w])
	}
	r := []rune(s)
	return string(r[:w-1]) + "…"
}
