// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package symmetry

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptDecryptAES(t *testing.T) {
	aesKey := base64.StdEncoding.EncodeToString([]byte("1234567890ABCDEF"))
	additionalData := "context-aad"

	encrypted, err := EncryptAES(aesKey, "hello world", additionalData)
	if err != nil {
		t.Fatalf("expected encrypt without error, got %v", err)
	}

	decrypted, err := DecryptAES(aesKey, encrypted, additionalData)
	if err != nil {
		t.Fatalf("expected decrypt without error, got %v", err)
	}

	if decrypted != "hello world" {
		t.Fatalf("expected plaintext %q, got %q", "hello world", decrypted)
	}
}

func TestEncryptAESErrors(t *testing.T) {
	_, err := EncryptAES("%%%invalid-base64%%%", "hello", "")
	if err == nil || !strings.Contains(err.Error(), "decode AES key") {
		t.Fatalf("expected AES decode error, got %v", err)
	}

	_, err = EncryptAES(base64.StdEncoding.EncodeToString([]byte("short")), "hello", "")
	if err == nil || !strings.Contains(err.Error(), "create AES cipher") {
		t.Fatalf("expected AES cipher error, got %v", err)
	}
}

func TestDecryptAESErrors(t *testing.T) {
	validAESKey := base64.StdEncoding.EncodeToString([]byte("1234567890ABCDEF"))

	_, err := DecryptAES("%%%invalid-base64%%%", "value", "")
	if err == nil || !strings.Contains(err.Error(), "decode AES key") {
		t.Fatalf("expected AES decode error, got %v", err)
	}

	_, err = DecryptAES(validAESKey, "%%%invalid-base64%%", "")
	if err == nil || !strings.Contains(err.Error(), "decode Base64 ciphertext") {
		t.Fatalf("expected encrypted payload decode error, got %v", err)
	}

	shortCipher := base64.StdEncoding.EncodeToString([]byte("tiny"))
	_, err = DecryptAES(validAESKey, shortCipher, "")
	if err == nil || !strings.Contains(err.Error(), "ciphertext is too short") {
		t.Fatalf("expected short encrypted data error, got %v", err)
	}

	invalidCipher := base64.StdEncoding.EncodeToString(append(make([]byte, 12), []byte("tampered")...))
	_, err = DecryptAES(validAESKey, invalidCipher, "")
	if err == nil || !strings.Contains(err.Error(), "decrypt AES-GCM") {
		t.Fatalf("expected AES-GCM decrypt error, got %v", err)
	}
}

func TestDecryptAESWithInvalidAdditionalData(t *testing.T) {
	aesKey := base64.StdEncoding.EncodeToString([]byte("1234567890ABCDEF"))

	encrypted, err := EncryptAES(aesKey, "hello world", "aad-ok")
	if err != nil {
		t.Fatalf("expected encrypt without error, got %v", err)
	}

	_, err = DecryptAES(aesKey, encrypted, "aad-wrong")
	if err == nil || !strings.Contains(err.Error(), "decrypt AES-GCM") {
		t.Fatalf("expected AES-GCM decrypt error by AAD mismatch, got %v", err)
	}
}
