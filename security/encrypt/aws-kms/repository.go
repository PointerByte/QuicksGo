// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package awskms

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/PointerByte/QuicksGo/security/encrypt/common"
	"github.com/PointerByte/QuicksGo/security/encrypt/local"
	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	kms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

const defaultKMSARNKey = "encrypt.vault.aws-kms.arn"

var (
	errAWSKMSKeyARNRequired     = errors.New("aws-kms: key arn or id is required")
	errAWSKMSEd25519Unsupported = errors.New("aws-kms: Ed25519 is not supported by this package")
	loadDefaultAWSConfigFn      = awsconfig.LoadDefaultConfig
	appendOTelMiddlewaresFn     = otelaws.AppendMiddlewares
	loadAWSConfigFn             = loadAWSConfig
	newKMSClientFn              = func(cfg sdkaws.Config) kmsClient {
		return kms.NewFromConfig(cfg)
	}
)

type kmsClient interface {
	CreateKey(ctx context.Context, params *kms.CreateKeyInput, optFns ...func(*kms.Options)) (*kms.CreateKeyOutput, error)
	GetPublicKey(ctx context.Context, params *kms.GetPublicKeyInput, optFns ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error)
	Encrypt(ctx context.Context, params *kms.EncryptInput, optFns ...func(*kms.Options)) (*kms.EncryptOutput, error)
	Decrypt(ctx context.Context, params *kms.DecryptInput, optFns ...func(*kms.Options)) (*kms.DecryptOutput, error)
	Sign(ctx context.Context, params *kms.SignInput, optFns ...func(*kms.Options)) (*kms.SignOutput, error)
	Verify(ctx context.Context, params *kms.VerifyInput, optFns ...func(*kms.Options)) (*kms.VerifyOutput, error)
}

type symmetricRepository struct{ local local.SymmetricRepository }
type hashRepository struct{ local local.HashRepository }

type asymmetricRepository struct {
	local local.AsymmetricRepository
}

type signatureRepository struct {
	local local.SignatureRepository
}

type repository struct {
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
	return &asymmetricRepository{
		local: local.NewAsymmetricRepository(),
	}
}

func NewSignatureRepository() SignatureRepository {
	return &signatureRepository{
		local: local.NewSignatureRepository(),
	}
}

func NewRepository() *repository {
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

func (repository *asymmetricRepository) GeneratesRSAKey(size common.SizeAsymetrycKey) (priv string, pub string, _ error) {
	client, err := newAWSKMSClient(context.Background())
	if err != nil {
		return "", "", err
	}

	keySpec, err := toAWSRSAKeySpec(size)
	if err != nil {
		return "", "", err
	}

	output, err := client.CreateKey(context.Background(), &kms.CreateKeyInput{
		KeyUsage: types.KeyUsageTypeEncryptDecrypt,
		KeySpec:  keySpec,
	})
	if err != nil {
		return "", "", fmt.Errorf("aws-kms: create rsa key: %w", err)
	}
	if output.KeyMetadata == nil || output.KeyMetadata.KeyId == nil || output.KeyMetadata.Arn == nil {
		return "", "", errors.New("aws-kms: missing key metadata from create key response")
	}

	viper.Set(defaultKMSARNKey, sdkaws.ToString(output.KeyMetadata.Arn))

	publicKeyOutput, err := client.GetPublicKey(context.Background(), &kms.GetPublicKeyInput{
		KeyId: output.KeyMetadata.KeyId,
	})
	if err != nil {
		return "", "", fmt.Errorf("aws-kms: get public key: %w", err)
	}

	return "", base64.StdEncoding.EncodeToString(publicKeyOutput.PublicKey), nil
}

func (repository *asymmetricRepository) RSA_OAEP_Encode(key, text string) (string, error) {
	if _, err := ParseRSAPublicKeyFromBase64(key); err == nil {
		return repository.local.RSA_OAEP_Encode(key, text)
	}

	client, err := newAWSKMSClient(context.Background())
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(key)
	if err != nil {
		return "", err
	}

	output, err := client.Encrypt(context.Background(), &kms.EncryptInput{
		KeyId:               sdkaws.String(keyID),
		Plaintext:           []byte(text),
		EncryptionAlgorithm: types.EncryptionAlgorithmSpecRsaesOaepSha256,
	})
	if err != nil {
		return "", fmt.Errorf("aws-kms: encrypt with rsa-oaep-sha256: %w", err)
	}
	return base64.StdEncoding.EncodeToString(output.CiphertextBlob), nil
}

func (repository *asymmetricRepository) RSA_OAEP_Decode(key, text string) (string, error) {
	if _, err := ParseRSAPrivateKeyFromBase64(key); err == nil {
		return repository.local.RSA_OAEP_Decode(key, text)
	}

	client, err := newAWSKMSClient(context.Background())
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(key)
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", fmt.Errorf("aws-kms: decode base64 ciphertext: %w", err)
	}

	output, err := client.Decrypt(context.Background(), &kms.DecryptInput{
		KeyId:               sdkaws.String(keyID),
		CiphertextBlob:      ciphertext,
		EncryptionAlgorithm: types.EncryptionAlgorithmSpecRsaesOaepSha256,
	})
	if err != nil {
		return "", fmt.Errorf("aws-kms: decrypt with rsa-oaep-sha256: %w", err)
	}
	return string(output.Plaintext), nil
}

