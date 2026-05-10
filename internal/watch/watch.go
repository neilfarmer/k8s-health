package watch

import (
	"context"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the watch TUI and blocks until the user quits or ctx is
// cancelled. It writes terminal output to out (typically os.Stdout).
func Run(ctx context.Context, out io.Writer, in io.Reader, opts Options) error {
	if opts.Run == nil {
		return fmt.Errorf("watch: Options.Run is required")
	}
	if opts.Interval <= 0 {
		return fmt.Errorf("watch: Options.Interval must be positive")
	}
	prog := tea.NewProgram(
		NewModel(opts),
		tea.WithOutput(out),
		tea.WithInput(in),
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)
	_, err := prog.Run()
	return err
}
