// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package signs

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
)

// ParseEd25519PublicKeyFromBase64 converts a Base64 X.509 DER Ed25519 public
// key into ed25519.PublicKey.
func ParseEd25519PublicKeyFromBase64(b64 string) (ed25519.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode public key from Base64: %w", err)
	}

	publicKeyAny, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	publicKey, ok := publicKeyAny.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key is not an Ed25519 key")
	}
	return publicKey, nil
}

// ParseEd25519PrivateKeyFromBase64 converts a Base64 PKCS#8 DER Ed25519
// private key into ed25519.PrivateKey.
func ParseEd25519PrivateKeyFromBase64(b64 string) (ed25519.PrivateKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode private key from Base64: %w", err)
	}

	privateKeyAny, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	privateKey, ok := privateKeyAny.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an Ed25519 key")
	}
	return privateKey, nil
}

// SignEd25519 signs text with Ed25519 and returns the Base64 signature.
func SignEd25519(key, text string) (string, error) {
	privateKey, err := ParseEd25519PrivateKeyFromBase64(key)
	if err != nil {
		return "", fmt.Errorf("load private key: %w", err)
	}

	signature := ed25519.Sign(privateKey, []byte(text))
	return base64.StdEncoding.EncodeToString(signature), nil
}

// VerifyEd25519 validates a Base64 Ed25519 signature.
func VerifyEd25519(key, text, signature string) error {
	publicKey, err := ParseEd25519PublicKeyFromBase64(key)
	if err != nil {
		return fmt.Errorf("load public key: %w", err)
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("decode signature from Base64: %w", err)
	}

	if !ed25519.Verify(publicKey, []byte(text), signatureBytes) {
		return errors.New("invalid Ed25519 signature")
	}
	return nil
}
