// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package awskms

import (
	"crypto/rsa"

	"github.com/PointerByte/QuicksGo/security/encrypt/common"
)

type SymmetricRepository interface {
	GeneratesSymetrycKey(size common.SizeSymetrycKey) (string, error)
	EncryptAES(symmetricalAccess, value, additionalData string) (string, error)
	DecryptAES(symmetricalAccess, cipherValue, additionalData string) (string, error)
	EncodeFernet(keyString, value string) (string, error)
	DecodeFernet(keyString, cipherValue string) (string, error)
}

type AsymmetricRepository interface {
	// GeneratesRSAKey creates an RSA key pair using AWS KMS when possible.
	// AWS KMS never exports the private key, so the private-key return value is
	// always empty and the generated key ARN is stored in viper under
	// "encrypt.aws-kms.arn".
	GeneratesRSAKey(size common.SizeAsymetrycKey) (priv string, pub string, _ error)
	// RSA_OAEP_Encode encrypts plaintext with a KMS key id/ARN or a Base64 RSA
	// public key.
	RSA_OAEP_Encode(key, text string) (string, error)
	// RSA_OAEP_Decode decrypts Base64 ciphertext with a KMS key id/ARN.
	RSA_OAEP_Decode(key, cipherText string) (string, error)
}

type HashRepository interface {
	GenerateHMAC(message, secretKey string) string
	ValidateHMAC(message, secretKey, providedHash string) bool
	Sha256Hex(message string) string
	Blake3(message string) string
}

type SignatureRepository interface {
	// GeneratesEd255Key returns empty values because AWS KMS does not expose
	// Ed25519 key generation in this package contract.
	GeneratesEd255Key(size common.SizeAsymetrycKey) (priv string, pub string, _ error)
	SignEd25519(key, text string) (string, error)
	VerifyEd25519(key, text, signature string) error
	SignRSAPSS(key, text string) (string, error)
	VerifyRSAPSS(key, text, signature string) error
	// SignSHA256 signs data with RSA PKCS#1 v1.5. When privateKey is nil, the
	// repository uses the configured AWS KMS ARN from viper.
	SignSHA256(data string, privateKey *rsa.PrivateKey) (string, error)
	// VerifySHA256 verifies an RSA PKCS#1 v1.5 SHA-256 signature. When publicKey
	// is nil, the repository uses the configured AWS KMS ARN from viper.
	VerifySHA256(data, signature string, publicKey *rsa.PublicKey) error
}

type Repository interface {
	SymmetricRepository
	AsymmetricRepository
	SignatureRepository
	HashRepository
}
