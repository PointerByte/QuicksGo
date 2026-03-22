// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package symmetry

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"
)

// Internal helpers for Fernet support.

type fernetKey struct {
	signingKey    []byte
	encryptionKey []byte
}

// decodeFernetKey decodes a Fernet key from Base64 and splits it into signing
// and encryption keys.
func decodeFernetKey(b64 string) (*fernetKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(keyBytes) != 32 {
		return nil, errors.New("fernet key must be 32 bytes")
	}

	return &fernetKey{
		signingKey:    keyBytes[:16],
		encryptionKey: keyBytes[16:32],
	}, nil
}

// EncodeFernet creates a standard Fernet token compatible with common Fernet
// implementations.
//
// keyString must decode to exactly 32 bytes. The token includes version,
// timestamp, IV, AES-128-CBC ciphertext, and an HMAC-SHA256 signature, and is
// returned as URL-safe Base64.
func EncodeFernet(keyString, originalString string) (string, error) {
	key, err := decodeFernetKey(keyString)
	if err != nil {
		return "", err
	}

	// Version (0x80) and timestamp (8 bytes).
	token := make([]byte, 9)
	token[0] = 0x80
	ts := uint64(time.Now().Unix())
	for i := range 8 {
		token[8-i] = byte(ts & 0xff)
		ts >>= 8
	}

	// Random 16-byte IV.
	iv := make([]byte, 16)
	_, err = rand.Read(iv)
	if err != nil {
		return "", err
	}
	token = append(token, iv...)

	// AES-CBC encryption.
	block, err := aes.NewCipher(key.encryptionKey)
	if err != nil {
		return "", err
	}

	plaintext := pkcs7Pad([]byte(originalString), aes.BlockSize)
	ciphertext := make([]byte, len(plaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)
	token = append(token, ciphertext...)

	// HMAC-SHA256 over the complete token body.
	mac := hmac.New(sha256.New, key.signingKey)
	mac.Write(token)
	signature := mac.Sum(nil)
	token = append(token, signature...)
	return base64.URLEncoding.EncodeToString(token), nil
}

// DecodeFernet validates and decrypts a standard Fernet token.
//
// It verifies the token structure, validates the HMAC-SHA256 signature,
// decrypts the ciphertext with AES-128-CBC, removes PKCS#7 padding, and
// returns the original plaintext.
func DecodeFernet(keyString, encryptedString string) (string, error) {
	key, err := decodeFernetKey(keyString)
	if err != nil {
		return "", err
	}

	token, err := base64.URLEncoding.DecodeString(encryptedString)
	if err != nil {
		return "", err
	}
	if len(token) < 9+16+32 {
		return "", errors.New("fernet token is too short")
	}

	// Split message and signature.
	message := token[:len(token)-32]
	signature := token[len(token)-32:]

	// Validate HMAC.
	mac := hmac.New(sha256.New, key.signingKey)
	mac.Write(message)
	expected := mac.Sum(nil)
	if !hmac.Equal(expected, signature) {
		return "", errors.New("invalid HMAC signature")
	}

	// Extract IV and ciphertext.
	iv := message[9 : 9+16]
	ciphertext := message[9+16:]

	// AES-CBC decryption.
	block, err := aes.NewCipher(key.encryptionKey)
	if err != nil {
		return "", err
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext is not a multiple of the block size")
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	plaintext, err = pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
