// Package cli implements the `curtain` CLI client.
//
// All cryptography follows the specification in README.md and must remain
// compatible with the browser implementation in ui/src/lib/crypto. The server
// never sees plaintext; the CLI decrypts variables locally and injects them
// into the child process environment.
package cli

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/hkdf"
)

const (
	// HKDF domain separation salt, as specified in the README.
	kdfSaltInfo = "curtain.kdf.v1"
	authKeyInfo = "auth-key-v1"
	wrapKeyInfo = "private-key-wrap-v1"

	privateKeyAAD = "curtain.private-key.v1"
	variableAAD   = "curtain.variable.v1"

	ivLength = 12
)

// kdfParams contains the client-selected Argon2id parameters. Its JSON shape
// matches dto.KDFParams on the server.
type kdfParams struct {
	Memory      uint32 `json:"m"`
	Iterations  uint32 `json:"t"`
	Parallelism uint32 `json:"p"`
	HashLength  uint32 `json:"hash_length"`
}

// defaultKDFParams matches DEFAULT_KDF_PARAMS in the web UI.
func defaultKDFParams() kdfParams {
	return kdfParams{Memory: 65536, Iterations: 3, Parallelism: 1, HashLength: 32}
}

// parseKDFParams reads the server's JSON KDF parameters and fills missing fields
// with defaults.
func parseKDFParams(raw string) (kdfParams, error) {
	params := defaultKDFParams()
	if raw == "" {
		return params, nil
	}
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return kdfParams{}, fmt.Errorf("invalid KDF parameters from server: %w", err)
	}
	if params.Memory == 0 || params.Iterations == 0 || params.Parallelism == 0 || params.HashLength == 0 {
		return kdfParams{}, errors.New("KDF parameters from server are incomplete")
	}
	return params, nil
}

func b64Encode(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

func b64Decode(text string) ([]byte, error) { return base64.StdEncoding.DecodeString(text) }

// deriveMasterKey derives the master key from a password with Argon2id.
// The operation is intentionally expensive and uses about 64 MiB of memory.
func deriveMasterKey(password, saltB64 string, params kdfParams) ([]byte, error) {
	salt, err := b64Decode(saltB64)
	if err != nil {
		return nil, fmt.Errorf("invalid KDF salt from server: %w", err)
	}
	return argon2.IDKey(
		[]byte(password), salt,
		params.Iterations, params.Memory, uint8(params.Parallelism), params.HashLength,
	), nil
}

// hkdfExpand derives 32 bytes with HKDF-SHA256 and domain separation.
func hkdfExpand(masterKey []byte, info string) ([]byte, error) {
	reader := hkdf.New(sha256.New, masterKey, []byte(kdfSaltInfo), []byte(info))
	out := make([]byte, 32)
	if _, err := io.ReadFull(reader, out); err != nil {
		return nil, fmt.Errorf("failed to derive key from master key: %w", err)
	}
	return out, nil
}

// deriveAuthKey returns the base64 authKey, the only password-derived value
// sent to the server during login.
func deriveAuthKey(masterKey []byte) (string, error) {
	raw, err := hkdfExpand(masterKey, authKeyInfo)
	if err != nil {
		return "", err
	}
	return b64Encode(raw), nil
}

// deriveWrapKey derives the AES-256 key used to decrypt the private key.
func deriveWrapKey(masterKey []byte) ([]byte, error) {
	return hkdfExpand(masterKey, wrapKeyInfo)
}

// decryptPrivateKey opens the base64(iv(12) || AES-256-GCM(PKCS#8)) blob with
// wrapKey and returns the RSA private key.
func decryptPrivateKey(wrapKey []byte, blobB64 string) (*rsa.PrivateKey, error) {
	pkcs8, err := decryptGCM(wrapKey, blobB64, []byte(privateKeyAAD))
	if err != nil {
		return nil, fmt.Errorf("failed to open private key: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(pkcs8)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return key, nil
}

// unwrapProjectKey decrypts an RSA-OAEP-SHA256-wrapped project key with the
// user's private key.
func unwrapProjectKey(privateKey *rsa.PrivateKey, wrappedB64 string) ([]byte, error) {
	wrapped, err := b64Decode(wrappedB64)
	if err != nil {
		return nil, fmt.Errorf("invalid encrypted project key: %w", err)
	}
	raw, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, wrapped, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap project key: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("unexpected project key size (%d bytes)", len(raw))
	}
	return raw, nil
}

// decryptVariable decrypts a variable name or value with the project key.
func decryptVariable(projectKey []byte, blobB64 string) (string, error) {
	plain, err := decryptGCM(projectKey, blobB64, []byte(variableAAD))
	if err != nil {
		return "", fmt.Errorf("failed to decrypt variable: %w", err)
	}
	return string(plain), nil
}

// decryptGCM opens Curtain's standard base64(iv(12) || ciphertext) format.
func decryptGCM(key []byte, blobB64 string, aad []byte) ([]byte, error) {
	blob, err := b64Decode(blobB64)
	if err != nil {
		return nil, err
	}
	if len(blob) <= ivLength {
		return nil, errors.New("encrypted blob is too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, blob[:ivLength], blob[ivLength:], aad)
}
