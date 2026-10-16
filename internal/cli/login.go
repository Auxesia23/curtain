package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// loginOptions contains flags for `curtain login`.
type loginOptions struct {
	server string
	email  string
}

func newLoginCmd() *cobra.Command {
	var opts loginOptions
	cmd := &cobra.Command{
		Use:     "login [flags]",
		Short:   "Log in and store credentials in the OS keyring",
		Example: "  curtain login --server https://vault.example.com",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runLogin(opts)
		},
	}
	cmd.Flags().StringVar(&opts.server, "server", "", "Curtain server URL")
	cmd.Flags().StringVar(&opts.email, "email", "", "account email")
	return cmd
}

// runLogin performs the CLI login flow:
// GET /auth/kdf -> derive masterKey/authKey -> POST /auth/login (CLI client)
// -> store the token and wrapKey in the OS keyring.
func runLogin(opts loginOptions) error {
	cfg, err := loadFileConfig()
	if err != nil {
		return err
	}

	server := opts.server
	if server == "" && cfg != nil {
		server = cfg.Server
	}
	if server == "" {
		server, err = promptLine("Server URL", defaultServer)
		if err != nil {
			return err
		}
	}
	server = normalizeServer(server)
	if server == "" {
		return errors.New("server URL is empty")
	}

	email := opts.email
	if email == "" && cfg != nil {
		email = cfg.Email
	}
	if email == "" {
		email, err = promptLine("Email", "")
		if err != nil {
			return err
		}
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("email is empty")
	}

	client := newAPIClient(server)
	kdf, err := client.kdf(email)
	if err != nil {
		if apiStatus(err) == 404 {
			return fmt.Errorf("account %s is not registered on %s (register via the web UI first)", email, server)
		}
		return err
	}

	password, err := readPassword("Password")
	if err != nil {
		return err
	}
	if password == "" {
		return errors.New("password is empty")
	}

	params, err := parseKDFParams(kdf.KDFParams)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "curtain: deriving key (argon2id)...")
	masterKey, err := deriveMasterKey(password, kdf.KDFSalt, params)
	if err != nil {
		return err
	}
	defer wipeBytes(masterKey)

	authKey, err := deriveAuthKey(masterKey)
	if err != nil {
		return err
	}
	res, err := client.login(email, authKey)
	if err != nil {
		switch apiStatus(err) {
		case 401:
			return errors.New("invalid email or password")
		case 403:
			return errors.New("account has been disabled by an admin")
		}
		return err
	}
	if res.Token == "" {
		return errors.New("server did not return a CLI token")
	}

	wrapKey, err := deriveWrapKey(masterKey)
	if err != nil {
		return err
	}
	if _, err := decryptPrivateKey(wrapKey, res.User.EncryptedPrivateKey); err != nil {
		return fmt.Errorf("failed to open private key: %w", err)
	}

	if err := saveCredentials(server, &credentialBlob{
		Email:   email,
		Token:   res.Token,
		WrapKey: b64Encode(wrapKey),
	}); err != nil {
		return err
	}
	if err := saveFileConfig(&fileConfig{Server: server, Email: email}); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "curtain: login successful — %s @ %s\n", email, server)
	return nil
}

// wipeBytes clears key material from memory after use.
func wipeBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
