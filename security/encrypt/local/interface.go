// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"
	"crypto/rsa"

	"github.com/PointerByte/QuicksGo/security/encrypt/common"
	"github.com/PointerByte/QuicksGo/security/encrypt/models"
)

//go:generate mockgen -source=interface.go -destination=mock_repository.go -package=encrypt

// SymmetricRepository exposes symmetric encryption helpers.
type SymmetricRepository interface {
	// GeneratesSymetrycKey returns a random Base64-encoded symmetric key.
	GeneratesSymetrycKey(ctx context.Context, size common.SizeSymetrycKey) (*models.SymmetricKeyData, error)

	// EncryptAES encrypts plaintext using a Base64-encoded AES key and optional
	// additional authenticated data.
	EncryptAES(ctx context.Context, secretKey, value string, additional *string) (string, error)
	// DecryptAES decrypts Base64 ciphertext produced by EncryptAES.
	DecryptAES(ctx context.Context, secretKey, cipherValue, additionalData string) (string, error)
}

// AsymmetricRepository exposes RSA key generation and RSA-OAEP helpers.
type AsymmetricRepository interface {
	// GeneratesRSAKey creates an RSA key pair encoded as Base64.
	GeneratesRSAKey(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error)
	// RSA_OAEP_Encode encrypts plaintext with a Base64-encoded RSA public key.
	RSA_OAEP_Encode(ctx context.Context, publicKey, text string) (string, error)
	// RSA_OAEP_Decode decrypts Base64 ciphertext with a Base64-encoded RSA
	// private key.
	RSA_OAEP_Decode(ctx context.Context, privateKey, cipherText string) (string, error)
}

// HashRepository exposes hashing and message-authentication helpers.
type HashRepository interface {
	// GenerateHMAC returns a Base64-encoded HMAC-SHA256 signature.
	GenerateHMAC(ctx context.Context, message, secretKey string) string
	// ValidateHMAC checks whether providedHash matches the message HMAC.
	ValidateHMAC(ctx context.Context, message, secretKey, providedHash string) bool
	// Sha256Hex returns the SHA-256 digest as a hexadecimal string.
	Sha256Hex(ctx context.Context, message string) string
	// Blake3 returns the BLAKE3 digest encoded as Base64.
	Blake3(ctx context.Context, message string) string
}

// SignatureRepository exposes asymmetric signing and verification helpers.
type SignatureRepository interface {
	// GeneratesEd255Key creates an Ed25519 key pair encoded as Base64.
	GeneratesEd255Key(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error)
	// SignEd25519 signs text using a Base64-encoded Ed25519 private key.
	SignEd25519(ctx context.Context, privateKey, text string) (string, error)
	// VerifyEd25519 validates an Ed25519 Base64 signature.
	VerifyEd25519(ctx context.Context, publicKey, text, signature string) error

	// SignRSAPSS signs text with RSA-PSS using a Base64-encoded private key.
	SignRSAPSS(ctx context.Context, privateKey, text string) (string, error)
	// VerifyRSAPSS validates an RSA-PSS Base64 signature.
	VerifyRSAPSS(ctx context.Context, publicKey, text, signature string) error
	// SignSHA256 signs data with RSA PKCS#1 v1.5 using SHA-256.
	SignSHA256(ctx context.Context, data string, privateKey *rsa.PrivateKey) (string, error)
	// VerifySHA256 validates an RSA PKCS#1 v1.5 SHA-256 signature.
	VerifySHA256(ctx context.Context, data, signature string, publicKey *rsa.PublicKey) error
}

// Repository groups the main encryption and signature capabilities exposed by
// the package.
type Repository interface {
	SymmetricRepository
	AsymmetricRepository
	SignatureRepository
}
