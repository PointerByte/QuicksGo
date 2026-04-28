// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package awskms

import (
	"context"
	"github.com/PointerByte/QuicksGo/encrypt/common"
	"github.com/PointerByte/QuicksGo/encrypt/models"
)

type SymmetricRepository interface {
	// GenerateSymetrycKeys creates a symmetric key managed by AWS KMS and
	// returns its metadata reference.
	GenerateSymetrycKeys(ctx context.Context, size common.SizeSymetrycKey) (*models.KeyData, error)
	// EncryptAES encrypts plaintext with an AWS KMS symmetric key reference or
	// falls back to local AES-GCM when secretKey is a Base64 AES key.
	EncryptAES(ctx context.Context, secretKey, value string, additional *string) (string, error)
	// DecryptAES decrypts ciphertext produced by EncryptAES using AWS KMS or a
	// local Base64 AES key.
	DecryptAES(ctx context.Context, secretKey, cipherValue string, additional *string) (string, error)
}

type AsymmetricRepository interface {
	// GenerateRSAKeys creates an RSA key pair using AWS KMS when possible.
	// AWS KMS never exports the private key, so the private-key return value is
	// always empty and the generated key ARN is stored in viper under
	// "encrypt.aws-kms.arn".
	GenerateRSAKeys(ctx context.Context, size common.SizeAsymetrycKey) (*models.KeyData, error)
	// GenerateECCKeys creates an AWS KMS key-agreement key on the requested NIST
	// curve and returns its public key and metadata.
	GenerateECCKeys(ctx context.Context, curve common.CurveAsymmetricKey) (*models.KeyData, error)
	// RSA_OAEP_Encode encrypts plaintext with a KMS key id/ARN or a Base64 RSA
	// public key, using local RSA-OAEP as fallback for exported keys.
	RSA_OAEP_Encode(ctx context.Context, publicKey, text string) (string, error)
	// RSA_OAEP_Decode decrypts Base64 ciphertext with a KMS key id/ARN or a
	// Base64 RSA private key.
	RSA_OAEP_Decode(ctx context.Context, privateKey, cipherText string) (string, error)
	// ECC_Encode encrypts plaintext using a local ECC public key or an AWS KMS
	// key-agreement key, deriving an AES-GCM key through ECDH.
	ECC_Encode(ctx context.Context, publicKey, text string) (string, error)
	// ECC_Decode decrypts ciphertext produced by ECC_Encode using a local ECC
	// private key or an AWS KMS key-agreement key reference.
	ECC_Decode(ctx context.Context, privateKey, cipherText string) (string, error)
}

type HashRepository interface {
	// HMAC generates an HMAC-SHA256 value with AWS KMS when secretKey is
	// a KMS reference, or locally otherwise.
	HMAC(ctx context.Context, secretKey, message string) string
	// Sha256Hex returns the SHA-256 digest encoded as hexadecimal.
	Sha256Hex(ctx context.Context, message string) string
	// Blake3 returns the BLAKE3 digest encoded as Base64.
	Blake3(ctx context.Context, message string) string
}

type SignatureRepository interface {
	// GenerateEd255Keys creates an Ed25519 signing key in AWS KMS when possible.
	GenerateEd255Keys(ctx context.Context) (*models.KeyData, error)
	// SignEd25519 signs text with an AWS KMS Ed25519 key reference or a Base64
	// Ed25519 private key.
	SignEd25519(ctx context.Context, privateKey, text string) (string, error)
	// VerifyEd25519 verifies a Base64 Ed25519 signature with AWS KMS or a
	// Base64 Ed25519 public key.
	VerifyEd25519(ctx context.Context, publicKey, text, signature string) error
	// SignRSAPSS signs text with an AWS KMS RSA signing key reference or a
	// Base64 RSA private key.
	SignRSAPSS(ctx context.Context, privateKey, text string) (string, error)
	// VerifyRSAPSS verifies a Base64 RSA-PSS signature with AWS KMS or a Base64
	// RSA public key.
	VerifyRSAPSS(ctx context.Context, publicKey, text, signature string) error
	// Sign_RSA_PKCS1v15_SHA256 signs data with RSA PKCS#1 v1.5. When privateKey is empty, the
	// repository uses the configured AWS KMS ARN from viper.
	Sign_RSA_PKCS1v15_SHA256(ctx context.Context, privateKey, data string) (string, error)
	// Verify_RSA_PKCS1v15_SHA256 verifies an RSA PKCS#1 v1.5 SHA-256 signature. When publicKey
	// is empty, the repository uses the configured AWS KMS ARN from viper.
	Verify_RSA_PKCS1v15_SHA256(ctx context.Context, data, publicKey string, signature string) error
}
