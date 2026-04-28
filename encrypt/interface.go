// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package encrypt

import (
	"context"
	"crypto/rsa"

	"github.com/PointerByte/QuicksGo/encrypt/common"
	"github.com/PointerByte/QuicksGo/encrypt/models"
)

//go:generate mockgen -source=interface.go -destination=mock_repository.go -package=encrypt

// SymmetricRepository exposes symmetric encryption helpers.
type SymmetricRepository interface {
	// GenerateSymetrycKeys returns a random Base64-encoded symmetric key.
	GenerateSymetrycKeys(ctx context.Context, size common.SizeSymetrycKey) (*models.KeyData, error)

	// EncryptAES encrypts plaintext using a Base64-encoded AES key and optional
	// additional authenticated data, returning the ciphertext in Base64.
	EncryptAES(ctx context.Context, secretKey, value string, additional *string) (string, error)
	// DecryptAES decrypts Base64 ciphertext produced by EncryptAES using the
	// same Base64 AES key and optional additional authenticated data.
	DecryptAES(ctx context.Context, secretKey, cipherValue string, additional *string) (string, error)
}

// AsymmetricRepository exposes RSA key generation and RSA-OAEP helpers.
type AsymmetricRepository interface {
	// GenerateRSAKeys creates an RSA key pair and returns the encoded key
	// material plus provider metadata.
	GenerateRSAKeys(ctx context.Context, size common.SizeAsymetrycKey) (*models.KeyData, error)
	// GenerateECCKeys creates an ECC key pair on the requested curve and returns
	// the encoded key material plus provider metadata.
	GenerateECCKeys(ctx context.Context, curve common.CurveAsymmetricKey) (*models.KeyData, error)
	// RSA_OAEP_Encode encrypts plaintext with a Base64-encoded RSA public key
	// and returns the ciphertext in Base64.
	RSA_OAEP_Encode(ctx context.Context, publicKey, text string) (string, error)
	// RSA_OAEP_Decode decrypts Base64 ciphertext with a Base64-encoded RSA
	// private key.
	RSA_OAEP_Decode(ctx context.Context, privateKey, cipherText string) (string, error)
	// ECC_Encode encrypts plaintext using an ECC public key with an ECDH-derived
	// AES-GCM key and returns an encoded payload.
	ECC_Encode(ctx context.Context, publicKey, text string) (string, error)
	// ECC_Decode decrypts ciphertext produced by ECC_Encode using the matching
	// ECC private key.
	ECC_Decode(ctx context.Context, privateKey, cipherText string) (string, error)
}

// HashRepository exposes hashing and message-authentication helpers.
type HashRepository interface {
	// HMAC returns a Base64-encoded HMAC-SHA256 signature.
	HMAC(ctx context.Context, secretKey, message string) string
	// Sha256Hex returns the SHA-256 digest as a hexadecimal string.
	Sha256Hex(ctx context.Context, message string) string
	// Blake3 returns the BLAKE3 digest encoded as Base64.
	Blake3(ctx context.Context, message string) string
}

// SignatureRepository exposes asymmetric signing and verification helpers.
type SignatureRepository interface {
	// GenerateEd255Keys creates an Ed25519 key pair and returns the encoded key
	// material plus provider metadata.
	GenerateEd255Keys(ctx context.Context, size common.SizeAsymetrycKey) (*models.KeyData, error)
	// SignEd25519 signs text using a Base64-encoded Ed25519 private key and
	// returns the signature in Base64.
	SignEd25519(ctx context.Context, privateKey, text string) (string, error)
	// VerifyEd25519 validates an Ed25519 Base64 signature.
	VerifyEd25519(ctx context.Context, publicKey, text, signature string) error

	// SignRSAPSS signs text with RSA-PSS using a Base64-encoded private key and
	// returns the signature in Base64.
	SignRSAPSS(ctx context.Context, privateKey, text string) (string, error)
	// VerifyRSAPSS validates an RSA-PSS Base64 signature.
	VerifyRSAPSS(ctx context.Context, publicKey, text, signature string) error
	// SignPKCS1v15_SHA256 signs data with RSA PKCS#1 v1.5 using SHA-256.
	SignPKCS1v15_SHA256(ctx context.Context, data string, privateKey *rsa.PrivateKey) (string, error)
	// VerifySHA256 validates an RSA PKCS#1 v1.5 SHA-256 signature.
	VerifySHA256(ctx context.Context, data, signature string, publicKey *rsa.PublicKey) error
}

// Repository groups the main encryption and signature capabilities exposed by
// the package in a single composite contract.
type IRepository interface {
	SymmetricRepository
	AsymmetricRepository
	HashRepository
	SignatureRepository
}
