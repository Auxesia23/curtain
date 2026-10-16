package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

// runOptions contains flags for `curtain run`.
type runOptions struct {
	project string
	env     string
}

func newRunCmd(runner runFunc) *cobra.Command {
	var opts runOptions
	cmd := &cobra.Command{
		Use:   "run [flags] -- COMMAND [ARG...]",
		Short: "Run a command with Curtain variables injected into its environment",
		Long: `Fetches the latest variables from the server, decrypts them locally, merges
them into the process environment (Curtain variables win over existing ones),
and runs COMMAND with exit code and signals passed through as-is.

The .curtain.json file is looked up from the current directory upwards, so
curtain run works from subdirectories too.`,
		Example: `  curtain run -- npm run dev
  curtain run --env staging -- npm run dev`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("%w: no command to run (usage: curtain run [flags] -- COMMAND [ARG...])", errUsage)
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			return runner(opts, args)
		},
	}
	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "override the project from .curtain.json (name or id)")
	cmd.Flags().StringVarP(&opts.env, "env", "e", "", "override the environment from .curtain.json (name)")
	// Stop parsing flags at the first command argument so flags belonging to
	// COMMAND (for example, `npm run dev --save-dev`) are passed through unchanged.
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// runWithEnv fetches variables, decrypts them locally, merges them into the
// process environment, and execs the command so exit codes and signals pass
// through (using syscall.Exec on Unix).
func runWithEnv(opts runOptions, argv []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	_, projCfg, err := findProjectConfig(cwd)
	if err != nil {
		return err
	}

	projectRef := opts.project
	envName := opts.env
	if projCfg != nil {
		if projectRef == "" {
			projectRef = strconv.FormatUint(uint64(projCfg.ProjectID), 10)
		}
		if envName == "" {
			envName = projCfg.Environment
		}
	}
	if projectRef == "" || envName == "" {
		return errors.New("not linked to a project: run `curtain init` in this directory, or pass --project and --env")
	}

	_, creds, client, err := loadSession()
	if err != nil {
		return err
	}

	fetched, err := fetchEnv(client, creds, projectRef, envName)
	if err != nil {
		return err
	}
	for _, warning := range fetched.Warnings {
		fmt.Fprintf(os.Stderr, "curtain: %s\n", warning)
	}

	env := buildEnv(os.Environ(), fetched.Vars)
	fmt.Fprintf(
		os.Stderr,
		"curtain: injecting %d variables from %s/%s\n",
		len(fetched.Vars), fetched.ProjectName, fetched.EnvName,
	)

	binPath, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("command %q not found in PATH", argv[0])
	}
	return execProcess(binPath, argv, env)
}
