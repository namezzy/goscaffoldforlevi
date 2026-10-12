package auth_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
)

func TestTokenCipherRoundTrips(t *testing.T) {
	t.Parallel()

	cipher := auth.NewTokenCipher("a-test-secret")
	const plaintext = "9x1Q4r-NiXPAO7k_J5Lew9f8Tro7f5sVCO428fTrXx0"

	ciphertext, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() = %v", err)
	}
	if ciphertext == plaintext {
		t.Fatal("Encrypt() returned the plaintext unchanged")
	}
	if strings.Contains(ciphertext, plaintext) {
		t.Fatal("the ciphertext contains the plaintext token as a substring")
	}

	decrypted, err := cipher.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() = %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

// TestTokenCipherEncryptionIsNotDeterministic guards against a fresh nonce
// not actually being generated per call — see invite.TestTokenCipherEncryptionIsNotDeterministic
// for the identical reasoning this mirrors.
func TestTokenCipherEncryptionIsNotDeterministic(t *testing.T) {
	t.Parallel()

	cipher := auth.NewTokenCipher("a-test-secret")
	first, err := cipher.Encrypt("same-plaintext")
	if err != nil {
		t.Fatalf("Encrypt() = %v", err)
	}
	second, err := cipher.Encrypt("same-plaintext")
	if err != nil {
		t.Fatalf("Encrypt() = %v", err)
	}
	if first == second {
		t.Error("two encryptions of the same plaintext produced identical ciphertext")
	}
}

func TestTokenCipherDecryptFailsWithTheWrongKey(t *testing.T) {
	t.Parallel()

	ciphertext, err := auth.NewTokenCipher("secret-one").Encrypt("a-token")
	if err != nil {
		t.Fatalf("Encrypt() = %v", err)
	}
	if _, err := auth.NewTokenCipher("secret-two").Decrypt(ciphertext); err == nil {
		t.Error("Decrypt() with the wrong key succeeded")
	}
}

func TestTokenCipherDecryptFailsOnTamperedCiphertext(t *testing.T) {
	t.Parallel()

	cipher := auth.NewTokenCipher("a-test-secret")
	ciphertext, err := cipher.Encrypt("a-token")
	if err != nil {
		t.Fatalf("Encrypt() = %v", err)
	}

	// See invite.TestTokenCipherDecryptFailsOnTamperedCiphertext's doc
	// comment for why this flips a decoded byte rather than editing the
	// last base64 character: the latter is flaky, because base64's final
	// quantum can carry unused bits.
	raw, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("decode ciphertext: %v", err)
	}
	raw[len(raw)-1] ^= 0xFF
	tampered := base64.RawURLEncoding.EncodeToString(raw)

	if _, err := cipher.Decrypt(tampered); err == nil {
		t.Error("Decrypt() accepted a tampered ciphertext (GCM should have caught this)")
	}
}
