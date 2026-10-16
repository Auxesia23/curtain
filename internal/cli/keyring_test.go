package cli

import (
	"testing"

	"github.com/zalando/go-keyring"
)

// TestKeyringRoundTrip uses go-keyring's in-memory backend instead of the
// actual operating system keyring.
func TestKeyringRoundTrip(t *testing.T) {
	keyring.MockInit()

	server := "https://vault.example.com"
	creds := &credentialBlob{Email: "fajar@example.com", Token: "tok-123", WrapKey: "d2l0aA=="}

	if got, err := loadCredentials(server); err != nil || got != nil {
		t.Fatalf("kredensial awal: got=%v err=%v", got, err)
	}

	if err := saveCredentials(server, creds); err != nil {
		t.Fatalf("saveCredentials: %v", err)
	}
	got, err := loadCredentials(server)
	if err != nil {
		t.Fatalf("loadCredentials: %v", err)
	}
	if got == nil || got.Email != creds.Email || got.Token != creds.Token || got.WrapKey != creds.WrapKey {
		t.Fatalf("kredensial tidak sesuai: %+v", got)
	}

	// Server URL normalization should map trailing slashes to the same entry.
	if _, err := loadCredentials(server + "/"); err != nil {
		t.Fatalf("load dengan trailing slash: %v", err)
	}

	if err := deleteCredentials(server); err != nil {
		t.Fatalf("deleteCredentials: %v", err)
	}
	if got, err := loadCredentials(server); err != nil || got != nil {
		t.Fatalf("setelah delete: got=%v err=%v", got, err)
	}
	// Deletion is idempotent.
	if err := deleteCredentials(server); err != nil {
		t.Fatalf("delete kedua: %v", err)
	}
}

func TestLoadCredentialsRejectsIncomplete(t *testing.T) {
	keyring.MockInit()
	server := "https://vault.example.com"
	if err := keyring.Set(keyringService, server, `{"v":1,"email":"a@b.c","token":"","wrap_key":""}`); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCredentials(server); err == nil {
		t.Fatal("kredensial tanpa token/wrap_key seharusnya ditolak")
	}
}
