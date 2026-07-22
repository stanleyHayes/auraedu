package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- SHA-1 is required by RFC 6238 TOTP, not used for signatures.
	"encoding/base32"
	"encoding/binary"
	"strconv"
	"strings"
	"time"
)

const totpPeriod = int64(30)

func NewTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// ValidateTOTP accepts the current RFC 6238 code with one 30-second step of clock skew.
// It returns the accepted counter so repositories can reject code replay atomically.
func ValidateTOTP(secret, code string, now time.Time) (int64, error) {
	if len(code) != 6 {
		return 0, ErrInvalidCredentials
	}
	if _, err := strconv.Atoi(code); err != nil {
		return 0, ErrInvalidCredentials
	}
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(decoded) < 16 {
		return 0, ErrInvalidCredentials
	}
	current := now.Unix() / totpPeriod
	for offset := int64(-1); offset <= 1; offset++ {
		counter := current + offset
		if counter < 0 {
			continue
		}
		expected := totpCode(decoded, uint64(counter))
		if hmac.Equal([]byte(expected), []byte(code)) {
			return counter, nil
		}
	}
	return 0, ErrInvalidCredentials
}

func totpCode(secret []byte, counter uint64) string {
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], counter)
	mac := hmac.New(sha1.New, secret) // #nosec G401 -- mandated by RFC 6238 compatibility.
	_, _ = mac.Write(message[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return strconv.Itoa(int(value%1_000_000) + 1_000_000)[1:]
}
