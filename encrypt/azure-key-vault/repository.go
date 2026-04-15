// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package azurekeyvault

import (
	"context"
	"crypto/aes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/PointerByte/QuicksGo/encrypt/common"
	"github.com/PointerByte/QuicksGo/encrypt/local"
	"github.com/PointerByte/QuicksGo/encrypt/models"
	"github.com/spf13/viper"
)

const (
	defaultAzureKeyIDKey     = "encrypt.vault.azure-key-vault.key-id"
	legacyAzureKeyIDKey      = "encrypt.azure-key-vault.key-id"
	defaultAzureVaultURLKey  = "encrypt.vault.azure-key-vault.vault-url"
	legacyAzureVaultURLKey   = "encrypt.azure-key-vault.vault-url"
	azureProviderName        = "azure-key-vault"
	azureSymmetricKeyPrefix  = "quicksgo-symmetric"
	azureAsymmetricKeyPrefix = "quicksgo-rsa"
)

var (
	errAzureKeyIDRequired      = errors.New("azure-key-vault: key id is required")
	errAzureVaultURLRequired   = errors.New("azure-key-vault: vault url is required")
	errAzureEd25519Unsupported = errors.New("azure-key-vault: Ed25519 provider-backed operations are not supported")
	newAzureCredentialFn       = func(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return azidentity.NewDefaultAzureCredential(options)
	}
	newAzureClientFn = func(vaultURL string, credential azcore.TokenCredential) (azureKeysClient, error) {
		return azkeys.NewClient(vaultURL, credential, nil)
	}
)

type azureKeysClient interface {
	CreateKey(ctx context.Context, name string, parameters azkeys.CreateKeyParameters, options *azkeys.CreateKeyOptions) (azkeys.CreateKeyResponse, error)
	Encrypt(ctx context.Context, name string, version string, parameters azkeys.KeyOperationParameters, options *azkeys.EncryptOptions) (azkeys.EncryptResponse, error)
	Decrypt(ctx context.Context, name string, version string, parameters azkeys.KeyOperationParameters, options *azkeys.DecryptOptions) (azkeys.DecryptResponse, error)
	Sign(ctx context.Context, name string, version string, parameters azkeys.SignParameters, options *azkeys.SignOptions) (azkeys.SignResponse, error)
	Verify(ctx context.Context, name string, version string, parameters azkeys.VerifyParameters, options *azkeys.VerifyOptions) (azkeys.VerifyResponse, error)
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

type azureAEADPayload struct {
	Result string `json:"result"`
	IV     string `json:"iv,omitempty"`
	Tag    string `json:"tag,omitempty"`
}

type azureKeyReference struct {
	VaultURL string
	Name     string
	Version  string
	KID      string
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
		return nil, fmt.Errorf("azure-key-vault: unsupported symmetric key size: %d", size)
	}

	client, vaultURL, err := newAzureKeysClient(ctx, "")
	if err != nil {
		return nil, err
	}

	keyName := fmt.Sprintf("%s-%d", azureSymmetricKeyPrefix, time.Now().UnixNano())
	response, err := client.CreateKey(ctx, keyName, azkeys.CreateKeyParameters{
		Kty: ptr(azkeys.KeyTypeOctHSM),
		KeyOps: []*azkeys.KeyOperation{
			ptr(azkeys.KeyOperationEncrypt),
			ptr(azkeys.KeyOperationDecrypt),
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("azure-key-vault: create symmetric key: %w", err)
	}

	keyID, keyRef, err := azureMetadataFromBundle(response.KeyBundle, vaultURL, keyName)
	if err != nil {
		return nil, err
	}
	return &models.SymmetricKeyData{KeyID: keyID, KeyRef: keyRef, Provider: azureProviderName}, nil
}

func (repository *symmetricRepository) EncryptAES(ctx context.Context, secretKey, value string, additional *string) (string, error) {
	if isLocalAESKey(secretKey) {
		return repository.local.EncryptAES(ctx, secretKey, value, additional)
	}

	reference, err := resolveAzureKeyReference(secretKey)
	if err != nil {
		return "", err
	}
	client, _, err := newAzureKeysClient(ctx, reference.VaultURL)
	if err != nil {
		return "", err
	}

	response, err := client.Encrypt(ctx, reference.Name, reference.Version, azkeys.KeyOperationParameters{
		Algorithm:                   ptr(azkeys.EncryptionAlgorithmA256GCM),
		Value:                       []byte(value),
		AdditionalAuthenticatedData: bytesFromOptionalString(additional),
	}, nil)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: encrypt with symmetric key: %w", err)
	}

	payloadBytes, err := json.Marshal(azureAEADPayload{
		Result: base64.StdEncoding.EncodeToString(response.Result),
		IV:     base64.StdEncoding.EncodeToString(response.IV),
		Tag:    base64.StdEncoding.EncodeToString(response.AuthenticationTag),
	})
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: encode ciphertext payload: %w", err)
	}
	return base64.StdEncoding.EncodeToString(payloadBytes), nil
}

