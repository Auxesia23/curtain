package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// stdinReader is shared by all prompts so buffered input is not consumed
// unexpectedly across consecutive questions.
var stdinReader = bufio.NewReader(os.Stdin)

// promptLine reads one line of input and returns fallback when it is empty.
func promptLine(label, fallback string) (string, error) {
	if fallback != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", label, fallback)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", label)
	}
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback, nil
	}
	return line, nil
}

// readPassword reads without echo when stdin is a TTY. Otherwise, it reads one
// line from stdin so the command can be scripted through a pipe.
func readPassword(label string) (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprintf(os.Stderr, "%s: ", label)
		pass, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		return string(pass), nil
	}
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("failed to read password from stdin: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// promptChoice asks the user to select one of count choices (1-based).
func promptChoice(label string, count int) (int, error) {
	for {
		line, err := promptLine(fmt.Sprintf("%s [1-%d]", label, count), "")
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > count {
			fmt.Fprintf(os.Stderr, "curtain: enter a number between 1 and %d\n", count)
			continue
		}
		return n - 1, nil
	}
}

// confirm asks for a yes/no confirmation, defaulting to no.
func confirm(label string) (bool, error) {
	line, err := promptLine(label+" [y/N]", "")
	if err != nil {
		return false, err
	}
	switch strings.ToLower(line) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}
