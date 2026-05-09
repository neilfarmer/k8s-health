package render

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/neilfarmer/k8s-health/internal/result"
)

type tableRenderer struct{}

// defaultTableWidth is used when stdout is not a TTY (pipe / file).
const defaultTableWidth = 120

func (tableRenderer) Render(w io.Writer, r result.Report) error {
	if err := writeTableHeader(w, r); err != nil {
		return err
	}

	if len(r.Findings) == 0 {
		_, err := fmt.Fprintln(w, "no findings")
		return err
	}

	maxWidth := terminalWidth(w)
	bucket := bucketByStatus(r.Findings)

	for _, s := range []result.Status{
		result.StatusCritical,
		result.StatusUnknown,
		result.StatusWarning,
		result.StatusOK,
		result.StatusSkipped,
	} {
		fs := bucket[s]
		if len(fs) == 0 {
			continue
		}
		if err := writeTableSection(w, s, fs, maxWidth); err != nil {
			return err
		}
	}

	worst := r.Worst()
	exit := r.ExitCode(false)
	_, err := fmt.Fprintf(w, "%d findings (worst=%s). Default exit code: %d\n", len(r.Findings), worst, exit)
	return err
}

func writeTableHeader(w io.Writer, r result.Report) error {
	if r.Cluster != "" {
		if _, err := fmt.Fprintf(w, "CLUSTER  %s\n", r.Cluster); err != nil {
			return err
		}
	}
	if r.Distro != "" {
		if _, err := fmt.Fprintf(w, "DISTRO   %s\n", r.Distro); err != nil {
			return err
		}
	}
	if !r.GeneratedAt.IsZero() {
		if _, err := fmt.Fprintf(w, "TIME     %s\n", r.GeneratedAt.Format("2006-01-02T15:04:05Z07:00")); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writeTableSection(w io.Writer, s result.Status, fs []result.Finding, maxWidth int) error {
	if _, err := fmt.Fprintf(w, "%s (%d)\n", s, len(fs)); err != nil {
		return err
	}

	sorted := append([]result.Finding(nil), fs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Check != sorted[j].Check {
			return sorted[i].Check < sorted[j].Check
		}
		return sorted[i].Resource < sorted[j].Resource
	})

	headers := []string{"CHECK", "RESOURCE", "MESSAGE"}
	rows := make([][]string, 0, len(sorted))
	for _, f := range sorted {
		res := f.Resource
		if res == "" {
			res = "-"
		}
		rows = append(rows, []string{f.Check, res, f.Message})
	}

	if err := drawTable(w, headers, rows, maxWidth); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w)
	return err
}