func (repository *symmetricRepository) DecryptAES(ctx context.Context, secretKey, cipherValue string, additional *string) (string, error) {
	if isLocalAESKey(secretKey) {
		return repository.local.DecryptAES(ctx, secretKey, cipherValue, additional)
	}

	reference, err := resolveAzureKeyReference(secretKey)
	if err != nil {
		return "", err
	}
	client, _, err := newAzureKeysClient(ctx, reference.VaultURL)
	if err != nil {
		return "", err
	}

	payloadBytes, err := base64.StdEncoding.DecodeString(cipherValue)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: decode ciphertext payload: %w", err)
	}
	var payload azureAEADPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", fmt.Errorf("azure-key-vault: decode ciphertext json: %w", err)
	}

	resultBytes, err := base64.StdEncoding.DecodeString(payload.Result)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: decode ciphertext bytes: %w", err)
	}
	ivBytes, err := base64.StdEncoding.DecodeString(payload.IV)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: decode iv: %w", err)
	}
	tagBytes, err := base64.StdEncoding.DecodeString(payload.Tag)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: decode authentication tag: %w", err)
	}

	response, err := client.Decrypt(ctx, reference.Name, reference.Version, azkeys.KeyOperationParameters{
		Algorithm:                   ptr(azkeys.EncryptionAlgorithmA256GCM),
		Value:                       resultBytes,
		IV:                          ivBytes,
		AuthenticationTag:           tagBytes,
		AdditionalAuthenticatedData: bytesFromOptionalString(additional),
	}, nil)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: decrypt with symmetric key: %w", err)
	}
	return string(response.Result), nil
}

func (repository *hashRepository) GenerateHMAC(ctx context.Context, secretKey, message string) string {
	if !looksLikeAzureKeyReference(secretKey) {
		return repository.local.GenerateHMAC(ctx, secretKey, message)
	}

	reference, err := resolveAzureKeyReference(secretKey)
	if err != nil {
		return ""
	}
	client, _, err := newAzureKeysClient(ctx, reference.VaultURL)
	if err != nil {
		return ""
	}

	response, err := client.Sign(ctx, reference.Name, reference.Version, azkeys.SignParameters{
		Algorithm: ptr(azkeys.SignatureAlgorithmHS256),
		Value:     []byte(message),
	}, nil)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(response.Result)
}

func (repository *hashRepository) ValidateHMAC(ctx context.Context, secretKey, message, providedHash string) bool {
	if !looksLikeAzureKeyReference(secretKey) {
		return repository.local.ValidateHMAC(ctx, secretKey, message, providedHash)
	}

	reference, err := resolveAzureKeyReference(secretKey)
	if err != nil {
		return false
	}
	client, _, err := newAzureKeysClient(ctx, reference.VaultURL)
	if err != nil {
		return false
	}

	signature, err := base64.StdEncoding.DecodeString(providedHash)
	if err != nil {
		return false
	}
	response, err := client.Verify(ctx, reference.Name, reference.Version, azkeys.VerifyParameters{
		Algorithm: ptr(azkeys.SignatureAlgorithmHS256),
		Digest:    []byte(message),
		Signature: signature,
	}, nil)
	if err != nil {
		return false
	}
	return boolValue(response.Value)
}

func (repository *hashRepository) Sha256Hex(ctx context.Context, message string) string {
	return repository.local.Sha256Hex(ctx, message)
}

func (repository *hashRepository) Blake3(ctx context.Context, message string) string {
	return repository.local.Blake3(ctx, message)
}

