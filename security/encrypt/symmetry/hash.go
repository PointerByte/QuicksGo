// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package symmetry

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/zeebo/blake3"
)

// GenerateHMAC computes an HMAC-SHA256 for message using secretKey and returns
// the digest encoded as standard Base64.
func GenerateHMAC(message, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ValidateHMAC reports whether providedHash matches the HMAC-SHA256 generated
// from message and secretKey.
func ValidateHMAC(message, secretKey, providedHash string) bool {
	expectedHash := GenerateHMAC(message, secretKey)
	return hmac.Equal([]byte(expectedHash), []byte(providedHash))
}

// Sha256Hex returns the SHA-256 digest of message encoded as hexadecimal.
func Sha256Hex(message string) string {
	hash := sha256.Sum256([]byte(message))
	return hex.EncodeToString(hash[:])
}

// Blake3 returns the BLAKE3 digest of message encoded as standard Base64.
func Blake3(message string) string {
	hash := blake3.Sum256([]byte(message))
	return base64.StdEncoding.EncodeToString(hash[:])
}
