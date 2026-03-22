// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
)

// ParseRSAPublicKeyFromBase64 converts a Base64-encoded X.509 DER key into an
// *rsa.PublicKey.
func ParseRSAPublicKeyFromBase64(b64 string) (*rsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode public key from Base64: %w", err)
	}

	pubIfc, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	pub, ok := pubIfc.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not an RSA key")
	}
	return pub, nil
}

// ParseRSAPrivateKeyFromBase64 converts a Base64-encoded PKCS#8 DER key into
// an *rsa.PrivateKey.
func ParseRSAPrivateKeyFromBase64(b64 string) (*rsa.PrivateKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode private key from Base64: %w", err)
	}

	keyIfc, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	priv, ok := keyIfc.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an RSA key")
	}
	return priv, nil
}

// Encode encrypts text with RSA-OAEP (SHA-256, MGF1-SHA256) and returns Base64.
func Encode(key, text string) (string, error) {
	pub, err := ParseRSAPublicKeyFromBase64(key)
	if err != nil {
		return "", fmt.Errorf("load public key: %w", err)
	}

	hash := sha256.New()
	cipherBytes, err := rsa.EncryptOAEP(hash, rand.Reader, pub, []byte(text), nil)
	if err != nil {
		return "", fmt.Errorf("encrypt with RSA-OAEP: %w", err)
	}
	return base64.StdEncoding.EncodeToString(cipherBytes), nil
}

// Decode decrypts a Base64 string with RSA-OAEP (SHA-256, MGF1-SHA256).
func Decode(key, text string) (string, error) {
	priv, err := ParseRSAPrivateKeyFromBase64(key)
	if err != nil {
		return "", fmt.Errorf("load private key: %w", err)
	}

	encrypted, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", fmt.Errorf("decode Base64 ciphertext: %w", err)
	}

	hash := sha256.New()
	plainBytes, err := rsa.DecryptOAEP(hash, rand.Reader, priv, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt with RSA-OAEP: %w", err)
	}
	return string(plainBytes), nil
}
