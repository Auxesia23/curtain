package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// keyringService is the service name in the OS keyring. Credentials use the
// server URL as the account so multiple Curtain instances can coexist.
const keyringService = "curtain"

// credentialBlob contains secrets stored in the OS keyring: the CLI session
// token and wrapKey, which decrypts the user's private key. wrapKey is as
// sensitive as the private key and is never written to a regular file.
type credentialBlob struct {
	Version int    `json:"v"`
	Email   string `json:"email"`
	Token   string `json:"token"`
	WrapKey string `json:"wrap_key"`
}

func saveCredentials(server string, cred *credentialBlob) error {
	cred.Version = configVersion
	data, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	if err := keyring.Set(keyringService, normalizeServer(server), string(data)); err != nil {
		return keyringError(err)
	}
	return nil
}

// loadCredentials reads credentials from the keyring and returns (nil, nil) if absent.
func loadCredentials(server string) (*credentialBlob, error) {
	raw, err := keyring.Get(keyringService, normalizeServer(server))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, keyringError(err)
	}
	var cred credentialBlob
	if err := json.Unmarshal([]byte(raw), &cred); err != nil {
		return nil, fmt.Errorf("credentials in the keyring are corrupt: %w (try `curtain login` again)", err)
	}
	if cred.Token == "" || cred.WrapKey == "" {
		return nil, errors.New("credentials in the keyring are incomplete (try `curtain login` again)")
	}
	return &cred, nil
}

// deleteCredentials removes credentials from the keyring. It is idempotent.
func deleteCredentials(server string) error {
	err := keyring.Delete(keyringService, normalizeServer(server))
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return keyringError(err)
	}
	return nil
}

// keyringError adds actionable guidance to keyring errors, especially on Linux,
// where Secret Service (gnome-keyring/KWallet) must be available.
func keyringError(err error) error {
	return fmt.Errorf(
		"cannot access the OS keyring: %w\n"+
			"  on Linux this requires a running Secret Service (gnome-keyring/KWallet); "+
			"on headless/SSH sessions, log in from a machine with a desktop session",
		err,
	)
}
