package invite

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

// TokenCipher encrypts the invitation token before it is queued for email
// delivery, and decrypts it on the way out — see EventInvitationCreated's
// doc comment for why the outbox never carries the plaintext.
//
// AES-256-GCM was chosen over, say, hashing: a hash cannot be reversed to
// recover the token notify.InvitationHandler needs to put in the email
// link, so this has to be genuine encryption, not a digest.
type TokenCipher struct {
	key [32]byte
}

// NewTokenCipher derives an AES-256 key from secret via SHA-256, under a
// fixed, purpose-specific label. Deriving rather than using secret
// verbatim means the same underlying value (config.Auth.JWTSecret, in
// practice — see cmd/api/features.go and cmd/worker/run.go) yields a key
// distinct from whatever else that secret is used for, so a compromise of
// one derived use does not directly hand over the others.
func NewTokenCipher(secret string) *TokenCipher {
	return &TokenCipher{key: sha256.Sum256([]byte("goscaffoldforlevi:invite-token-cipher:v1:" + secret))}
}

// Encrypt returns an opaque, base64url-encoded ciphertext. The nonce is
// generated fresh per call and prepended to the sealed output, which is the
// standard arrangement for AES-GCM: the nonce is not secret, only
// single-use per key, and the recipient needs it back to decrypt.
func (c *TokenCipher) Encrypt(plaintext string) (string, error) {
	gcm, err := c.gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("invite: generate nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. An error here — a wrong key, a truncated or
// tampered value — is always treated as permanent by the only caller that
// matters (notify.InvitationHandler): no amount of retrying decodes a
// ciphertext correctly the second time.
func (c *TokenCipher) Decrypt(ciphertext string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("invite: decode ciphertext: %w", err)
	}
	gcm, err := c.gcm()
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invite: ciphertext shorter than a nonce")
	}
	nonce, sealed := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("invite: decrypt token: %w", err)
	}
	return string(plain), nil
}

func (c *TokenCipher) gcm() (cipher.AEAD, error) {
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return nil, fmt.Errorf("invite: build cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("invite: build GCM: %w", err)
	}
	return gcm, nil
}
