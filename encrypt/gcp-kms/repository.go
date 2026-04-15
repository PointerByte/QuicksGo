// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package gcpkms

import (
	"context"
	"crypto"
	"crypto/aes"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	kms "cloud.google.com/go/kms/apiv1"
	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/PointerByte/QuicksGo/encrypt/common"
	"github.com/PointerByte/QuicksGo/encrypt/local"
	"github.com/PointerByte/QuicksGo/encrypt/models"
	"github.com/spf13/viper"
)

const (
	defaultGCPKeyIDKey     = "encrypt.vault.gcp-kms.key-id"
	legacyGCPKeyIDKey      = "encrypt.gcp-kms.key-id"
	gcpProviderName        = "gcp-kms"
	gcpSymmetricKeyPrefix  = "quicksgo-symmetric"
	gcpAsymmetricKeyPrefix = "quicksgo-rsa"
	gcpEd25519KeyPrefix    = "quicksgo-ed25519"
)

var (
	errGCPKMSKeyIDRequired   = errors.New("gcp-kms: key id is required")
	errGCPKMSKeyRingRequired = errors.New("gcp-kms: key ring path is required")
	errGCPKMSVersionRequired = errors.New("gcp-kms: crypto key version is required")
	newGCPKMSClientFn        = kms.NewKeyManagementClient
	newGCPClientFn           = func(ctx context.Context) (gcpKMSClient, error) {
		client, err := newGCPKMSClientFn(ctx)
		if err != nil {
			return nil, fmt.Errorf("gcp-kms: create client: %w", err)
		}
		return &gcpClientAdapter{KeyManagementClient: client}, nil
	}
)

type gcpKMSClient interface {
	CreateCryptoKey(ctx context.Context, req *kmspb.CreateCryptoKeyRequest) (*kmspb.CryptoKey, error)
	CreateCryptoKeyVersion(ctx context.Context, req *kmspb.CreateCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error)
	GetPublicKey(ctx context.Context, req *kmspb.GetPublicKeyRequest) (*kmspb.PublicKey, error)
	Encrypt(ctx context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error)
	Decrypt(ctx context.Context, req *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error)
	AsymmetricSign(ctx context.Context, req *kmspb.AsymmetricSignRequest) (*kmspb.AsymmetricSignResponse, error)
	AsymmetricDecrypt(ctx context.Context, req *kmspb.AsymmetricDecryptRequest) (*kmspb.AsymmetricDecryptResponse, error)
	MacSign(ctx context.Context, req *kmspb.MacSignRequest) (*kmspb.MacSignResponse, error)
	MacVerify(ctx context.Context, req *kmspb.MacVerifyRequest) (*kmspb.MacVerifyResponse, error)
	Close() error
}

type gcpClientAdapter struct{ *kms.KeyManagementClient }

func (adapter *gcpClientAdapter) CreateCryptoKey(ctx context.Context, req *kmspb.CreateCryptoKeyRequest) (*kmspb.CryptoKey, error) {
	return adapter.KeyManagementClient.CreateCryptoKey(ctx, req)
}
func (adapter *gcpClientAdapter) CreateCryptoKeyVersion(ctx context.Context, req *kmspb.CreateCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error) {
	return adapter.KeyManagementClient.CreateCryptoKeyVersion(ctx, req)
}
func (adapter *gcpClientAdapter) GetPublicKey(ctx context.Context, req *kmspb.GetPublicKeyRequest) (*kmspb.PublicKey, error) {
	return adapter.KeyManagementClient.GetPublicKey(ctx, req)
}
func (adapter *gcpClientAdapter) Encrypt(ctx context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) {
	return adapter.KeyManagementClient.Encrypt(ctx, req)
}
func (adapter *gcpClientAdapter) Decrypt(ctx context.Context, req *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) {
	return adapter.KeyManagementClient.Decrypt(ctx, req)
}
func (adapter *gcpClientAdapter) AsymmetricSign(ctx context.Context, req *kmspb.AsymmetricSignRequest) (*kmspb.AsymmetricSignResponse, error) {
	return adapter.KeyManagementClient.AsymmetricSign(ctx, req)
}
func (adapter *gcpClientAdapter) AsymmetricDecrypt(ctx context.Context, req *kmspb.AsymmetricDecryptRequest) (*kmspb.AsymmetricDecryptResponse, error) {
	return adapter.KeyManagementClient.AsymmetricDecrypt(ctx, req)
}
func (adapter *gcpClientAdapter) MacSign(ctx context.Context, req *kmspb.MacSignRequest) (*kmspb.MacSignResponse, error) {
	return adapter.KeyManagementClient.MacSign(ctx, req)
}
func (adapter *gcpClientAdapter) MacVerify(ctx context.Context, req *kmspb.MacVerifyRequest) (*kmspb.MacVerifyResponse, error) {
	return adapter.KeyManagementClient.MacVerify(ctx, req)
}