func (repository *asymmetricRepository) GeneratesRSAKey(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error) {
	keySize, err := azureRSAKeySize(size)
	if err != nil {
		return nil, err
	}

	client, vaultURL, err := newAzureKeysClient(ctx, "")
	if err != nil {
		return nil, err
	}

	keyName := fmt.Sprintf("%s-%d", azureAsymmetricKeyPrefix, time.Now().UnixNano())
	response, err := client.CreateKey(ctx, keyName, azkeys.CreateKeyParameters{
		Kty:     ptr(azkeys.KeyTypeRSA),
		KeySize: ptr(int32(keySize)),
		KeyOps: []*azkeys.KeyOperation{
			ptr(azkeys.KeyOperationEncrypt),
			ptr(azkeys.KeyOperationDecrypt),
			ptr(azkeys.KeyOperationSign),
			ptr(azkeys.KeyOperationVerify),
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("azure-key-vault: create rsa key: %w", err)
	}

	publicKey, err := rsaPublicKeyFromAzureBundle(response.KeyBundle)
	if err != nil {
		return nil, err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("azure-key-vault: marshal public key: %w", err)
	}

	keyID, keyRef, err := azureMetadataFromBundle(response.KeyBundle, vaultURL, keyName)
	if err != nil {
		return nil, err
	}
	return &models.AsymmetricKeyData{
		PublicKey: base64.StdEncoding.EncodeToString(publicDER),
		KeyID:     keyID,
		KeyRef:    keyRef,
		Provider:  azureProviderName,
	}, nil
}

func (repository *asymmetricRepository) RSA_OAEP_Encode(ctx context.Context, publicKey, text string) (string, error) {
	if _, err := ParseRSAPublicKeyFromBase64(publicKey); err == nil {
		return repository.local.RSA_OAEP_Encode(ctx, publicKey, text)
	}

	reference, err := resolveAzureKeyReference(publicKey)
	if err != nil {
		return "", err
	}
	client, _, err := newAzureKeysClient(ctx, reference.VaultURL)
	if err != nil {
		return "", err
	}

	response, err := client.Encrypt(ctx, reference.Name, reference.Version, azkeys.KeyOperationParameters{
		Algorithm: ptr(azkeys.EncryptionAlgorithmRSAOAEP256),
		Value:     []byte(text),
	}, nil)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: encrypt with rsa-oaep-256: %w", err)
	}
	return base64.StdEncoding.EncodeToString(response.Result), nil
}

func (repository *asymmetricRepository) RSA_OAEP_Decode(ctx context.Context, privateKey, cipherText string) (string, error) {
	if _, err := ParseRSAPrivateKeyFromBase64(privateKey); err == nil {
		return repository.local.RSA_OAEP_Decode(ctx, privateKey, cipherText)
	}

	reference, err := resolveAzureKeyReference(privateKey)
	if err != nil {
		return "", err
	}
	client, _, err := newAzureKeysClient(ctx, reference.VaultURL)
	if err != nil {
		return "", err
	}

	cipherBytes, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: decode ciphertext: %w", err)
	}
	response, err := client.Decrypt(ctx, reference.Name, reference.Version, azkeys.KeyOperationParameters{
		Algorithm: ptr(azkeys.EncryptionAlgorithmRSAOAEP256),
		Value:     cipherBytes,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: decrypt with rsa-oaep-256: %w", err)
	}
	return string(response.Result), nil
}

func (repository *signatureRepository) GeneratesEd255Key(ctx context.Context, size common.SizeAsymetrycKey) (*models.AsymmetricKeyData, error) {
	_ = ctx
	_ = size
	return nil, errAzureEd25519Unsupported
}

func (repository *signatureRepository) SignEd25519(ctx context.Context, privateKey, text string) (string, error) {
	if _, err := ParseEd25519PrivateKeyFromBase64(privateKey); err == nil {
		return repository.local.SignEd25519(ctx, privateKey, text)
	}
	return "", errAzureEd25519Unsupported
}

func (repository *signatureRepository) VerifyEd25519(ctx context.Context, publicKey, text, signature string) error {
	if _, err := ParseEd25519PublicKeyFromBase64(publicKey); err == nil {
		return repository.local.VerifyEd25519(ctx, publicKey, text, signature)
	}
	return errAzureEd25519Unsupported
}

func (repository *signatureRepository) SignRSAPSS(ctx context.Context, privateKey, text string) (string, error) {
	if _, err := ParseRSAPrivateKeyFromBase64(privateKey); err == nil {
		return repository.local.SignRSAPSS(ctx, privateKey, text)
	}

	reference, err := resolveAzureKeyReference(privateKey)
	if err != nil {
		return "", err
	}
	client, _, err := newAzureKeysClient(ctx, reference.VaultURL)
	if err != nil {
		return "", err
	}

	digest := sha256.Sum256([]byte(text))
	response, err := client.Sign(ctx, reference.Name, reference.Version, azkeys.SignParameters{
		Algorithm: ptr(azkeys.SignatureAlgorithmPS256),
		Value:     digest[:],
	}, nil)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: sign rsa-pss-sha256: %w", err)
	}
	return base64.StdEncoding.EncodeToString(response.Result), nil
}

