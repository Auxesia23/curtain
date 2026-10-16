//go:build unix

package cli

import "syscall"

// execProcess replaces the CLI process with the requested command, like `exec`
// in a shell. The TTY, signals (Ctrl+C), and exit code are inherited directly.
func execProcess(binPath string, argv, env []string) error {
	return syscall.Exec(binPath, argv, env)
}