type symmetricRepository struct{ local local.SymmetricRepository }
type hashRepository struct{ local local.HashRepository }
type asymmetricRepository struct{ local local.AsymmetricRepository }
type signatureRepository struct{ local local.SignatureRepository }

type Repository struct {
	SymmetricRepository
	AsymmetricRepository
	SignatureRepository
	HashRepository
}

func NewSymmetricRepository() SymmetricRepository {
	return &symmetricRepository{local: local.NewSymmetricRepository()}
}

func NewHashRepository() HashRepository {
	return &hashRepository{local: local.NewHashRepository()}
}

func NewAsymmetricRepository() AsymmetricRepository {
	return &asymmetricRepository{local: local.NewAsymmetricRepository()}
}

func NewSignatureRepository() SignatureRepository {
	return &signatureRepository{local: local.NewSignatureRepository()}
}

func NewRepository() *Repository {
	return &Repository{
		SymmetricRepository:  NewSymmetricRepository(),
		AsymmetricRepository: NewAsymmetricRepository(),
		SignatureRepository:  NewSignatureRepository(),
		HashRepository:       NewHashRepository(),
	}
}

func (repository *symmetricRepository) GeneratesSymetrycKey(ctx context.Context, size common.SizeSymetrycKey) (*models.SymmetricKeyData, error) {
	if size != common.Key256Bits {
		return nil, fmt.Errorf("gcp-kms: unsupported symmetric key size: %d", size)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	parent, err := resolveGCPKeyRingName("")
	if err != nil {
		return nil, err
	}

	keyID := fmt.Sprintf("%s-%d", gcpSymmetricKeyPrefix, time.Now().UnixNano())
	cryptoKey, err := client.CreateCryptoKey(ctx, &kmspb.CreateCryptoKeyRequest{
		Parent:      parent,
		CryptoKeyId: keyID,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: kmspb.CryptoKey_ENCRYPT_DECRYPT,
			VersionTemplate: &kmspb.CryptoKeyVersionTemplate{
				Algorithm: kmspb.CryptoKeyVersion_GOOGLE_SYMMETRIC_ENCRYPTION,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("gcp-kms: create symmetric key: %w", err)
	}
	return &models.SymmetricKeyData{KeyID: keyID, KeyRef: cryptoKey.GetName(), Provider: gcpProviderName}, nil
}

func (repository *symmetricRepository) EncryptAES(ctx context.Context, secretKey, value string, additional *string) (string, error) {
	if isLocalAESKey(secretKey) {
		return repository.local.EncryptAES(ctx, secretKey, value, additional)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	keyName, err := resolveGCPCryptoKeyName(secretKey)
	if err != nil {
		return "", err
	}
	response, err := client.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:                        keyName,
		Plaintext:                   []byte(value),
		AdditionalAuthenticatedData: bytesFromOptionalString(additional),
	})
	if err != nil {
		return "", fmt.Errorf("gcp-kms: encrypt with symmetric key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(response.Ciphertext), nil
}

func (repository *symmetricRepository) DecryptAES(ctx context.Context, secretKey, cipherValue string, additional *string) (string, error) {
	if isLocalAESKey(secretKey) {
		return repository.local.DecryptAES(ctx, secretKey, cipherValue, additional)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	keyName, err := resolveGCPCryptoKeyName(secretKey)
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(cipherValue)
	if err != nil {
		return "", fmt.Errorf("gcp-kms: decode ciphertext: %w", err)
	}
	response, err := client.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:                        keyName,
		Ciphertext:                  ciphertext,
		AdditionalAuthenticatedData: bytesFromOptionalString(additional),
	})
	if err != nil {
		return "", fmt.Errorf("gcp-kms: decrypt with symmetric key: %w", err)
	}
	return string(response.Plaintext), nil
}

func (repository *hashRepository) GenerateHMAC(ctx context.Context, secretKey, message string) string {
	if !looksLikeGCPKMSKeyReference(secretKey) {
		return repository.local.GenerateHMAC(ctx, secretKey, message)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return ""
	}
	defer client.Close()

	versionName, err := resolveGCPCryptoKeyVersionName(secretKey)
	if err != nil {
		return ""
	}
	response, err := client.MacSign(ctx, &kmspb.MacSignRequest{Name: versionName, Data: []byte(message)})
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(response.Mac)
}

func (repository *hashRepository) ValidateHMAC(ctx context.Context, secretKey, message, providedHash string) bool {
	if !looksLikeGCPKMSKeyReference(secretKey) {
		return repository.local.ValidateHMAC(ctx, secretKey, message, providedHash)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return false
	}
	defer client.Close()

	versionName, err := resolveGCPCryptoKeyVersionName(secretKey)
	if err != nil {
		return false
	}
	macBytes, err := base64.StdEncoding.DecodeString(providedHash)
	if err != nil {
		return false
	}
	response, err := client.MacVerify(ctx, &kmspb.MacVerifyRequest{Name: versionName, Data: []byte(message), Mac: macBytes})
	if err != nil {
		return false
	}
	return response.Success
}

func (repository *hashRepository) Sha256Hex(ctx context.Context, message string) string {
	return repository.local.Sha256Hex(ctx, message)
}

func (repository *hashRepository) Blake3(ctx context.Context, message string) string {
	return repository.local.Blake3(ctx, message)
}

func (repository *asymmetricRepository) GeneratesRSAKey(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error) {
	algorithm, err := gcpRSADecryptAlgorithm(size)
	if err != nil {
		return nil, err
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	parent, err := resolveGCPKeyRingName("")
	if err != nil {
		return nil, err
	}
	keyID := fmt.Sprintf("%s-%d", gcpAsymmetricKeyPrefix, time.Now().UnixNano())
	cryptoKey, err := client.CreateCryptoKey(ctx, &kmspb.CreateCryptoKeyRequest{
		Parent:      parent,
		CryptoKeyId: keyID,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: kmspb.CryptoKey_ASYMMETRIC_DECRYPT,
			VersionTemplate: &kmspb.CryptoKeyVersionTemplate{
				Algorithm: algorithm,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("gcp-kms: create rsa key: %w", err)
	}

	versionName, err := ensureGCPVersion(ctx, client, cryptoKey.GetName(), algorithm, cryptoKey.GetPrimary())
	if err != nil {
		return nil, err
	}
	publicKey, err := fetchGCPPublicKey(ctx, client, versionName)
	if err != nil {
		return nil, err
	}

	return &models.AsymmetricKeyData{
		PublicKey: base64.StdEncoding.EncodeToString(publicKey),
		KeyID:     keyID,
		KeyRef:    versionName,
		Provider:  gcpProviderName,
	}, nil
}

func (repository *asymmetricRepository) RSA_OAEP_Encode(ctx context.Context, publicKey, text string) (string, error) {
	if _, err := ParseRSAPublicKeyFromBase64(publicKey); err == nil {
		return repository.local.RSA_OAEP_Encode(ctx, publicKey, text)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	versionName, err := resolveGCPCryptoKeyVersionName(publicKey)
	if err != nil {
		return "", err
	}
	publicKeyDER, err := fetchGCPPublicKey(ctx, client, versionName)
	if err != nil {
		return "", err
	}
	return repository.local.RSA_OAEP_Encode(ctx, base64.StdEncoding.EncodeToString(publicKeyDER), text)
}

func (repository *asymmetricRepository) RSA_OAEP_Decode(ctx context.Context, privateKey, cipherText string) (string, error) {
	if _, err := ParseRSAPrivateKeyFromBase64(privateKey); err == nil {
		return repository.local.RSA_OAEP_Decode(ctx, privateKey, cipherText)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	versionName, err := resolveGCPCryptoKeyVersionName(privateKey)
	if err != nil {
		return "", err
	}
	cipherBytes, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("gcp-kms: decode ciphertext: %w", err)
	}
	response, err := client.AsymmetricDecrypt(ctx, &kmspb.AsymmetricDecryptRequest{Name: versionName, Ciphertext: cipherBytes})
	if err != nil {
		return "", fmt.Errorf("gcp-kms: asymmetric decrypt: %w", err)
	}
	return string(response.Plaintext), nil
}

func (repository *signatureRepository) GeneratesEd255Key(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error) {
	_ = size

	client, err := newGCPClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	parent, err := resolveGCPKeyRingName("")
	if err != nil {
		return nil, err
	}
	keyID := fmt.Sprintf("%s-%d", gcpEd25519KeyPrefix, time.Now().UnixNano())
	cryptoKey, err := client.CreateCryptoKey(ctx, &kmspb.CreateCryptoKeyRequest{
		Parent:      parent,
		CryptoKeyId: keyID,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: kmspb.CryptoKey_ASYMMETRIC_SIGN,
			VersionTemplate: &kmspb.CryptoKeyVersionTemplate{
				Algorithm: kmspb.CryptoKeyVersion_EC_SIGN_ED25519,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("gcp-kms: create ed25519 key: %w", err)
	}

	versionName, err := ensureGCPVersion(ctx, client, cryptoKey.GetName(), kmspb.CryptoKeyVersion_EC_SIGN_ED25519, cryptoKey.GetPrimary())
	if err != nil {
		return nil, err
	}
	publicKey, err := fetchGCPPublicKey(ctx, client, versionName)
	if err != nil {
		return nil, err
	}

	return &models.AsymmetricKeyData{
		PublicKey: base64.StdEncoding.EncodeToString(publicKey),
		KeyID:     keyID,
		KeyRef:    versionName,
		Provider:  gcpProviderName,
	}, nil
}

func (repository *signatureRepository) SignEd25519(ctx context.Context, privateKey, text string) (string, error) {
	if _, err := ParseEd25519PrivateKeyFromBase64(privateKey); err == nil {
		return repository.local.SignEd25519(ctx, privateKey, text)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	versionName, err := resolveGCPCryptoKeyVersionName(privateKey)
	if err != nil {
		return "", err
	}
	response, err := client.AsymmetricSign(ctx, &kmspb.AsymmetricSignRequest{Name: versionName, Data: []byte(text)})
	if err != nil {
		return "", fmt.Errorf("gcp-kms: sign ed25519: %w", err)
	}
	return base64.StdEncoding.EncodeToString(response.Signature), nil
}

func (repository *signatureRepository) VerifyEd25519(ctx context.Context, publicKey, text, signature string) error {
	if _, err := ParseEd25519PublicKeyFromBase64(publicKey); err == nil {
		return repository.local.VerifyEd25519(ctx, publicKey, text, signature)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	versionName, err := resolveGCPCryptoKeyVersionName(publicKey)
	if err != nil {
		return err
	}
	publicKeyDER, err := fetchGCPPublicKey(ctx, client, versionName)
	if err != nil {
		return err
	}
	keyAny, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return fmt.Errorf("gcp-kms: parse public key: %w", err)
	}
	edPublicKey, ok := keyAny.(ed25519.PublicKey)
	if !ok {
		return errors.New("gcp-kms: public key is not an Ed25519 key")
	}
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("gcp-kms: decode signature from base64: %w", err)
	}
	if !ed25519.Verify(edPublicKey, []byte(text), signatureBytes) {
		return errors.New("gcp-kms: invalid Ed25519 signature")
	}
	return nil
}

func (repository *signatureRepository) SignRSAPSS(ctx context.Context, privateKey, text string) (string, error) {
	if _, err := ParseRSAPrivateKeyFromBase64(privateKey); err == nil {
		return repository.local.SignRSAPSS(ctx, privateKey, text)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	versionName, err := resolveGCPCryptoKeyVersionName(privateKey)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(text))
	response, err := client.AsymmetricSign(ctx, &kmspb.AsymmetricSignRequest{
		Name:   versionName,
		Digest: &kmspb.Digest{Digest: &kmspb.Digest_Sha256{Sha256: digest[:]}},
	})
	if err != nil {
		return "", fmt.Errorf("gcp-kms: sign rsa-pss-sha256: %w", err)
	}
	return base64.StdEncoding.EncodeToString(response.Signature), nil
}

func (repository *signatureRepository) VerifyRSAPSS(ctx context.Context, publicKey, text, signature string) error {
	if _, err := ParseRSAPublicKeyFromBase64(publicKey); err == nil {
		return repository.local.VerifyRSAPSS(ctx, publicKey, text, signature)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	versionName, err := resolveGCPCryptoKeyVersionName(publicKey)
	if err != nil {
		return err
	}
	publicKeyDER, err := fetchGCPPublicKey(ctx, client, versionName)
	if err != nil {
		return err
	}
	keyAny, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return fmt.Errorf("gcp-kms: parse public key: %w", err)
	}
	rsaPublicKey, ok := keyAny.(*rsa.PublicKey)
	if !ok {
		return errors.New("gcp-kms: public key is not an RSA key")
	}
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("gcp-kms: decode signature from base64: %w", err)
	}
	digest := sha256.Sum256([]byte(text))
	if err := rsa.VerifyPSS(rsaPublicKey, crypto.SHA256, digest[:], signatureBytes, nil); err != nil {
		return fmt.Errorf("gcp-kms: invalid RSA-PSS signature: %w", err)
	}
	return nil
}

func (repository *signatureRepository) SignPKCS1v15_SHA256(ctx context.Context, data string, privateKey *rsa.PrivateKey) (string, error) {
	if privateKey != nil {
		return repository.local.SignPKCS1v15_SHA256(ctx, data, privateKey)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	versionName, err := resolveGCPCryptoKeyVersionName("")
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(data))
	response, err := client.AsymmetricSign(ctx, &kmspb.AsymmetricSignRequest{
		Name:   versionName,
		Digest: &kmspb.Digest{Digest: &kmspb.Digest_Sha256{Sha256: digest[:]}},
	})
	if err != nil {
		return "", fmt.Errorf("gcp-kms: sign rsa-sha256: %w", err)
	}
	return base64.StdEncoding.EncodeToString(response.Signature), nil
}

func (repository *signatureRepository) VerifySHA256(ctx context.Context, data, signature string, publicKey *rsa.PublicKey) error {
	if publicKey != nil {
		return repository.local.VerifySHA256(ctx, data, signature, publicKey)
	}

	client, err := newGCPClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	versionName, err := resolveGCPCryptoKeyVersionName("")
	if err != nil {
		return err
	}
	publicKeyDER, err := fetchGCPPublicKey(ctx, client, versionName)
	if err != nil {
		return err
	}
	keyAny, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return fmt.Errorf("gcp-kms: parse public key: %w", err)
	}
	rsaPublicKey, ok := keyAny.(*rsa.PublicKey)
	if !ok {
		return errors.New("gcp-kms: public key is not an RSA key")
	}
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("gcp-kms: decode signature from base64: %w", err)
	}
	digest := sha256.Sum256([]byte(data))
	if err := rsa.VerifyPKCS1v15(rsaPublicKey, crypto.SHA256, digest[:], signatureBytes); err != nil {
		return fmt.Errorf("gcp-kms: invalid RSA SHA-256 signature: %w", err)
	}
	return nil
}

func newGCPClient(ctx context.Context) (gcpKMSClient, error) {
	return newGCPClientFn(ctx)
}

func configuredGCPKeyID() string {
	if configured := strings.TrimSpace(viper.GetString(defaultGCPKeyIDKey)); configured != "" {
		return configured
	}
	return strings.TrimSpace(viper.GetString(legacyGCPKeyIDKey))
}

func resolveGCPKeyRingName(key string) (string, error) {
	rawKey := strings.TrimSpace(key)
	if rawKey == "" {
		rawKey = configuredGCPKeyID()
	}
	if rawKey == "" {
		return "", errGCPKMSKeyIDRequired
	}

	segments := strings.Split(strings.Trim(rawKey, "/"), "/")
	for i := range segments {
		if segments[i] == "cryptoKeys" && i >= 5 {
			return strings.Join(segments[:i], "/"), nil
		}
	}
	return "", errGCPKMSKeyRingRequired
}

func resolveGCPCryptoKeyName(key string) (string, error) {
	rawKey := strings.TrimSpace(key)
	if rawKey == "" {
		rawKey = configuredGCPKeyID()
	}
	if rawKey == "" {
		return "", errGCPKMSKeyIDRequired
	}
	if index := strings.Index(rawKey, "/cryptoKeyVersions/"); index >= 0 {
		return rawKey[:index], nil
	}
	if strings.Contains(rawKey, "/cryptoKeys/") {
		return rawKey, nil
	}
	return "", errGCPKMSKeyIDRequired
}

func resolveGCPCryptoKeyVersionName(key string) (string, error) {
	rawKey := strings.TrimSpace(key)
	if rawKey == "" {
		rawKey = configuredGCPKeyID()
	}
	if rawKey == "" {
		return "", errGCPKMSKeyIDRequired
	}
	if strings.Contains(rawKey, "/cryptoKeyVersions/") {
		return rawKey, nil
	}
	return "", errGCPKMSVersionRequired
}

func ensureGCPVersion(ctx context.Context, client gcpKMSClient, cryptoKeyName string, algorithm kmspb.CryptoKeyVersion_CryptoKeyVersionAlgorithm, primary *kmspb.CryptoKeyVersion) (string, error) {
	if primary != nil && primary.GetName() != "" {
		return primary.GetName(), nil
	}
	version, err := client.CreateCryptoKeyVersion(ctx, &kmspb.CreateCryptoKeyVersionRequest{
		Parent: cryptoKeyName,
		CryptoKeyVersion: &kmspb.CryptoKeyVersion{
			Algorithm: algorithm,
		},
	})
	if err != nil {
		return "", fmt.Errorf("gcp-kms: create crypto key version: %w", err)
	}
	if version.GetName() == "" {
		return "", errors.New("gcp-kms: missing crypto key version metadata")
	}
	return version.GetName(), nil
}

func fetchGCPPublicKey(ctx context.Context, client gcpKMSClient, versionName string) ([]byte, error) {
	response, err := client.GetPublicKey(ctx, &kmspb.GetPublicKeyRequest{Name: versionName})
	if err != nil {
		return nil, fmt.Errorf("gcp-kms: get public key: %w", err)
	}
	block, _ := pem.Decode([]byte(response.Pem))
	if block == nil {
		return nil, errors.New("gcp-kms: invalid PEM public key")
	}
	return block.Bytes, nil
}

func gcpRSADecryptAlgorithm(size common.SizeAsymetrycKey) (kmspb.CryptoKeyVersion_CryptoKeyVersionAlgorithm, error) {
	switch size {
	case common.Key2048Bits:
		return kmspb.CryptoKeyVersion_RSA_DECRYPT_OAEP_2048_SHA256, nil
	case common.Key3072Bits:
		return kmspb.CryptoKeyVersion_RSA_DECRYPT_OAEP_3072_SHA256, nil
	case common.Key4096Bits:
		return kmspb.CryptoKeyVersion_RSA_DECRYPT_OAEP_4096_SHA256, nil
	default:
		return 0, fmt.Errorf("gcp-kms: unsupported rsa key size: %d", size)
	}
}

func isLocalAESKey(key string) bool {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return false
	}
	_, err = aes.NewCipher(decoded)
	return err == nil
}

func looksLikeGCPKMSKeyReference(key string) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return configuredGCPKeyID() != ""
	}
	return strings.HasPrefix(trimmed, "projects/")
}

func bytesFromOptionalString(value *string) []byte {
	if value == nil {
		return nil
	}
	return []byte(*value)
}
