// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package symmetry

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestDecodeFernetKey(t *testing.T) {
	keyBytes := []byte("1234567890ABCDEF1234567890ABCDEF")
	key, err := decodeFernetKey(base64.StdEncoding.EncodeToString(keyBytes))
	if err != nil {
		t.Fatalf("expected decode without error, got %v", err)
	}

	if string(key.signingKey) != "1234567890ABCDEF" {
		t.Fatalf("unexpected signing key: %q", string(key.signingKey))
	}

	if string(key.encryptionKey) != "1234567890ABCDEF" {
		t.Fatalf("unexpected encryption key: %q", string(key.encryptionKey))
	}
}

func TestDecodeFernetKeyErrors(t *testing.T) {
	_, err := decodeFernetKey("%%%invalid-base64%%%")
	if err == nil {
		t.Fatal("expected base64 decode error")
	}

	shortKey := base64.StdEncoding.EncodeToString([]byte("short"))
	_, err = decodeFernetKey(shortKey)
	if err == nil || !strings.Contains(err.Error(), "fernet key must be 32 bytes") {
		t.Fatalf("expected invalid length error, got %v", err)
	}
}

func TestEncodeDecodeFernet(t *testing.T) {
	keyString := base64.StdEncoding.EncodeToString([]byte("1234567890ABCDEF1234567890ABCDEF"))

	token, err := EncodeFernet(keyString, "hello fernet")
	if err != nil {
		t.Fatalf("expected encode without error, got %v", err)
	}

	plainText, err := DecodeFernet(keyString, token)
	if err != nil {
		t.Fatalf("expected decode without error, got %v", err)
	}

	if plainText != "hello fernet" {
		t.Fatalf("expected plaintext %q, got %q", "hello fernet", plainText)
	}
}

func TestDecodeFernetErrors(t *testing.T) {
	keyString := base64.StdEncoding.EncodeToString([]byte("1234567890ABCDEF1234567890ABCDEF"))

	_, err := DecodeFernet("%%%invalid-base64%%%", "token")
	if err == nil {
		t.Fatal("expected invalid key error")
	}

	_, err = DecodeFernet(keyString, "%%%invalid-base64%%%")
	if err == nil {
		t.Fatal("expected invalid token base64 error")
	}

	shortToken := base64.URLEncoding.EncodeToString([]byte("tiny"))
	_, err = DecodeFernet(keyString, shortToken)
	if err == nil || !strings.Contains(err.Error(), "fernet token is too short") {
		t.Fatalf("expected short token error, got %v", err)
	}

	validToken, err := EncodeFernet(keyString, "hello")
	if err != nil {
		t.Fatalf("expected encode without error, got %v", err)
	}

	rawToken, err := base64.URLEncoding.DecodeString(validToken)
	if err != nil {
		t.Fatalf("expected raw token without error, got %v", err)
	}

	rawToken[len(rawToken)-1] ^= 0x01
	tamperedToken := base64.URLEncoding.EncodeToString(rawToken)
	_, err = DecodeFernet(keyString, tamperedToken)
	if err == nil || !strings.Contains(err.Error(), "invalid HMAC signature") {
		t.Fatalf("expected invalid signature error, got %v", err)
	}

	ciphertextInvalidToken := mustInvalidCiphertextFernetToken(t, keyString)
	_, err = DecodeFernet(keyString, ciphertextInvalidToken)
	if err == nil || !strings.Contains(err.Error(), "ciphertext is not a multiple of the block size") {
		t.Fatalf("expected ciphertext block size error, got %v", err)
	}
}

func mustInvalidCiphertextFernetToken(t *testing.T, keyString string) string {
	t.Helper()

	key, err := decodeFernetKey(keyString)
	if err != nil {
		t.Fatalf("expected key without error, got %v", err)
	}

	message := append([]byte{0x80}, make([]byte, 8)...)
	message = append(message, make([]byte, 16)...)
	message = append(message, []byte{0x01}...)

	mac := hmac.New(sha256.New, key.signingKey)
	if _, err := mac.Write(message); err != nil {
		t.Fatalf("expected hmac write without error, got %v", err)
	}

	token := append(message, mac.Sum(nil)...)
	return base64.URLEncoding.EncodeToString(token)
}
