package watch

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/neilfarmer/k8s-health/internal/result"
)

// RunFunc executes one round of checks and returns the resulting Report.
// The TUI is parameterized over this so tests can inject a fake.
type RunFunc func(ctx context.Context) (result.Report, error)

// Options configures a watch session.
type Options struct {
	Interval time.Duration
	Run      RunFunc
}

type tickMsg time.Time
type reportMsg struct {
	rep result.Report
	dur time.Duration
}
type errMsg struct{ err error }

// Model is the bubbletea model backing `khealth watch`. It is exported
// so tests can drive Update directly without spinning a real Program.
type Model struct {
	opts     Options
	tracker  *Tracker
	rep      result.Report
	newKeys  map[FindingKey]struct{}
	err      error
	paused   bool
	runs     int
	lastRun  time.Time
	lastDur  time.Duration
	width    int
	height   int
	quitting bool
}

// NewModel returns an initialized Model.
func NewModel(o Options) Model {
	return Model{
		opts:    o,
		tracker: NewTracker(),
	}
}

// Init runs the initial check and starts the tick timer.
func (m Model) Init() tea.Cmd {
	return tea.Batch(runOnce(m.opts.Run), tickEvery(m.opts.Interval))
}

func runOnce(fn RunFunc) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		start := time.Now()
		rep, err := fn(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return reportMsg{rep: rep, dur: time.Since(start)}
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Update handles incoming messages and returns the next model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "p":
			m.paused = !m.paused
			return m, nil
		case "r":
			return m, runOnce(m.opts.Run)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		if m.paused {
			return m, tickEvery(m.opts.Interval)
		}
		return m, tea.Batch(runOnce(m.opts.Run), tickEvery(m.opts.Interval))
	case reportMsg:
		m.rep = msg.rep
		m.lastDur = msg.dur
		m.lastRun = time.Now()
		m.runs++
		m.newKeys = m.tracker.Diff(msg.rep.Findings)
		m.err = nil
		return m, nil
	case errMsg:
		m.err = msg.err
		m.lastRun = time.Now()
		return m, nil
	}
	return m, nil
}

// View renders the current model state to a string.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(headerLine(m))
	b.WriteString("\n\n")
	if m.err != nil {
		b.WriteString(errStyle.Render("error: " + m.err.Error()))
		b.WriteString("\n")
		b.WriteString(footerLine(m))
		return b.String()
	}
	if m.runs == 0 {
		b.WriteString(dimStyle.Render("loading…"))
		b.WriteString("\n")
		b.WriteString(footerLine(m))
		return b.String()
	}
	b.WriteString(findingsView(m))
	b.WriteString("\n")
	b.WriteString(summaryLine(m.rep))
	b.WriteString("\n")
	b.WriteString(footerLine(m))
	return b.String()
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Faint(true)
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	critStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	unkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	skipStyle    = lipgloss.NewStyle().Faint(true)
	checkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	nsStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	newRowStyle  = lipgloss.NewStyle().Bold(true)
	persistStyle = lipgloss.NewStyle().Faint(true)
)

func statusStyle(s result.Status) lipgloss.Style {
	switch s {
	case result.StatusCritical:
		return critStyle
	case result.StatusWarning:
		return warnStyle
	case result.StatusUnknown:
		return unkStyle
	case result.StatusOK:
		return okStyle
	case result.StatusSkipped:
		return skipStyle
	}
	return lipgloss.NewStyle()
}

func statusIcon(s result.Status) string {
	switch s {
	case result.StatusCritical:
		return "✖"
	case result.StatusWarning:
		return "⚠"
	case result.StatusUnknown:
		return "?"
	case result.StatusOK:
		return "✓"
	case result.StatusSkipped:
		return "⏭"
	}
	return "·"
}

func headerLine(m Model) string {
	parts := []string{titleStyle.Render("khealth watch")}
	if m.rep.Cluster != "" {
		parts = append(parts, titleStyle.Render(m.rep.Cluster))
	}
	if m.rep.Distro != "" {
		parts = append(parts, dimStyle.Render("distro="+m.rep.Distro))
	}
	parts = append(parts, dimStyle.Render(fmt.Sprintf("tick #%d", m.runs)))
	if !m.lastRun.IsZero() {
		parts = append(parts, dimStyle.Render(m.lastRun.Format("15:04:05")))
		parts = append(parts, dimStyle.Render(fmt.Sprintf("(%s)", m.lastDur.Round(time.Millisecond))))
	}
	state := okStyle.Render("⏵ live")
	if m.paused {
		state = warnStyle.Render("⏸ paused")
	}
	parts = append(parts, state)
	return strings.Join(parts, dimStyle.Render(" · "))
}