func (repository *signatureRepository) VerifyRSAPSS(ctx context.Context, publicKey, text, signature string) error {
	if _, err := ParseRSAPublicKeyFromBase64(publicKey); err == nil {
		return repository.local.VerifyRSAPSS(ctx, publicKey, text, signature)
	}

	reference, err := resolveAzureKeyReference(publicKey)
	if err != nil {
		return err
	}
	client, _, err := newAzureKeysClient(ctx, reference.VaultURL)
	if err != nil {
		return err
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("azure-key-vault: decode signature from base64: %w", err)
	}
	digest := sha256.Sum256([]byte(text))
	response, err := client.Verify(ctx, reference.Name, reference.Version, azkeys.VerifyParameters{
		Algorithm: ptr(azkeys.SignatureAlgorithmPS256),
		Digest:    digest[:],
		Signature: signatureBytes,
	}, nil)
	if err != nil {
		return fmt.Errorf("azure-key-vault: verify rsa-pss-sha256: %w", err)
	}
	if !boolValue(response.Value) {
		return errors.New("azure-key-vault: invalid RSA-PSS signature")
	}
	return nil
}

func (repository *signatureRepository) SignPKCS1v15_SHA256(ctx context.Context, data string, privateKey *rsa.PrivateKey) (string, error) {
	if privateKey != nil {
		return repository.local.SignPKCS1v15_SHA256(ctx, data, privateKey)
	}

	reference, err := resolveAzureKeyReference("")
	if err != nil {
		return "", err
	}
	client, _, err := newAzureKeysClient(ctx, reference.VaultURL)
	if err != nil {
		return "", err
	}

	digest := sha256.Sum256([]byte(data))
	response, err := client.Sign(ctx, reference.Name, reference.Version, azkeys.SignParameters{
		Algorithm: ptr(azkeys.SignatureAlgorithmRS256),
		Value:     digest[:],
	}, nil)
	if err != nil {
		return "", fmt.Errorf("azure-key-vault: sign rsa-sha256: %w", err)
	}
	return base64.StdEncoding.EncodeToString(response.Result), nil
}

func (repository *signatureRepository) VerifySHA256(ctx context.Context, data, signature string, publicKey *rsa.PublicKey) error {
	if publicKey != nil {
		return repository.local.VerifySHA256(ctx, data, signature, publicKey)
	}

	reference, err := resolveAzureKeyReference("")
	if err != nil {
		return err
	}
	client, _, err := newAzureKeysClient(ctx, reference.VaultURL)
	if err != nil {
		return err
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("azure-key-vault: decode signature from base64: %w", err)
	}
	digest := sha256.Sum256([]byte(data))
	response, err := client.Verify(ctx, reference.Name, reference.Version, azkeys.VerifyParameters{
		Algorithm: ptr(azkeys.SignatureAlgorithmRS256),
		Digest:    digest[:],
		Signature: signatureBytes,
	}, nil)
	if err != nil {
		return fmt.Errorf("azure-key-vault: verify rsa-sha256: %w", err)
	}
	if !boolValue(response.Value) {
		return errors.New("azure-key-vault: invalid RSA SHA-256 signature")
	}
	return nil
}

