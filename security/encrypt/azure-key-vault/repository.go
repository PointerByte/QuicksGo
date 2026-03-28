// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package azurekeyvault

import (
	"crypto/rsa"
	"errors"
	"strings"

	"github.com/PointerByte/QuicksGo/security/encrypt/common"
	"github.com/PointerByte/QuicksGo/security/encrypt/local"
	"github.com/spf13/viper"
)

const defaultKeyIDKey = "encrypt.vault.azure-key-vault.key-id"

var (
	errAzureKeyIDRequired            = errors.New("azure-key-vault: key id is required")
	errAzureAsymmetricNotImplemented = errors.New("azure-key-vault: provider-backed asymmetric operations are not implemented yet")
	errAzureEd25519Unsupported       = errors.New("azure-key-vault: Ed25519 is not supported by this package")
)

type symmetricRepository struct{ local local.SymmetricRepository }
type hashRepository struct{ local local.HashRepository }
type asymmetricRepository struct{ local local.AsymmetricRepository }
type signatureRepository struct{ local local.SignatureRepository }

type repository struct {
	SymmetricRepository
	AsymmetricRepository
	SignatureRepository
	HashRepository
}

func NewSymmetricRepository() SymmetricRepository {
	return &symmetricRepository{local: local.NewSymmetricRepository()}
}
func NewHashRepository() HashRepository { return &hashRepository{local: local.NewHashRepository()} }
func NewAsymmetricRepository() AsymmetricRepository {
	return &asymmetricRepository{local: local.NewAsymmetricRepository()}
}
func NewSignatureRepository() SignatureRepository {
	return &signatureRepository{local: local.NewSignatureRepository()}
}

func NewRepository() Repository {
	return &repository{
		SymmetricRepository:  NewSymmetricRepository(),
		AsymmetricRepository: NewAsymmetricRepository(),
		SignatureRepository:  NewSignatureRepository(),
		HashRepository:       NewHashRepository(),
	}
}

func (repository *symmetricRepository) GeneratesSymetrycKey(size common.SizeSymetrycKey) (string, error) {
	return repository.local.GeneratesSymetrycKey(size)
}
func (repository *symmetricRepository) EncryptAES(symmetricalAccess, valorCampo, additionalData string) (string, error) {
	return repository.local.EncryptAES(symmetricalAccess, valorCampo, additionalData)
}
func (repository *symmetricRepository) DecryptAES(symmetricalAccess, valorCifrado, additionalData string) (string, error) {
	return repository.local.DecryptAES(symmetricalAccess, valorCifrado, additionalData)
}
func (repository *symmetricRepository) EncodeFernet(keyString, originalString string) (string, error) {
	return repository.local.EncodeFernet(keyString, originalString)
}
func (repository *symmetricRepository) DecodeFernet(keyString, encryptedString string) (string, error) {
	return repository.local.DecodeFernet(keyString, encryptedString)
}
func (repository *hashRepository) GenerateHMAC(message, secretKey string) string {
	return repository.local.GenerateHMAC(message, secretKey)
}
func (repository *hashRepository) ValidateHMAC(message, secretKey, providedHash string) bool {
	return repository.local.ValidateHMAC(message, secretKey, providedHash)
}
func (repository *hashRepository) Sha256Hex(message string) string {
	return repository.local.Sha256Hex(message)
}
func (repository *hashRepository) Blake3(message string) string {
	return repository.local.Blake3(message)
}

// GeneratesRSAKey returns empty values because Azure Key Vault private keys are
// provider-managed and this package does not yet provision them directly.
func (repository *asymmetricRepository) GeneratesRSAKey(size common.SizeAsymetrycKey) (priv string, pub string, _ error) {
	_ = size
	return "", "", errAzureAsymmetricNotImplemented
}

func (repository *asymmetricRepository) RSA_OAEP_Encode(key, text string) (string, error) {
	if _, err := ParseRSAPublicKeyFromBase64(key); err == nil {
		return repository.local.RSA_OAEP_Encode(key, text)
	}
	if _, err := resolveAzureKeyID(); err != nil {
		return "", err
	}
	return "", errAzureAsymmetricNotImplemented
}

func (repository *asymmetricRepository) RSA_OAEP_Decode(key, text string) (string, error) {
	if _, err := ParseRSAPrivateKeyFromBase64(key); err == nil {
		return repository.local.RSA_OAEP_Decode(key, text)
	}
	if _, err := resolveAzureKeyID(); err != nil {
		return "", err
	}
	return "", errAzureAsymmetricNotImplemented
}

func (repository *signatureRepository) GeneratesEd255Key(size common.SizeAsymetrycKey) (priv string, pub string, _ error) {
	_ = size
	return "", "", errAzureEd25519Unsupported
}
func (repository *signatureRepository) SignEd25519(key, text string) (string, error) {
	_, _ = key, text
	return "", errAzureEd25519Unsupported
}
func (repository *signatureRepository) VerifyEd25519(key, text, signature string) error {
	_, _, _ = key, text, signature
	return errAzureEd25519Unsupported
}
func (repository *signatureRepository) SignRSAPSS(key, text string) (string, error) {
	if _, err := ParseRSAPrivateKeyFromBase64(key); err == nil {
		return repository.local.SignRSAPSS(key, text)
	}
	if _, err := resolveAzureKeyID(); err != nil {
		return "", err
	}
	return "", errAzureAsymmetricNotImplemented
}
func (repository *signatureRepository) VerifyRSAPSS(key, text, signature string) error {
	if _, err := ParseRSAPublicKeyFromBase64(key); err == nil {
		return repository.local.VerifyRSAPSS(key, text, signature)
	}
	if _, err := resolveAzureKeyID(); err != nil {
		return err
	}
	return errAzureAsymmetricNotImplemented
}
func (repository *signatureRepository) SignSHA256(data string, privateKey *rsa.PrivateKey) (string, error) {
	if privateKey != nil {
		return repository.local.SignSHA256(data, privateKey)
	}
	if _, err := resolveAzureKeyID(); err != nil {
		return "", err
	}
	return "", errAzureAsymmetricNotImplemented
}
func (repository *signatureRepository) VerifySHA256(data, signature string, publicKey *rsa.PublicKey) error {
	if publicKey != nil {
		return repository.local.VerifySHA256(data, signature, publicKey)
	}
	if _, err := resolveAzureKeyID(); err != nil {
		return err
	}
	return errAzureAsymmetricNotImplemented
}

func resolveAzureKeyID() (string, error) {
	if configured := strings.TrimSpace(viper.GetString(defaultKeyIDKey)); configured != "" {
		return configured, nil
	}
	return "", errAzureKeyIDRequired
}
