// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package symmetry

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// EncryptAES encrypts valorCampo using AES-GCM and returns a Base64 payload
// with this layout:
//
//	Base64( nonce | ciphertext_and_tag )
//
// symmetricalAccess must be a Base64-encoded AES key whose decoded size is 16,
// 24, or 32 bytes.
func EncryptAES(symmetricalAccess, valorCampo, additionalData string) (string, error) {
	aesKeyBytes, err := base64.StdEncoding.DecodeString(symmetricalAccess)
	if err != nil {
		return "", fmt.Errorf("decode AES key: %w", err)
	}

	block, err := aes.NewCipher(aesKeyBytes)
	if err != nil {
		return "", fmt.Errorf("create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	cipherText := gcm.Seal(nil, nonce, []byte(valorCampo), []byte(additionalData))
	payload := append(nonce, cipherText...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

// DecryptAES decrypts payloads produced by EncryptAES using AES-GCM.
func DecryptAES(symmetricalAccess, valorCifrado, additionalData string) (string, error) {
	aesKeyBytes, err := base64.StdEncoding.DecodeString(symmetricalAccess)
	if err != nil {
		return "", fmt.Errorf("decode AES key: %w", err)
	}

	block, err := aes.NewCipher(aesKeyBytes)
	if err != nil {
		return "", fmt.Errorf("create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	allBytes, err := base64.StdEncoding.DecodeString(valorCifrado)
	if err != nil {
		return "", fmt.Errorf("decode Base64 ciphertext: %w", err)
	}

	if len(allBytes) < gcm.NonceSize() {
		return "", errors.New("ciphertext is too short")
	}

	nonce := allBytes[:gcm.NonceSize()]
	cipherText := allBytes[gcm.NonceSize():]
	plainText, err := gcm.Open(nil, nonce, cipherText, []byte(additionalData))
	if err != nil {
		return "", fmt.Errorf("decrypt AES-GCM: %w", err)
	}
	return string(plainText), nil
}
