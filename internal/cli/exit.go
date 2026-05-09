package cli

// exitError is returned by subcommands that want main() to exit with a
// specific code (per the contract in docs/adr/0006-output-formats.md).
//
// cmd/khealth/main.go inspects errors with errors.As and uses Code() to
// drive os.Exit. Anything else is treated as a tool error (exit 3).
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }
func (e *exitError) Code() int     { return e.code }

// ExitCoder is implemented by errors that want to influence the process
// exit code. main.go uses errors.As to find one.
type ExitCoder interface {
	error
	Code() int
}