// terminalWidth returns the width of w if it is a TTY, otherwise the
// default. Honors COLUMNS env if set so users can override.
func terminalWidth(w io.Writer) int {
	if v := os.Getenv("COLUMNS"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	f, ok := w.(*os.File)
	if !ok {
		return defaultTableWidth
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil || width <= 0 {
		return defaultTableWidth
	}
	return width
}

// drawTable emits a Unicode-bordered table. Column widths are computed
// from content; if the natural total exceeds maxWidth the wider columns
// (MESSAGE first, then RESOURCE) are shrunk and their cells wrapped.
func drawTable(w io.Writer, headers []string, rows [][]string, maxWidth int) error {
	widths := computeWidths(headers, rows, maxWidth)

	top := borderLine("┌", "┬", "┐", widths)
	mid := borderLine("├", "┼", "┤", widths)
	bot := borderLine("└", "┴", "┘", widths)

	if _, err := fmt.Fprintln(w, top); err != nil {
		return err
	}
	if err := writeWrappedRow(w, headers, widths); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, mid); err != nil {
		return err
	}
	for _, r := range rows {
		if err := writeWrappedRow(w, r, widths); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, bot)
	return err
}

// computeWidths starts from the natural max width per column (header +
// every cell) and, if the total exceeds maxWidth, shrinks the rightmost
// columns until it fits. Each column has a small floor so labels stay
// legible.
func computeWidths(headers []string, rows [][]string, maxWidth int) []int {
	cols := len(headers)
	widths := make([]int, cols)
	for i, h := range headers {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, r := range rows {
		for i := 0; i < cols && i < len(r); i++ {
			if n := utf8.RuneCountInString(r[i]); n > widths[i] {
				widths[i] = n
			}
		}
	}

	overhead := 1 + 3*cols // leftmost "│" + (" cell │") per col
	avail := maxWidth - overhead
	if avail <= 0 {
		return widths
	}
	if sum(widths) <= avail {
		return widths
	}

	// Shrink from the right. MESSAGE (last) typically dominates.
	floors := make([]int, cols)
	for i := range floors {
		floors[i] = minColWidth(headers[i])
	}
	for col := cols - 1; col >= 0; col-- {
		if sum(widths) <= avail {
			break
		}
		room := widths[col] - floors[col]
		if room <= 0 {
			continue
		}
		shrinkBy := sum(widths) - avail
		if shrinkBy > room {
			shrinkBy = room
		}
		widths[col] -= shrinkBy
	}
	return widths
}

func minColWidth(header string) int {
	n := utf8.RuneCountInString(header)
	if n < 8 {
		n = 8
	}
	return n
}

func sum(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x
	}
	return s
}

func writeWrappedRow(w io.Writer, cells []string, widths []int) error {
	wrapped := make([][]string, len(widths))
	height := 1
	for i := range widths {
		val := ""
		if i < len(cells) {
			val = cells[i]
		}
		wrapped[i] = wrapCell(val, widths[i])
		if len(wrapped[i]) > height {
			height = len(wrapped[i])
		}
	}
	for line := 0; line < height; line++ {
		var b strings.Builder
		b.WriteString("│")
		for i, cw := range widths {
			seg := ""
			if line < len(wrapped[i]) {
				seg = wrapped[i][line]
			}
			pad := cw - utf8.RuneCountInString(seg)
			if pad < 0 {
				pad = 0
			}
			b.WriteString(" ")
			b.WriteString(seg)
			b.WriteString(strings.Repeat(" ", pad))
			b.WriteString(" │")
		}
		if _, err := fmt.Fprintln(w, b.String()); err != nil {
			return err
		}
	}
	return nil
}

// wrapCell breaks s into lines no wider than width runes. It prefers
// breaking at spaces but hard-breaks tokens longer than width.
func wrapCell(s string, width int) []string {
	if width <= 0 || s == "" {
		return []string{s}
	}
	if utf8.RuneCountInString(s) <= width {
		return []string{s}
	}

	out := []string{}
	var line strings.Builder
	lineLen := 0
	flush := func() {
		out = append(out, line.String())
		line.Reset()
		lineLen = 0
	}
	for _, word := range strings.Fields(s) {
		wlen := utf8.RuneCountInString(word)
		if wlen > width {
			if lineLen > 0 {
				flush()
			}
			runes := []rune(word)
			for i := 0; i < len(runes); i += width {
				end := i + width
				if end > len(runes) {
					end = len(runes)
				}
				out = append(out, string(runes[i:end]))
			}
			continue
		}
		need := wlen
		if lineLen > 0 {
			need++ // space
		}
		if lineLen+need > width {
			flush()
			line.WriteString(word)
			lineLen = wlen
			continue
		}
		if lineLen > 0 {
			line.WriteString(" ")
			lineLen++
		}
		line.WriteString(word)
		lineLen += wlen
	}
	if lineLen > 0 {
		flush()
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func borderLine(left, sep, right string, widths []int) string {
	var b strings.Builder
	b.WriteString(left)
	for i, w := range widths {
		b.WriteString(strings.Repeat("─", w+2))
		if i < len(widths)-1 {
			b.WriteString(sep)
		}
	}
	b.WriteString(right)
	return b.String()
}
