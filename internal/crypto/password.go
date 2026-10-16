// Package crypto contains server-side cryptographic primitives.
//
// The server never receives user passwords. Clients derive an `authKey` from
// the password with Argon2id and send it over HTTPS; the server stores only
// an Argon2id hash of that value.
package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Server-side authKey hashing parameters. This is lighter than the client KDF
// while remaining computationally expensive.
const (
	argonMemory  = 64 * 1024 // KiB (64 MiB)
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

var ErrInvalidHash = errors.New("invalid hash format")

// HashSecret returns an Argon2id hash in PHC string format:
// $argon2id$v=19$m=65536,t=2,p=1$<salt>$<hash>
func HashSecret(secret string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(secret), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonTime,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifySecret compares a secret with its stored hash in constant time.
func VerifySecret(secret, encoded string) error {
	params, salt, want, err := decodeHash(encoded)
	if err != nil {
		return err
	}
	got := argon2.IDKey([]byte(secret), salt, params.time, params.memory, params.threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return errors.New("credentials do not match")
	}
	return nil
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

func decodeHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return argonParams{}, nil, nil, ErrInvalidHash
	}

	var params argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.memory, &params.time, &params.threads); err != nil {
		return argonParams{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, ErrInvalidHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, ErrInvalidHash
	}
	return params, salt, key, nil
}
