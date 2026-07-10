package domain

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Time    = 1
	argon2Memory  = 16 * 1024
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

type argonParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"keyLen"`
}

// Credential is an argon2id password hash.
type Credential struct {
	Salt   []byte
	Hash   []byte
	Algo   string
	Params argonParams
}

// NewCredential hashes a password with argon2id and a fresh random salt.
func NewCredential(password string) (Credential, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return Credential{}, err
	}
	params := argonParams{Time: argon2Time, Memory: argon2Memory, Threads: argon2Threads, KeyLen: argon2KeyLen}
	hash := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, params.KeyLen)
	return Credential{Salt: salt, Hash: hash, Algo: "argon2id", Params: params}, nil
}

// Verify reports whether the password matches, in constant time.
func (c Credential) Verify(password string) bool {
	if len(c.Hash) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), c.Salt, c.Params.Time, c.Params.Memory, c.Params.Threads, c.Params.KeyLen)
	return subtle.ConstantTimeCompare(got, c.Hash) == 1
}

// HashToken hashes a random token for safe storage.
func HashToken(token string) string {
	h := argon2.IDKey([]byte(token), []byte("auraedu-token-pepper"), 1, 8*1024, 2, 32)
	return base64.RawURLEncoding.EncodeToString(h)
}

// RandomToken returns a cryptographically random URL-safe token.
func RandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
