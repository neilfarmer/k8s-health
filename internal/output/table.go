package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/neilfarmer/k8s-health/internal/checker"
	"golang.org/x/term"
)

// TableFormatter renders results as a colored terminal table.
type TableFormatter struct {
	NoColor bool
	Verbose bool
}

// NewTableFormatter creates a new table formatter.
func NewTableFormatter(noColor, verbose bool) *TableFormatter {
	if noColor || !isTerminal() {
		color.NoColor = true
	}
	return &TableFormatter{NoColor: noColor, Verbose: verbose}
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// Render outputs the scan results to the writer.
func (f *TableFormatter) Render(w io.Writer, results []*checker.Result, contextName string, namespaces []string, minSeverity checker.Severity) {
	f.renderHeader(w, contextName, namespaces)

	totalFindings := 0
	checkersWithIssues := 0

	var summary ReportSummary
	summary.TotalCheckers = len(results)

	for _, r := range results {
		filtered := f.filterFindings(r.Findings, minSeverity)

		for _, finding := range filtered {
			switch finding.Severity {
			case checker.SeverityCritical:
				summary.Critical++
			case checker.SeverityWarning:
				summary.Warning++
			case checker.SeverityInfo:
				summary.Info++
			}
		}

		if r.Error != nil {
			f.renderError(w, r)
			checkersWithIssues++
			continue
		}

		if len(filtered) == 0 && !f.Verbose {
			continue
		}

		if len(filtered) > 0 {
			checkersWithIssues++
			totalFindings += len(filtered)
		}

		f.renderCheckerResult(w, r.CheckerName, filtered)
	}

	summary.CheckersWithIssues = checkersWithIssues
	f.renderSummary(w, summary, totalFindings)
}

func (f *TableFormatter) renderHeader(w io.Writer, contextName string, namespaces []string) {
	bold := color.New(color.Bold)
	line := strings.Repeat("=", 80)

	fmt.Fprintln(w, line)
	bold.Fprintf(w, "  k8s-health Cluster Scan Report\n")

	ns := "all namespaces"
	if len(namespaces) > 0 {
		ns = strings.Join(namespaces, ", ")
	}
	fmt.Fprintf(w, "  Context: %s  |  Scanned: %s  |  %s\n",
		contextName, ns, time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintln(w, line)
	fmt.Fprintln(w)
}

func (f *TableFormatter) filterFindings(findings []checker.Finding, minSeverity checker.Severity) []checker.Finding {
	var filtered []checker.Finding
	for _, finding := range findings {
		if finding.Severity >= minSeverity {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

func (f *TableFormatter) renderCheckerResult(w io.Writer, name string, findings []checker.Finding) {
	headerColor := color.New(color.Bold, color.FgCyan)

	if len(findings) == 0 {
		headerColor.Fprintf(w, "--- %s (healthy) ", titleCase(name))
		fmt.Fprintln(w, strings.Repeat("-", maxInt(0, 80-len(name)-16)))
		fmt.Fprintln(w)
		return
	}

	headerColor.Fprintf(w, "--- %s (%d issue", titleCase(name), len(findings))
	if len(findings) != 1 {
		fmt.Fprint(w, "s")
	}
	fmt.Fprint(w, ") ")
	fmt.Fprintln(w, strings.Repeat("-", maxInt(0, 80-len(name)-len(fmt.Sprintf("%d", len(findings)))-14)))

	// Calculate column widths
	sevWidth := 10
	nsWidth := 5
	resWidth := 8
	for _, finding := range findings {
		ns := finding.Namespace
		if ns == "" {
			ns = "-"
		}
		if len(ns) > nsWidth {
			nsWidth = len(ns)
		}
		res := finding.Resource()
		if len(res) > resWidth {
			resWidth = len(res)
		}
	}
	// Cap column widths
	if nsWidth > 25 {
		nsWidth = 25
	}
	if resWidth > 40 {
		resWidth = 40
	}

	// Print header
	headerFmt := color.New(color.Bold)
	headerFmt.Fprintf(w, "  %-*s  %-*s  %-*s  %s\n", sevWidth, "SEVERITY", nsWidth, "NAMESPACE", resWidth, "RESOURCE", "MESSAGE")

	// Print findings
	for _, finding := range findings {
		sevStr := colorSeverity(finding.Severity)
		ns := finding.Namespace
		if ns == "" {
			ns = "-"
		}
		if len(ns) > nsWidth {
			ns = ns[:nsWidth-1] + "~"
		}
		res := finding.Resource()
		if len(res) > resWidth {
			res = res[:resWidth-1] + "~"
		}
		// Severity string has ANSI codes, so pad the raw text width
		sevPad := sevWidth + len(sevStr) - len(finding.Severity.String())
		fmt.Fprintf(w, "  %-*s  %-*s  %-*s  %s\n", sevPad, sevStr, nsWidth, ns, resWidth, res, finding.Message)
	}
	fmt.Fprintln(w)
}

func (f *TableFormatter) renderError(w io.Writer, r *checker.Result) {
	errColor := color.New(color.FgRed, color.Bold)
	errColor.Fprintf(w, "--- %s (error) ", titleCase(r.CheckerName))
	fmt.Fprintln(w, strings.Repeat("-", maxInt(0, 80-len(r.CheckerName)-13)))
	fmt.Fprintf(w, "  Error: %v\n\n", r.Error)
}

func (f *TableFormatter) renderSummary(w io.Writer, summary ReportSummary, totalFindings int) {
	line := strings.Repeat("=", 80)
	bold := color.New(color.Bold)
	critColor := color.New(color.FgRed, color.Bold)
	warnColor := color.New(color.FgYellow, color.Bold)
	infoColor := color.New(color.FgCyan)

	fmt.Fprintln(w, line)
	bold.Fprintln(w, "  SUMMARY")
	fmt.Fprintf(w, "  Total checks: %d  |  Checks with issues: %d  |  Healthy: %d\n",
		summary.TotalCheckers, summary.CheckersWithIssues,
		summary.TotalCheckers-summary.CheckersWithIssues)

	fmt.Fprint(w, "  ")
	critColor.Fprintf(w, "Critical: %d", summary.Critical)
	fmt.Fprint(w, "  |  ")
	warnColor.Fprintf(w, "Warning: %d", summary.Warning)
	fmt.Fprint(w, "  |  ")
	infoColor.Fprintf(w, "Info: %d", summary.Info)
	fmt.Fprintln(w)

	exitCode := 0
	if totalFindings > 0 {
		exitCode = 1
	}
	fmt.Fprintf(w, "  Exit code: %d", exitCode)
	if exitCode == 0 {
		color.New(color.FgGreen).Fprint(w, " (healthy)")
	} else {
		color.New(color.FgRed).Fprint(w, " (issues found)")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, line)
}

func colorSeverity(s checker.Severity) string {
	switch s {
	case checker.SeverityCritical:
		return color.New(color.FgRed, color.Bold).Sprint("CRITICAL")
	case checker.SeverityWarning:
		return color.New(color.FgYellow, color.Bold).Sprint("WARNING")
	case checker.SeverityInfo:
		return color.New(color.FgCyan).Sprint("INFO")
	default:
		return s.String()
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
