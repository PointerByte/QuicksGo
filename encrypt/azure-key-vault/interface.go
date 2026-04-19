// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package azurekeyvault

import (
	"context"
	"crypto/rsa"

	"github.com/PointerByte/QuicksGo/encrypt/common"
	"github.com/PointerByte/QuicksGo/encrypt/models"
)

type SymmetricRepository interface {
	// GeneratesSymetrycKey creates an Azure Key Vault symmetric key and returns
	// its metadata reference.
	GeneratesSymetrycKey(ctx context.Context, size common.SizeSymetrycKey) (*models.SymmetricKeyData, error)
	// EncryptAES encrypts plaintext with an Azure Key Vault symmetric key
	// reference or falls back to local AES-GCM when secretKey is a Base64 AES
	// key.
	EncryptAES(ctx context.Context, secretKey, value string, additional *string) (string, error)
	// DecryptAES decrypts ciphertext produced by EncryptAES using Azure Key
	// Vault or a local Base64 AES key.
	DecryptAES(ctx context.Context, secretKey, cipherValue string, additional *string) (string, error)
}

type AsymmetricRepository interface {
	// GeneratesRSAKey creates an RSA key in Azure Key Vault and returns its
	// public key plus metadata reference.
	GeneratesRSAKey(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error)
	// GeneratesECCKey creates an ECC key pair when provider-backed support is
	// available for the backend.
	GeneratesECCKey(ctx context.Context, curve common.CurveAsymmetricKey) (*models.AsymmetricKeyData, error)
	// RSA_OAEP_Encode encrypts plaintext with an Azure Key Vault key reference
	// or a Base64 RSA public key.
	RSA_OAEP_Encode(ctx context.Context, publicKey, text string) (string, error)
	// RSA_OAEP_Decode decrypts ciphertext produced by RSA_OAEP_Encode using an
	// Azure Key Vault key reference or a Base64 RSA private key.
	RSA_OAEP_Decode(ctx context.Context, privateKey, cipherText string) (string, error)
	// ECC_Encode encrypts plaintext with a supported provider-backed ECC key or
	// falls back to a local Base64 ECC public key.
	ECC_Encode(ctx context.Context, publicKey, text string) (string, error)
	// ECC_Decode decrypts ciphertext produced by ECC_Encode using a supported
	// provider-backed ECC key or a local Base64 ECC private key.
	ECC_Decode(ctx context.Context, privateKey, cipherText string) (string, error)
}

type HashRepository interface {
	// GenerateHMAC generates an HMAC-SHA256 value with Azure Key Vault when
	// secretKey is a vault reference, or locally otherwise.
	GenerateHMAC(ctx context.Context, secretKey, message string) string
	// ValidateHMAC validates a provided HMAC-SHA256 value with Azure Key Vault
	// or locally, depending on the secretKey format.
	ValidateHMAC(ctx context.Context, secretKey, message, providedHash string) bool
	// Sha256Hex returns the SHA-256 digest encoded as hexadecimal.
	Sha256Hex(ctx context.Context, message string) string
	// Blake3 returns the BLAKE3 digest encoded as Base64.
	Blake3(ctx context.Context, message string) string
}

type SignatureRepository interface {
	// GeneratesEd255Key creates an Ed25519 signing key when provider-backed
	// support is available for the backend.
	GeneratesEd255Key(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error)
	// SignEd25519 signs text with a supported provider-backed key or a Base64
	// Ed25519 private key.
	SignEd25519(ctx context.Context, privateKey, text string) (string, error)
	// VerifyEd25519 verifies a Base64 Ed25519 signature with a supported
	// provider-backed key or a Base64 Ed25519 public key.
	VerifyEd25519(ctx context.Context, publicKey, text, signature string) error
	// SignRSAPSS signs text with an Azure Key Vault RSA signing key reference or
	// a Base64 RSA private key.
	SignRSAPSS(ctx context.Context, privateKey, text string) (string, error)
	// VerifyRSAPSS verifies a Base64 RSA-PSS signature with an Azure Key Vault
	// key reference or a Base64 RSA public key.
	VerifyRSAPSS(ctx context.Context, publicKey, text, signature string) error
	// SignPKCS1v15_SHA256 signs data with RSA PKCS#1 v1.5 using Azure Key Vault
	// when privateKey is nil, or a local RSA private key otherwise.
	SignPKCS1v15_SHA256(ctx context.Context, data string, privateKey *rsa.PrivateKey) (string, error)
	// VerifySHA256 verifies an RSA PKCS#1 v1.5 SHA-256 signature with Azure Key
	// Vault when publicKey is nil, or a local RSA public key otherwise.
	VerifySHA256(ctx context.Context, data, signature string, publicKey *rsa.PublicKey) error
}
