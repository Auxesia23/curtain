package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the CLI session and remove local credentials",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runLogout()
		},
	}
}

// runLogout revokes the CLI session on the server when reachable, then removes
// the local credentials from the OS keyring.
func runLogout() error {
	cfg, err := loadFileConfig()
	if err != nil {
		return err
	}
	if cfg == nil || cfg.Server == "" {
		fmt.Fprintln(os.Stderr, "curtain: no stored credentials")
		return nil
	}

	creds, err := loadCredentials(cfg.Server)
	if err != nil {
		return err
	}
	if creds != nil {
		client := newAPIClient(cfg.Server)
		client.token = creds.Token
		if err := client.logout(); err != nil {
			fmt.Fprintf(os.Stderr, "curtain: warning: failed to revoke the session on the server (%v)\n", err)
		}
	}
	if err := deleteCredentials(cfg.Server); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "curtain: logged out from %s (local credentials removed)\n", cfg.Server)
	return nil
}
