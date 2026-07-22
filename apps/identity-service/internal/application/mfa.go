package application

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/jwt"
	"github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
)

const mfaChallengeTTL = 5 * time.Minute

type LoginStartResult struct {
	Status         string
	AccessToken    string
	RefreshToken   string
	User           domain.User
	Expires        time.Time
	ChallengeToken string
	Secret         string
	OTPAuthURI     string
}

func WithPrivilegedMFA(secret string, required bool) Option {
	return func(s *Service) {
		key := sha256.Sum256([]byte(secret))
		s.mfaKey = key[:]
		s.mfaRequired = required
	}
}

func (s *Service) LoginStart(ctx context.Context, email, password string) (LoginStartResult, error) {
	u, err := s.authenticate(ctx, email, password)
	if err != nil {
		return LoginStartResult{}, err
	}
	if !s.mfaRequired || !privilegedRole(u.Role) {
		access, refresh, user, expires, issueErr := s.issueSession(ctx, u)
		return LoginStartResult{Status: "authenticated", AccessToken: access, RefreshToken: refresh, User: user, Expires: expires}, issueErr
	}

	_, enrolled, err := s.repo.GetMFA(ctx, u.ID)
	if err != nil {
		return LoginStartResult{}, err
	}
	purpose := "mfa_verify"
	secret := ""
	fingerprint := ""
	if !enrolled {
		purpose = "mfa_setup"
		secret, err = domain.NewTOTPSecret()
		if err != nil {
			return LoginStartResult{}, err
		}
		fingerprint = secretFingerprint(secret)
	}
	challenge, err := s.signMFAChallenge(u, purpose, fingerprint)
	if err != nil {
		return LoginStartResult{}, err
	}
	result := LoginStartResult{Status: purpose + "_required", ChallengeToken: challenge, User: publicChallengeUser(u)}
	if purpose == "mfa_setup" {
		result.Status = "mfa_setup_required"
		result.Secret = secret
		result.OTPAuthURI = totpURI(u, secret)
	} else {
		result.Status = "mfa_required"
	}
	return result, nil
}

func (s *Service) CompleteMFA(ctx context.Context, challengeToken, code, setupSecret string) (string, string, domain.User, time.Time, error) {
	if len(s.mfaKey) != 32 || challengeToken == "" || code == "" {
		return "", "", domain.User{}, time.Time{}, domain.ErrInvalidCredentials
	}
	claims, err := jwt.Verify(challengeToken, s.challengeKey(), s.now())
	if err != nil || (claims.TokenType != "mfa_setup" && claims.TokenType != "mfa_verify") {
		return "", "", domain.User{}, time.Time{}, domain.ErrInvalidCredentials
	}
	actor := tenancy.ActorFromContext(ctx)
	if claims.TenantID != "" && claims.TenantID != actor.TenantID {
		return "", "", domain.User{}, time.Time{}, domain.ErrInvalidCredentials
	}
	privileged := tenancy.WithActor(ctx, auth.Actor{PlatformAdmin: true})
	u, err := s.repo.GetUser(privileged, claims.Subject)
	if err != nil || u.Status != domain.StatusActive || u.Role != claims.Role || u.TenantID != claims.TenantID || !privilegedRole(u.Role) {
		return "", "", domain.User{}, time.Time{}, domain.ErrInvalidCredentials
	}

	secret, err := s.resolveMFASecret(ctx, u, claims, setupSecret)
	if err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}

	counter, err := domain.ValidateTOTP(secret, code, s.now())
	if err != nil {
		return "", "", domain.User{}, time.Time{}, domain.ErrInvalidCredentials
	}
	if err := s.commitMFA(ctx, u, claims.TokenType, secret, counter); err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}
	return s.issueSession(ctx, u)
}

func (s *Service) resolveMFASecret(ctx context.Context, user domain.User, claims jwt.Claims, setupSecret string) (string, error) {
	if claims.TokenType == "mfa_setup" {
		if setupSecret == "" || !hmac.Equal([]byte(secretFingerprint(setupSecret)), []byte(claims.Challenge)) {
			return "", domain.ErrInvalidCredentials
		}
		return setupSecret, nil
	}
	record, found, err := s.repo.GetMFA(ctx, user.ID)
	if err != nil || !found {
		return "", domain.ErrInvalidCredentials
	}
	secret, err := s.decryptMFASecret(user, record.EncryptedSecret)
	if err != nil {
		return "", domain.ErrInvalidCredentials
	}
	return secret, nil
}

func (s *Service) commitMFA(ctx context.Context, user domain.User, tokenType, secret string, counter int64) error {
	if tokenType != "mfa_setup" {
		if err := s.repo.AdvanceMFACounter(ctx, user.ID, counter); err != nil {
			return domain.ErrInvalidCredentials
		}
		return nil
	}
	encrypted, err := s.encryptMFASecret(user, secret)
	if err != nil {
		return err
	}
	return s.repo.SaveMFA(ctx, user.ID, encrypted, counter)
}

func (s *Service) signMFAChallenge(u domain.User, purpose, fingerprint string) (string, error) {
	now := s.now()
	return jwt.SignChallenge(jwt.Claims{
		Subject: u.ID, UserID: u.ID, TenantID: u.TenantID, Role: u.Role,
		Challenge: fingerprint, IssuedAt: now.Unix(), ExpiresAt: now.Add(mfaChallengeTTL).Unix(),
	}, purpose, s.challengeKey())
}

func (s *Service) challengeKey() []byte {
	sum := sha256.Sum256(append([]byte("auraedu-mfa-challenge:"), s.mfaKey...))
	return sum[:]
}

func (s *Service) encryptMFASecret(u domain.User, secret string) ([]byte, error) {
	block, err := aes.NewCipher(s.mfaKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(secret), mfaAAD(u)), nil
}

func (s *Service) decryptMFASecret(u domain.User, encrypted []byte) (string, error) {
	block, err := aes.NewCipher(s.mfaKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(encrypted) < gcm.NonceSize() {
		return "", errors.New("invalid MFA secret ciphertext")
	}
	plain, err := gcm.Open(nil, encrypted[:gcm.NonceSize()], encrypted[gcm.NonceSize():], mfaAAD(u))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func mfaAAD(u domain.User) []byte { return []byte(u.ID + "\x00" + u.TenantID) }

func secretFingerprint(secret string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(secret))))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func privilegedRole(role string) bool {
	return role == auth.RolePlatformSuperAdmin || role == "school_admin" || role == "super_admin"
}

func totpURI(u domain.User, secret string) string {
	label := url.PathEscape(fmt.Sprintf("%s:%s", firstNonEmpty(u.TenantID, "platform"), u.Email))
	query := url.Values{"secret": {secret}, "issuer": {"AuraEDU"}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + label + "?" + query.Encode()
}

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func publicChallengeUser(u domain.User) domain.User {
	u.Permissions = nil
	return u
}