func newAzureKeysClient(_ context.Context, vaultURL string) (azureKeysClient, string, error) {
	resolvedVaultURL := strings.TrimSpace(vaultURL)
	if resolvedVaultURL == "" {
		var err error
		resolvedVaultURL, err = resolveAzureVaultURL()
		if err != nil {
			return nil, "", err
		}
	}

	credential, err := newAzureCredentialFn(nil)
	if err != nil {
		return nil, "", fmt.Errorf("azure-key-vault: create credential: %w", err)
	}
	client, err := newAzureClientFn(resolvedVaultURL, credential)
	if err != nil {
		return nil, "", fmt.Errorf("azure-key-vault: create client: %w", err)
	}
	return client, resolvedVaultURL, nil
}

func resolveAzureKeyReference(key string) (*azureKeyReference, error) {
	rawKey := strings.TrimSpace(key)
	if rawKey == "" {
		rawKey = configuredAzureKeyID()
	}
	if rawKey == "" {
		return nil, errAzureKeyIDRequired
	}

	parsed, err := url.Parse(rawKey)
	if err != nil {
		return nil, fmt.Errorf("azure-key-vault: parse key id: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("azure-key-vault: invalid key id %q", rawKey)
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 || segments[0] != "keys" {
		return nil, fmt.Errorf("azure-key-vault: invalid key path %q", parsed.Path)
	}

	reference := &azureKeyReference{
		VaultURL: (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String(),
		Name:     segments[1],
		KID:      rawKey,
	}
	if len(segments) > 2 {
		reference.Version = segments[2]
	}
	return reference, nil
}

func resolveAzureVaultURL() (string, error) {
	if configured := strings.TrimSpace(viper.GetString(defaultAzureVaultURLKey)); configured != "" {
		return configured, nil
	}
	if configured := strings.TrimSpace(viper.GetString(legacyAzureVaultURLKey)); configured != "" {
		return configured, nil
	}

	reference := configuredAzureKeyID()
	if reference == "" {
		return "", errAzureVaultURLRequired
	}
	parsed, err := resolveAzureKeyReference(reference)
	if err != nil {
		return "", err
	}
	return parsed.VaultURL, nil
}

func configuredAzureKeyID() string {
	if configured := strings.TrimSpace(viper.GetString(defaultAzureKeyIDKey)); configured != "" {
		return configured
	}
	return strings.TrimSpace(viper.GetString(legacyAzureKeyIDKey))
}

func azureRSAKeySize(size common.SizeAsymetrycKey) (int, error) {
	switch size {
	case common.Key2048Bits, common.Key3072Bits, common.Key4096Bits:
		return int(size), nil
	default:
		return 0, fmt.Errorf("azure-key-vault: unsupported rsa key size: %d", size)
	}
}

func azureMetadataFromBundle(bundle azkeys.KeyBundle, vaultURL, keyName string) (string, string, error) {
	if bundle.Key != nil && bundle.Key.KID != nil {
		keyRef := string(*bundle.Key.KID)
		return bundle.Key.KID.Name(), keyRef, nil
	}
	if strings.TrimSpace(vaultURL) == "" {
		return "", "", errors.New("azure-key-vault: missing key metadata from response")
	}
	keyRef := strings.TrimRight(vaultURL, "/") + "/keys/" + keyName
	return keyName, keyRef, nil
}

func rsaPublicKeyFromAzureBundle(bundle azkeys.KeyBundle) (*rsa.PublicKey, error) {
	if bundle.Key == nil || len(bundle.Key.N) == 0 || len(bundle.Key.E) == 0 {
		return nil, errors.New("azure-key-vault: missing rsa public key material")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(bundle.Key.N),
		E: int(new(big.Int).SetBytes(bundle.Key.E).Int64()),
	}, nil
}

func isLocalAESKey(key string) bool {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return false
	}
	_, err = aes.NewCipher(decoded)
	return err == nil
}

func looksLikeAzureKeyReference(key string) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return configuredAzureKeyID() != ""
	}
	return strings.HasPrefix(trimmed, "https://") && strings.Contains(trimmed, "/keys/")
}

func bytesFromOptionalString(value *string) []byte {
	if value == nil {
		return nil
	}
	return []byte(*value)
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func ptr[T any](value T) *T {
	return &value
}