func findingsView(m Model) string {
	if len(m.rep.Findings) == 0 {
		return okStyle.Render("✓ no findings")
	}
	bucket := map[result.Status][]result.Finding{}
	for _, f := range m.rep.Findings {
		bucket[f.Status] = append(bucket[f.Status], f)
	}
	order := []result.Status{
		result.StatusCritical,
		result.StatusUnknown,
		result.StatusWarning,
	}
	var b strings.Builder
	checkW, resW := compactCols(m.rep.Findings, order)
	for _, s := range order {
		fs := bucket[s]
		if len(fs) == 0 {
			continue
		}
		st := statusStyle(s)
		b.WriteString(st.Render(statusIcon(s)) + "  " + titleStyle.Render(fmt.Sprintf("%s (%d)", s, len(fs))))
		b.WriteString("\n")
		sorted := append([]result.Finding(nil), fs...)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Check != sorted[j].Check {
				return sorted[i].Check < sorted[j].Check
			}
			return sorted[i].Resource < sorted[j].Resource
		})
		for _, f := range sorted {
			b.WriteString(renderRow(f, checkW, resW, m.newKeys))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if oks, skips := bucket[result.StatusOK], bucket[result.StatusSkipped]; len(oks)+len(skips) > 0 {
		bits := []string{}
		if n := len(oks); n > 0 {
			bits = append(bits, okStyle.Render(fmt.Sprintf("✓ %d OK", n)))
		}
		if n := len(skips); n > 0 {
			bits = append(bits, skipStyle.Render(fmt.Sprintf("⏭ %d SKIP", n)))
		}
		b.WriteString(strings.Join(bits, "   "))
		b.WriteString("\n")
	}
	return b.String()
}

func renderRow(f result.Finding, checkW, resW int, newKeys map[FindingKey]struct{}) string {
	st := statusStyle(f.Status)
	check := padRune(f.Check, checkW)
	res := f.Resource
	if res == "" {
		res = "-"
	}
	res = padRune(res, resW)

	bar := st.Render("▌")
	checkCell := checkStyle.Render(check)
	resCell := colorizeRes(res)

	row := fmt.Sprintf("  %s  %s  %s  %s", bar, checkCell, resCell, f.Message)
	if _, ok := newKeys[Key(f)]; ok {
		return newRowStyle.Render(row)
	}
	return persistStyle.Render(row)
}

func colorizeRes(s string) string {
	const sep = " in ns/"
	if i := strings.Index(s, sep); i >= 0 {
		return dimStyle.Render(s[:i]) + dimStyle.Render(sep) + nsStyle.Render(strings.TrimRight(s[i+len(sep):], " ")) + paddingTail(s)
	}
	return dimStyle.Render(s)
}

// paddingTail returns the trailing-space portion of s so coloring the
// trimmed namespace doesn't shrink the visible width.
func paddingTail(s string) string {
	end := len(s)
	for end > 0 && s[end-1] == ' ' {
		end--
	}
	return s[end:]
}

func compactCols(fs []result.Finding, order []result.Status) (checkW, resW int) {
	const checkCap = 32
	const resCap = 56
	want := map[result.Status]bool{}
	for _, s := range order {
		want[s] = true
	}
	for _, f := range fs {
		if !want[f.Status] {
			continue
		}
		if n := runeLen(f.Check); n > checkW {
			checkW = n
		}
		res := f.Resource
		if res == "" {
			res = "-"
		}
		if n := runeLen(res); n > resW {
			resW = n
		}
	}
	if checkW > checkCap {
		checkW = checkCap
	}
	if resW > resCap {
		resW = resCap
	}
	return checkW, resW
}

func summaryLine(r result.Report) string {
	bucket := map[result.Status]int{}
	for _, f := range r.Findings {
		bucket[f.Status]++
	}
	order := []struct {
		s     result.Status
		style lipgloss.Style
	}{
		{result.StatusCritical, critStyle},
		{result.StatusWarning, warnStyle},
		{result.StatusUnknown, unkStyle},
		{result.StatusSkipped, skipStyle},
		{result.StatusOK, okStyle},
	}
	parts := make([]string, 0, len(order))
	for _, o := range order {
		parts = append(parts, o.style.Render(fmt.Sprintf("%d %s", bucket[o.s], o.s)))
	}
	worst := r.Worst()
	return dimStyle.Render("Summary: ") + strings.Join(parts, dimStyle.Render("  ·  ")) +
		dimStyle.Render("   worst=") + statusStyle(worst).Render(string(worst))
}

func footerLine(m Model) string {
	keys := []string{
		dimStyle.Render("[q]") + " quit",
		dimStyle.Render("[p]") + " pause",
		dimStyle.Render("[r]") + " refresh",
	}
	return dimStyle.Render(strings.Join(keys, "   "))
}

func padRune(s string, w int) string {
	pad := w - runeLen(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
