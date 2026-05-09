package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTestCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run declarative HealthTest manifests",
	}

	cmd.AddCommand(
		newTestSubCmd(g, "run", "Run one or more HealthTest YAMLs"),
		newTestSubCmd(g, "lint", "Validate HealthTest schema without executing"),
		newTestSubCmd(g, "list", "Show test names, steps, and estimated duration"),
	)

	return cmd
}

func newTestSubCmd(g *GlobalFlags, verb, short string) *cobra.Command {
	return &cobra.Command{
		Use:   verb + " PATH...",
		Short: short,
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(),
				"khealth test %s: not yet implemented (output=%s, paths=%v)\n",
				verb, g.Output, args)
			return err
		},
	}
}
