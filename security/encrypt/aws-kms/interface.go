// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package awskms

import (
	"context"
	"crypto/rsa"

	"github.com/PointerByte/QuicksGo/security/encrypt/common"
	"github.com/PointerByte/QuicksGo/security/encrypt/models"
)

type SymmetricRepository interface {
	GeneratesSymetrycKey(ctx context.Context, size common.SizeSymetrycKey) (*models.SymmetricKeyData, error)
	EncryptAES(ctx context.Context, secretKey, value string, additional *string) (string, error)
	DecryptAES(ctx context.Context, secretKey, cipherValue string, additional *string) (string, error)
}

type AsymmetricRepository interface {
	// GeneratesRSAKey creates an RSA key pair using AWS KMS when possible.
	// AWS KMS never exports the private key, so the private-key return value is
	// always empty and the generated key ARN is stored in viper under
	// "encrypt.aws-kms.arn".
	GeneratesRSAKey(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error)
	// RSA_OAEP_Encode encrypts plaintext with a KMS key id/ARN or a Base64 RSA
	// public key.
	RSA_OAEP_Encode(ctx context.Context, publicKey, text string) (string, error)
	// RSA_OAEP_Decode decrypts Base64 ciphertext with a KMS key id/ARN.
	RSA_OAEP_Decode(ctx context.Context, privateKey, cipherText string) (string, error)
}

type HashRepository interface {
	GenerateHMAC(ctx context.Context, secretKey, message string) string
	ValidateHMAC(ctx context.Context, secretKey, message, providedHash string) bool
	Sha256Hex(ctx context.Context, message string) string
	Blake3(ctx context.Context, message string) string
}

type SignatureRepository interface {
	// GeneratesEd255Key creates an Ed25519 signing key in AWS KMS when possible.
	GeneratesEd255Key(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error)
	SignEd25519(ctx context.Context, privateKey, text string) (string, error)
	VerifyEd25519(ctx context.Context, publicKey, text, signature string) error
	SignRSAPSS(ctx context.Context, privateKey, text string) (string, error)
	VerifyRSAPSS(ctx context.Context, publicKey, text, signature string) error
	// SignPKCS1v15_SHA256 signs data with RSA PKCS#1 v1.5. When privateKey is nil, the
	// repository uses the configured AWS KMS ARN from viper.
	SignPKCS1v15_SHA256(ctx context.Context, data string, privateKey *rsa.PrivateKey) (string, error)
	// VerifySHA256 verifies an RSA PKCS#1 v1.5 SHA-256 signature. When publicKey
	// is nil, the repository uses the configured AWS KMS ARN from viper.
	VerifySHA256(ctx context.Context, data, signature string, publicKey *rsa.PublicKey) error
}

type Repository interface {
	SymmetricRepository
	AsymmetricRepository
	SignatureRepository
	HashRepository
}
