// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package signs

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	rsautil "github.com/PointerByte/QuicksGo/security/encrypt/asymmetry/rsa"
)

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
func VerifySHA256(key []byte, signature []byte, publicKey *rsa.PublicKey) error {
	if publicKey == nil {
		return errors.New("public key is required")
	}

	hashed := sha256.Sum256(key)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], signature); err != nil {
		return fmt.Errorf("invalid RSA SHA-256 signature: %w", err)
	}
	return nil
}
