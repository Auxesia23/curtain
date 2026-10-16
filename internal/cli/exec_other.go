//go:build !unix

package cli

import "errors"

// execProcess is not supported on non-Unix systems such as Windows. The stub
// keeps `go build ./...` working for cross-platform compilation.
func execProcess(string, []string, []string) error {
	return errors.New("`curtain run` is not supported on this OS yet (Linux/macOS/BSD only)")
}
