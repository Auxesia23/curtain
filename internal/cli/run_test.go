package cli

import (
	"errors"
	"slices"
	"testing"
)

// recordingRunner captures parsed `run` arguments without starting a real child process.
type recordingRunner struct {
	calls int
	opts  runOptions
	argv  []string
}

func (r *recordingRunner) run(opts runOptions, argv []string) error {
	r.calls++
	r.opts = opts
	r.argv = argv
	return nil
}

// TestRunCommandPassThrough verifies that target arguments, including target
// flags, pass through unchanged and Curtain flags are parsed correctly.
func TestRunCommandPassThrough(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantOpts runOptions
		wantArgv []string
	}{
		{
			name:     "with -- separator",
			args:     []string{"run", "--", "npm", "run", "dev"},
			wantArgv: []string{"npm", "run", "dev"},
		},
		{
			name:     "without -- separator",
			args:     []string{"run", "npm", "run", "dev"},
			wantArgv: []string{"npm", "run", "dev"},
		},
		{
			name:     "short flags before command",
			args:     []string{"run", "-e", "staging", "-p", "3", "--", "npm", "run", "dev"},
			wantOpts: runOptions{project: "3", env: "staging"},
			wantArgv: []string{"npm", "run", "dev"},
		},
		{
			name:     "long flags before command",
			args:     []string{"run", "--env", "staging", "--project", "my-app", "--", "npm", "run", "dev"},
			wantOpts: runOptions{project: "my-app", env: "staging"},
			wantArgv: []string{"npm", "run", "dev"},
		},
		{
			name:     "target flags after --",
			args:     []string{"run", "--", "npm", "run", "dev", "--save-dev", "-x"},
			wantArgv: []string{"npm", "run", "dev", "--save-dev", "-x"},
		},
		{
			name:     "target flags without --",
			args:     []string{"run", "npm", "run", "dev", "--save-dev"},
			wantArgv: []string{"npm", "run", "dev", "--save-dev"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &recordingRunner{}
			if code := execute(tc.args, recorder.run); code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if recorder.calls != 1 {
				t.Fatalf("runner calls = %d, want 1", recorder.calls)
			}
			if recorder.opts != tc.wantOpts {
				t.Fatalf("opts = %+v, want %+v", recorder.opts, tc.wantOpts)
			}
			if !slices.Equal(recorder.argv, tc.wantArgv) {
				t.Fatalf("argv = %v, want %v", recorder.argv, tc.wantArgv)
			}
		})
	}
}

// TestUsageExitCodes verifies that usage errors exit with code 2 without calling the runner.
func TestUsageExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no command at all", []string{}},
		{"run without command", []string{"run"}},
		{"run with only --", []string{"run", "--"}},
		{"unknown subcommand", []string{"bogus"}},
		{"unknown flag", []string{"run", "--bogus", "--", "npm"}},
		{"unexpected extra arg", []string{"login", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &recordingRunner{}
			if code := execute(tc.args, recorder.run); code != 2 {
				t.Fatalf("exit code = %d, want 2", code)
			}
			if recorder.calls != 0 {
				t.Fatalf("runner harus tidak dipanggil, calls = %d", recorder.calls)
			}
		})
	}
}

// TestRuntimeExitCode verifies that runtime failures exit with code 1.
func TestRuntimeExitCode(t *testing.T) {
	failing := func(runOptions, []string) error { return errors.New("boom") }
	if code := execute([]string{"run", "--", "npm"}, failing); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

// TestVersionExitCode verifies that `curtain -v` succeeds without calling the runner.
func TestVersionExitCode(t *testing.T) {
	recorder := &recordingRunner{}
	if code := execute([]string{"-v"}, recorder.run); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if recorder.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", recorder.calls)
	}
}
