package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
)

// pbkdf2Iterations is the work factor for PBKDF2-HMAC-SHA256 (OWASP 2023 guidance).
// argon2id is the eventual target (AURA-4.1); PBKDF2 keeps the service dependency-free.
const pbkdf2Iterations = 600_000

// Credential is a salted, stretched password hash. Plaintext passwords never leave
// this package and are never stored or logged.
type Credential struct {
	Salt       []byte
	Hash       []byte
	Iterations int
}

// NewCredential hashes a password with a fresh random 16-byte salt.
func NewCredential(password string) (Credential, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return Credential{}, err
	}
	return Credential{
		Salt:       salt,
		Hash:       pbkdf2SHA256([]byte(password), salt, pbkdf2Iterations),
		Iterations: pbkdf2Iterations,
	}, nil
}

// Verify reports whether the password matches, in constant time.
func (c Credential) Verify(password string) bool {
	if len(c.Hash) == 0 {
		return false
	}
	got := pbkdf2SHA256([]byte(password), c.Salt, c.Iterations)
	return subtle.ConstantTimeCompare(got, c.Hash) == 1
}

// pbkdf2SHA256 derives a 32-byte key (RFC 8018). dkLen == hLen (SHA-256 = 32 bytes),
// so a single block T_1 suffices.
func pbkdf2SHA256(password, salt []byte, iterations int) []byte {
	prf := func(data []byte) []byte {
		m := hmac.New(sha256.New, password)
		m.Write(data)
		return m.Sum(nil)
	}
	// U_1 = PRF(salt || INT_32_BE(1))
	u := prf(append(append([]byte{}, salt...), 0, 0, 0, 1))
	out := make([]byte, len(u))
	copy(out, u)
	for i := 1; i < iterations; i++ {
		u = prf(u)
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return out
}
