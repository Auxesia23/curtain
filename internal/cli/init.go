package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// initOptions contains flags for `curtain init`.
type initOptions struct {
	project string
	env     string
	force   bool
}

func newInitCmd() *cobra.Command {
	var opts initOptions
	cmd := &cobra.Command{
		Use:   "init [flags]",
		Short: "Select a project + environment and write .curtain.json",
		Long: `Writes .curtain.json in the current directory, linking it to one project
and environment on the server. The file contains no secrets and is safe to
commit.`,
		Example: `  curtain init
  curtain init --project my-app --env development`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runInit(opts)
		},
	}
	cmd.Flags().StringVar(&opts.project, "project", "", "project (name or numeric id)")
	cmd.Flags().StringVar(&opts.env, "env", "", "environment (name)")
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite .curtain.json without confirmation")
	return cmd
}

// runInit links the current directory to a project and environment by writing
// .curtain.json. The file contains no secrets and is safe to commit.
func runInit(opts initOptions) error {
	_, _, client, err := loadSession()
	if err != nil {
		return err
	}

	projects, err := client.listProjects()
	if err != nil {
		return sessionError("fetching projects", err)
	}
	if len(projects) == 0 {
		return errors.New("no accessible projects — create one via the web UI first")
	}

	var project projectInfo
	if opts.project != "" {
		project, err = resolveProject(projects, opts.project)
		if err != nil {
			return err
		}
	} else {
		project, err = pickProject(projects)
		if err != nil {
			return err
		}
	}

	envs, err := client.listEnvironments(project.ID)
	if err != nil {
		return sessionError("fetching environments", err)
	}
	if len(envs) == 0 {
		return fmt.Errorf("project %q has no environments yet — create one via the web UI first", project.Name)
	}

	var env environmentInfo
	if opts.env != "" {
		env, err = resolveEnvironment(envs, opts.env)
		if err != nil {
			return err
		}
	} else {
		env, err = pickEnvironment(envs)
		if err != nil {
			return err
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path := filepath.Join(cwd, ProjectConfigName)
	if _, err := os.Stat(path); err == nil && !opts.force {
		overwrite, err := confirm(fmt.Sprintf("%s already exists — overwrite?", ProjectConfigName))
		if err != nil {
			return err
		}
		if !overwrite {
			return errors.New("cancelled")
		}
	}

	cfg := &projectConfig{
		ProjectID:   project.ID,
		ProjectName: project.Name,
		Environment: env.Name,
	}
	if err := saveProjectConfig(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(
		os.Stderr,
		"curtain: %s written — project %q (id %d), environment %q\n",
		ProjectConfigName, project.Name, project.ID, env.Name,
	)
	return nil
}

func pickProject(projects []projectInfo) (projectInfo, error) {
	fmt.Fprintln(os.Stderr, "Projects available:")
	for i, p := range projects {
		fmt.Fprintf(os.Stderr, "  %d) %s (id %d, role %s)\n", i+1, p.Name, p.ID, p.Role)
	}
	idx, err := promptChoice("Select a project", len(projects))
	if err != nil {
		return projectInfo{}, err
	}
	return projects[idx], nil
}

func pickEnvironment(envs []environmentInfo) (environmentInfo, error) {
	fmt.Fprintln(os.Stderr, "Environments available:")
	for i, e := range envs {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, e.Name)
	}
	idx, err := promptChoice("Select an environment", len(envs))
	if err != nil {
		return environmentInfo{}, err
	}
	return envs[idx], nil
}
