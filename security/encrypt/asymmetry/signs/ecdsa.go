// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package signs

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"

	rsautil "github.com/PointerByte/QuicksGo/security/encrypt/asymmetry/rsa"
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

// SignRSAPSS is kept as an alias for compatibility.
func SignRSAPSS(key, text string) (string, error) {
	priv, err := rsautil.ParseRSAPrivateKeyFromBase64(key)
	if err != nil {
		return "", fmt.Errorf("load private key: %w", err)
	}

	hashed := sha256.Sum256([]byte(text))
	signature, err := rsa.SignPSS(rand.Reader, priv, crypto.SHA256, hashed[:], nil)
	if err != nil {
		return "", fmt.Errorf("sign with RSA-PSS: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// VerifyRSAPSS is kept as an alias for compatibility.
func VerifyRSAPSS(key, text, signature string) error {
	pub, err := rsautil.ParseRSAPublicKeyFromBase64(key)
	if err != nil {
		return fmt.Errorf("load public key: %w", err)
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("decode signature from Base64: %w", err)
	}

	hashed := sha256.Sum256([]byte(text))
	if err := rsa.VerifyPSS(pub, crypto.SHA256, hashed[:], signatureBytes, nil); err != nil {
		return fmt.Errorf("invalid RSA-PSS signature: %w", err)
	}
	return nil
}

// SignSHA256 signs bytes using RSA PKCS#1 v1.5 with SHA-256.
func SignSHA256(key []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	if privateKey == nil {
		return nil, errors.New("private key is required")
	}

	hashed := sha256.Sum256(key)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return nil, fmt.Errorf("sign with RSA SHA-256: %w", err)
	}
	return signature, nil
}

// VerifySHA256 validates an RSA PKCS#1 v1.5 signature with SHA-256.
func VerifySHA256(key, signature []byte, publicKey *rsa.PublicKey) error {
	if publicKey == nil {
		return errors.New("public key is required")
	}

	hashed := sha256.Sum256(key)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], signature); err != nil {
		return fmt.Errorf("invalid RSA SHA-256 signature: %w", err)
	}
	return nil
}
