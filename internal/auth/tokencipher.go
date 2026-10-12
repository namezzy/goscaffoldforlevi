package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// TokenCipher encrypts a password reset token before it is queued for email
// delivery, and decrypts it on the way out — mirroring
// invite.TokenCipher's reasoning exactly (see that type's doc comment): the
// outbox is a generic events table, readable through the messaging admin
// endpoints and the dead-letter queue, so the plaintext, redeemable token
// must never sit in it even transiently. A hash — what
// password_reset_tokens stores at rest — cannot be reversed back into the
// link the email needs, so this has to be genuine encryption.
//
// This is a second, separate cipher rather than a shared one: auth is a
// lower-level package invite itself depends on (for NewRefreshToken), so
// auth cannot import invite without a cycle, and the two token kinds are
// unrelated besides sharing this technique.
type TokenCipher struct {
	key [32]byte
}

// NewTokenCipher derives an AES-256 key from secret via SHA-256, under a
// fixed, purpose-specific label — see invite.NewTokenCipher's doc comment
// for why deriving rather than using secret verbatim matters.
func NewTokenCipher(secret string) *TokenCipher {
	return &TokenCipher{key: sha256.Sum256([]byte("goscaffoldforlevi:auth-password-reset-cipher:v1:" + secret))}
}

// Encrypt returns an opaque, base64url-encoded ciphertext.
func (c *TokenCipher) Encrypt(plaintext string) (string, error) {
	gcm, err := c.gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("auth: generate nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. Any failure here — wrong key, truncated or
// tampered value — is permanent: no amount of retrying decodes a
// ciphertext correctly the second time.
func (c *TokenCipher) Decrypt(ciphertext string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("auth: decode ciphertext: %w", err)
	}
	gcm, err := c.gcm()
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("auth: ciphertext shorter than a nonce")
	}
	nonce, sealed := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("auth: decrypt token: %w", err)
	}
	return string(plain), nil
}

func (c *TokenCipher) gcm() (cipher.AEAD, error) {
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return nil, fmt.Errorf("auth: build cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("auth: build GCM: %w", err)
	}
	return gcm, nil
}
