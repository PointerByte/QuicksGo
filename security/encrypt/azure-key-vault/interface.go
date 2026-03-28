// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package azurekeyvault

import (
	"crypto/rsa"

	"github.com/PointerByte/QuicksGo/security/encrypt/common"
)

type SymmetricRepository interface {
	GeneratesSymetrycKey(size common.SizeSymetrycKey) (string, error)
	EncryptAES(symmetricalAccess, valorCampo, additionalData string) (string, error)
	DecryptAES(symmetricalAccess, valorCifrado, additionalData string) (string, error)
	EncodeFernet(keyString, originalString string) (string, error)
	DecodeFernet(keyString, encryptedString string) (string, error)
}

type AsymmetricRepository interface {
	GeneratesRSAKey(size common.SizeAsymetrycKey) (priv string, pub string, _ error)
	RSA_OAEP_Encode(key, text string) (string, error)
	RSA_OAEP_Decode(key, text string) (string, error)
}

type HashRepository interface {
	GenerateHMAC(message, secretKey string) string
	ValidateHMAC(message, secretKey, providedHash string) bool
	Sha256Hex(message string) string
	Blake3(message string) string
}

type SignatureRepository interface {
	GeneratesEd255Key(size common.SizeAsymetrycKey) (priv string, pub string, _ error)
	SignEd25519(key, text string) (string, error)
	VerifyEd25519(key, text, signature string) error
	SignRSAPSS(key, text string) (string, error)
	VerifyRSAPSS(key, text, signature string) error
	SignSHA256(data string, privateKey *rsa.PrivateKey) (string, error)
	VerifySHA256(data, signature string, publicKey *rsa.PublicKey) error
}

type Repository interface {
	SymmetricRepository
	AsymmetricRepository
	SignatureRepository
	HashRepository
}
