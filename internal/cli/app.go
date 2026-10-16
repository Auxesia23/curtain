package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// version is shown by `curtain --version`.
const version = "0.1.0"

// errUsage marks command-line usage errors (exit code 2).
var errUsage = errors.New("invalid usage")

// Run is the CLI entry point. It returns the process exit code.
func Run(args []string) int {
	return execute(args, runWithEnv)
}

// runFunc lets tests inspect `run` argument parsing without starting a child process.
type runFunc func(opts runOptions, argv []string) error

// execute builds and runs the command tree, then maps errors to exit codes:
// 2 for usage errors and 1 for runtime failures.
func execute(args []string, runner runFunc) int {
	root := newRootCmd(runner)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "curtain: %v\n", err)
		if isUsageError(err) {
			return 2
		}
		return 1
	}
	return 0
}

// isUsageError distinguishes command-line errors (exit 2) from runtime errors
// (exit 1). FlagErrorFunc wraps errors with errUsage; Cobra's unknown-command
// errors are recognized by their message prefix.
func isUsageError(err error) bool {
	if errors.Is(err, errUsage) {
		return true
	}
	message := err.Error()
	return strings.HasPrefix(message, "unknown command") || strings.HasPrefix(message, "unknown flag")
}

// newRootCmd builds the Cobra command tree (root plus four CLI commands).
func newRootCmd(runner runFunc) *cobra.Command {
	root := &cobra.Command{
		Use:   "curtain",
		Short: "Inject environment variables into a child process",
		Long: `Curtain CLI client.

It fetches environment variables from a Curtain server, decrypts them locally
(zero-knowledge: the server never sees plaintext), and runs your command with
the variables injected into its process environment.

Project, environment, and variable management happens in the web UI.`,
		Version:       version,
		SilenceUsage:  true, // Do not print usage for runtime errors.
		SilenceErrors: true, // execute() prints each error once.
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Without a subcommand, show help and return a usage error.
			_ = cmd.Help()
			return fmt.Errorf("%w: no command given", errUsage)
		},
	}
	root.SetVersionTemplate("curtain {{.Version}}\n")
	// Keep the CLI surface small by disabling Cobra's default completion command.
	root.CompletionOptions.DisableDefaultCmd = true
	// Treat all flag errors, such as unknown flags, as usage errors.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %v", errUsage, err)
	})

	root.AddCommand(
		newLoginCmd(),
		newLogoutCmd(),
		newInitCmd(),
		newRunCmd(runner),
	)
	return root
}