func (repository *signatureRepository) GeneratesEd255Key(size common.SizeAsymetrycKey) (priv string, pub string, _ error) {
	_ = size
	return "", "", errAWSKMSEd25519Unsupported
}

func (repository *signatureRepository) SignEd25519(key, text string) (string, error) {
	_, _ = key, text
	return "", errAWSKMSEd25519Unsupported
}

func (repository *signatureRepository) VerifyEd25519(key, text, signature string) error {
	_, _, _ = key, text, signature
	return errAWSKMSEd25519Unsupported
}

func (repository *signatureRepository) SignRSAPSS(key, text string) (string, error) {
	if _, err := ParseRSAPrivateKeyFromBase64(key); err == nil {
		return repository.local.SignRSAPSS(key, text)
	}

	client, err := newAWSKMSClient(context.Background())
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(key)
	if err != nil {
		return "", err
	}

	output, err := client.Sign(context.Background(), &kms.SignInput{
		KeyId:            sdkaws.String(keyID),
		Message:          []byte(text),
		MessageType:      types.MessageTypeRaw,
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPssSha256,
	})
	if err != nil {
		return "", fmt.Errorf("aws-kms: sign rsa-pss-sha256: %w", err)
	}
	return base64.StdEncoding.EncodeToString(output.Signature), nil
}

func (repository *signatureRepository) VerifyRSAPSS(key, text, signature string) error {
	if _, err := ParseRSAPublicKeyFromBase64(key); err == nil {
		return repository.local.VerifyRSAPSS(key, text, signature)
	}

	client, err := newAWSKMSClient(context.Background())
	if err != nil {
		return err
	}

	keyID, err := resolveAWSKMSKeyID(key)
	if err != nil {
		return err
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("aws-kms: decode signature from base64: %w", err)
	}

	output, err := client.Verify(context.Background(), &kms.VerifyInput{
		KeyId:            sdkaws.String(keyID),
		Message:          []byte(text),
		MessageType:      types.MessageTypeRaw,
		Signature:        signatureBytes,
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPssSha256,
	})
	if err != nil {
		return fmt.Errorf("aws-kms: verify rsa-pss-sha256: %w", err)
	}
	if !output.SignatureValid {
		return errors.New("aws-kms: invalid RSA-PSS signature")
	}
	return nil
}

func (repository *signatureRepository) SignSHA256(data string, privateKey *rsa.PrivateKey) (string, error) {
	if privateKey != nil {
		return repository.local.SignSHA256(data, privateKey)
	}

	client, err := newAWSKMSClient(context.Background())
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID("")
	if err != nil {
		return "", err
	}

	output, err := client.Sign(context.Background(), &kms.SignInput{
		KeyId:            sdkaws.String(keyID),
		Message:          []byte(data),
		MessageType:      types.MessageTypeRaw,
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	})
	if err != nil {
		return "", fmt.Errorf("aws-kms: sign rsa-pkcs1v15-sha256: %w", err)
	}
	return base64.StdEncoding.EncodeToString(output.Signature), nil
}

func (repository *signatureRepository) VerifySHA256(data, signature string, publicKey *rsa.PublicKey) error {
	if publicKey != nil {
		return repository.local.VerifySHA256(data, signature, publicKey)
	}

	client, err := newAWSKMSClient(context.Background())
	if err != nil {
		return err
	}

	keyID, err := resolveAWSKMSKeyID("")
	if err != nil {
		return err
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("aws-kms: decode signature from base64: %w", err)
	}

	output, err := client.Verify(context.Background(), &kms.VerifyInput{
		KeyId:            sdkaws.String(keyID),
		Message:          []byte(data),
		MessageType:      types.MessageTypeRaw,
		Signature:        signatureBytes,
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	})
	if err != nil {
		return fmt.Errorf("aws-kms: verify rsa-pkcs1v15-sha256: %w", err)
	}
	if !output.SignatureValid {
		return errors.New("aws-kms: invalid RSA SHA-256 signature")
	}
	return nil
}

func newAWSKMSClient(ctx context.Context) (kmsClient, error) {
	cfg, err := loadAWSConfigFn(ctx)
	if err != nil {
		return nil, fmt.Errorf("aws-kms: load aws config: %w", err)
	}
	return newKMSClientFn(cfg), nil
}

func loadAWSConfig(ctx context.Context) (sdkaws.Config, error) {
	cfg, err := loadDefaultAWSConfigFn(ctx)
	if err != nil {
		return cfg, err
	}
	appendOTelMiddlewaresFn(&cfg.APIOptions)
	return cfg, nil
}

func resolveAWSKMSKeyID(key string) (string, error) {
	if trimmed := strings.TrimSpace(key); trimmed != "" {
		return trimmed, nil
	}
	if configured := strings.TrimSpace(viper.GetString(defaultKMSARNKey)); configured != "" {
		return configured, nil
	}
	return "", errAWSKMSKeyARNRequired
}

func toAWSRSAKeySpec(size common.SizeAsymetrycKey) (types.KeySpec, error) {
	switch size {
	case common.Key2048Bits:
		return types.KeySpecRsa2048, nil
	case common.Key3072Bits:
		return types.KeySpecRsa3072, nil
	case common.Key4096Bits:
		return types.KeySpecRsa4096, nil
	default:
		return "", fmt.Errorf("aws-kms: unsupported rsa key size: %d", size)
	}
}
