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
	GeneratesSymetrycKey(ctx context.Context, size common.SizeSymetrycKey) (*models.SymmetricKeyData, error)
	EncryptAES(ctx context.Context, secretKey, value string, additional *string) (string, error)
	DecryptAES(ctx context.Context, secretKey, cipherValue string, additional *string) (string, error)
}

type AsymmetricRepository interface {
	GeneratesRSAKey(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error)
	RSA_OAEP_Encode(ctx context.Context, publicKey, text string) (string, error)
	RSA_OAEP_Decode(ctx context.Context, privateKey, cipherText string) (string, error)
}

type HashRepository interface {
	GenerateHMAC(ctx context.Context, secretKey, message string) string
	ValidateHMAC(ctx context.Context, secretKey, message, providedHash string) bool
	Sha256Hex(ctx context.Context, message string) string
	Blake3(ctx context.Context, message string) string
}

type SignatureRepository interface {
	GeneratesEd255Key(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error)
	SignEd25519(ctx context.Context, privateKey, text string) (string, error)
	VerifyEd25519(ctx context.Context, publicKey, text, signature string) error
	SignRSAPSS(ctx context.Context, privateKey, text string) (string, error)
	VerifyRSAPSS(ctx context.Context, publicKey, text, signature string) error
	SignPKCS1v15_SHA256(ctx context.Context, data string, privateKey *rsa.PrivateKey) (string, error)
	VerifySHA256(ctx context.Context, data, signature string, publicKey *rsa.PublicKey) error
}
