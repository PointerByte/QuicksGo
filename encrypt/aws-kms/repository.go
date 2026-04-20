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

	"github.com/PointerByte/QuicksGo/encrypt/common"
	"github.com/PointerByte/QuicksGo/encrypt/local"
	"github.com/PointerByte/QuicksGo/encrypt/models"
	"github.com/PointerByte/QuicksGo/encrypt/utilities"
	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	kms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

const defaultKMSARNKey = "encrypt.vault.aws-kms.arn"

var (
	errAWSKMSKeyARNRequired = errors.New("aws-kms: key arn or id is required")
	loadDefaultAWSConfigFn  = awsconfig.LoadDefaultConfig
	appendOTelMiddlewaresFn = otelaws.AppendMiddlewares
	loadAWSConfigFn         = loadAWSConfig
	newKMSClientFn          = func(cfg sdkaws.Config) kmsClient {
		return kms.NewFromConfig(cfg)
	}
)

type kmsClient interface {
	CreateKey(ctx context.Context, params *kms.CreateKeyInput, optFns ...func(*kms.Options)) (*kms.CreateKeyOutput, error)
	DeriveSharedSecret(ctx context.Context, params *kms.DeriveSharedSecretInput, optFns ...func(*kms.Options)) (*kms.DeriveSharedSecretOutput, error)
	GetPublicKey(ctx context.Context, params *kms.GetPublicKeyInput, optFns ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error)
	Encrypt(ctx context.Context, params *kms.EncryptInput, optFns ...func(*kms.Options)) (*kms.EncryptOutput, error)
	Decrypt(ctx context.Context, params *kms.DecryptInput, optFns ...func(*kms.Options)) (*kms.DecryptOutput, error)
	GenerateMac(ctx context.Context, params *kms.GenerateMacInput, optFns ...func(*kms.Options)) (*kms.GenerateMacOutput, error)
	VerifyMac(ctx context.Context, params *kms.VerifyMacInput, optFns ...func(*kms.Options)) (*kms.VerifyMacOutput, error)
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
	return &asymmetricRepository{
		local: local.NewAsymmetricRepository(),
	}
}

func NewSignatureRepository() SignatureRepository {
	return &signatureRepository{
		local: local.NewSignatureRepository(),
	}
}

func NewRepository() *Repository {
	return &Repository{
		SymmetricRepository:  NewSymmetricRepository(),
		AsymmetricRepository: NewAsymmetricRepository(),
		SignatureRepository:  NewSignatureRepository(),
		HashRepository:       NewHashRepository(),
	}
}

func (repository *symmetricRepository) GenerateSymetrycKeys(ctx context.Context, size common.SizeSymetrycKey) (*models.KeyData, error) {
	keySpec, err := toAWSSymmetricKeySpec(size)
	if err != nil {
		return nil, err
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return nil, err
	}

	output, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		KeyUsage: types.KeyUsageTypeEncryptDecrypt,
		KeySpec:  keySpec,
		Origin:   types.OriginTypeAwsKms,
	})
	if err != nil {
		return nil, fmt.Errorf("aws-kms: create symmetric key: %w", err)
	}
	if output.KeyMetadata == nil || output.KeyMetadata.KeyId == nil || output.KeyMetadata.Arn == nil {
		return nil, errors.New("aws-kms: missing key metadata from create symmetric key response")
	}

	keyID := sdkaws.ToString(output.KeyMetadata.KeyId)
	keyRef := sdkaws.ToString(output.KeyMetadata.Arn)

	return &models.KeyData{
		KeyID:    keyID,
		KeyRef:   keyRef,
		Provider: "aws-kms",
	}, nil
}

func (repository *symmetricRepository) EncryptAES(ctx context.Context, secretKey, value string, additional *string) (string, error) {
	if utilities.IsLocalAESKey(secretKey) {
		return repository.local.EncryptAES(ctx, secretKey, value, additional)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(secretKey)
	if err != nil {
		return "", err
	}

	input := &kms.EncryptInput{
		KeyId:     sdkaws.String(keyID),
		Plaintext: []byte(value),
	}
	if additional != nil {
		input.EncryptionContext = map[string]string{"additional": *additional}
	}

	output, err := client.Encrypt(ctx, input)
	if err != nil {
		return "", fmt.Errorf("aws-kms: encrypt with symmetric key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(output.CiphertextBlob), nil
}

func (repository *symmetricRepository) DecryptAES(ctx context.Context, secretKey, cipherValue string, additional *string) (string, error) {
	if utilities.IsLocalAESKey(secretKey) {
		return repository.local.DecryptAES(ctx, secretKey, cipherValue, additional)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(secretKey)
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(cipherValue)
	if err != nil {
		return "", fmt.Errorf("aws-kms: decode base64 ciphertext: %w", err)
	}

	input := &kms.DecryptInput{
		KeyId:          sdkaws.String(keyID),
		CiphertextBlob: ciphertext,
	}
	if additional != nil {
		input.EncryptionContext = map[string]string{"additional": *additional}
	}

	output, err := client.Decrypt(ctx, input)
	if err != nil {
		return "", fmt.Errorf("aws-kms: decrypt with symmetric key: %w", err)
	}
	return string(output.Plaintext), nil
}

func (repository *hashRepository) GenerateHMAC(ctx context.Context, secretKey, message string) string {
	if !looksLikeAWSKMSKeyReference(secretKey) {
		return repository.local.GenerateHMAC(ctx, secretKey, message)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return ""
	}

	keyID, err := resolveAWSKMSKeyID(secretKey)
	if err != nil {
		return ""
	}

	output, err := client.GenerateMac(ctx, &kms.GenerateMacInput{
		KeyId:        sdkaws.String(keyID),
		Message:      []byte(message),
		MacAlgorithm: types.MacAlgorithmSpecHmacSha256,
	})
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(output.Mac)
}

func (repository *hashRepository) ValidateHMAC(ctx context.Context, secretKey, message, providedHash string) bool {
	if !looksLikeAWSKMSKeyReference(secretKey) {
		return repository.local.ValidateHMAC(ctx, secretKey, message, providedHash)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return false
	}

	keyID, err := resolveAWSKMSKeyID(secretKey)
	if err != nil {
		return false
	}

	mac, err := base64.StdEncoding.DecodeString(providedHash)
	if err != nil {
		return false
	}

	output, err := client.VerifyMac(ctx, &kms.VerifyMacInput{
		KeyId:        sdkaws.String(keyID),
		Mac:          mac,
		Message:      []byte(message),
		MacAlgorithm: types.MacAlgorithmSpecHmacSha256,
	})
	if err != nil {
		return false
	}
	return output.MacValid
}

func (repository *hashRepository) Sha256Hex(ctx context.Context, message string) string {
	return repository.local.Sha256Hex(ctx, message)
}

func (repository *hashRepository) Blake3(ctx context.Context, message string) string {
	return repository.local.Blake3(ctx, message)
}

func (repository *asymmetricRepository) GenerateRSAKeys(ctx context.Context, size common.SizeAsymetrycKey) (*models.KeyData, error) {
	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return nil, err
	}

	keySpec, err := toAWSRSAKeySpec(size)
	if err != nil {
		return nil, err
	}

	output, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		KeyUsage: types.KeyUsageTypeEncryptDecrypt,
		KeySpec:  keySpec,
	})
	if err != nil {
		return nil, fmt.Errorf("aws-kms: create rsa key: %w", err)
	}
	if output.KeyMetadata == nil || output.KeyMetadata.KeyId == nil || output.KeyMetadata.Arn == nil {
		return nil, errors.New("aws-kms: missing key metadata from create key response")
	}

	publicKeyOutput, err := client.GetPublicKey(ctx, &kms.GetPublicKeyInput{
		KeyId: output.KeyMetadata.KeyId,
	})
	if err != nil {
		return nil, fmt.Errorf("aws-kms: get public key: %w", err)
	}

	keyID := sdkaws.ToString(output.KeyMetadata.KeyId)
	keyRef := sdkaws.ToString(output.KeyMetadata.Arn)
	publicKey := base64.StdEncoding.EncodeToString(publicKeyOutput.PublicKey)
	return &models.KeyData{
		PublicKey: publicKey,
		KeyID:     keyID,
		KeyRef:    keyRef,
		Provider:  "aws-kms",
	}, nil
}

func (repository *asymmetricRepository) GenerateECCKeys(ctx context.Context, curve common.CurveAsymmetricKey) (*models.KeyData, error) {
	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return nil, err
	}

	keySpec, err := toAWSECCKeySpec(curve)
	if err != nil {
		return nil, err
	}

	output, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		KeyUsage: types.KeyUsageTypeKeyAgreement,
		KeySpec:  keySpec,
		Origin:   types.OriginTypeAwsKms,
	})
	if err != nil {
		return nil, fmt.Errorf("aws-kms: create ecc key: %w", err)
	}
	if output.KeyMetadata == nil || output.KeyMetadata.KeyId == nil || output.KeyMetadata.Arn == nil {
		return nil, errors.New("aws-kms: missing key metadata from create ecc key response")
	}

	publicKeyOutput, err := client.GetPublicKey(ctx, &kms.GetPublicKeyInput{KeyId: output.KeyMetadata.KeyId})
	if err != nil {
		return nil, fmt.Errorf("aws-kms: get ecc public key: %w", err)
	}

	return &models.KeyData{
		PublicKey: base64.StdEncoding.EncodeToString(publicKeyOutput.PublicKey),
		KeyID:     sdkaws.ToString(output.KeyMetadata.KeyId),
		KeyRef:    sdkaws.ToString(output.KeyMetadata.Arn),
		Provider:  "aws-kms",
	}, nil
}

func (repository *asymmetricRepository) RSA_OAEP_Encode(ctx context.Context, publicKey, text string) (string, error) {
	if _, err := utilities.ParseRSAPublicKeyFromBase64(publicKey); err == nil {
		return repository.local.RSA_OAEP_Encode(ctx, publicKey, text)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(publicKey)
	if err != nil {
		return "", err
	}

	output, err := client.Encrypt(ctx, &kms.EncryptInput{
		KeyId:               sdkaws.String(keyID),
		Plaintext:           []byte(text),
		EncryptionAlgorithm: types.EncryptionAlgorithmSpecRsaesOaepSha256,
	})
	if err != nil {
		return "", fmt.Errorf("aws-kms: encrypt with rsa-oaep-sha256: %w", err)
	}
	return base64.StdEncoding.EncodeToString(output.CiphertextBlob), nil
}

func (repository *asymmetricRepository) RSA_OAEP_Decode(ctx context.Context, privateKey, cipherText string) (string, error) {
	if _, err := utilities.ParseRSAPrivateKeyFromBase64(privateKey); err == nil {
		return repository.local.RSA_OAEP_Decode(ctx, privateKey, cipherText)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(privateKey)
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("aws-kms: decode base64 ciphertext: %w", err)
	}

	output, err := client.Decrypt(ctx, &kms.DecryptInput{
		KeyId:               sdkaws.String(keyID),
		CiphertextBlob:      ciphertext,
		EncryptionAlgorithm: types.EncryptionAlgorithmSpecRsaesOaepSha256,
	})
	if err != nil {
		return "", fmt.Errorf("aws-kms: decrypt with rsa-oaep-sha256: %w", err)
	}
	return string(output.Plaintext), nil
}

func (repository *asymmetricRepository) ECC_Encode(ctx context.Context, publicKey, text string) (string, error) {
	if _, err := utilities.ParseECDHPublicKeyFromBase64(publicKey); err == nil {
		return repository.local.ECC_Encode(ctx, publicKey, text)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(publicKey)
	if err != nil {
		return "", err
	}

	publicKeyOutput, err := client.GetPublicKey(ctx, &kms.GetPublicKeyInput{KeyId: sdkaws.String(keyID)})
	if err != nil {
		return "", fmt.Errorf("aws-kms: get ecc public key: %w", err)
	}

	return repository.local.ECC_Encode(ctx, base64.StdEncoding.EncodeToString(publicKeyOutput.PublicKey), text)
}

func (repository *asymmetricRepository) ECC_Decode(ctx context.Context, privateKey, cipherText string) (string, error) {
	if _, err := utilities.ParseECDHPrivateKeyFromBase64(privateKey); err == nil {
		return repository.local.ECC_Decode(ctx, privateKey, cipherText)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(privateKey)
	if err != nil {
		return "", err
	}

	payload, err := utilities.DecodeECCCipherPayload(cipherText)
	if err != nil {
		return "", err
	}

	ephemeralPublicKeyDER, err := base64.StdEncoding.DecodeString(payload.EphemeralPublicKey)
	if err != nil {
		return "", fmt.Errorf("aws-kms: decode ephemeral public key: %w", err)
	}

	sharedSecretOutput, err := client.DeriveSharedSecret(ctx, &kms.DeriveSharedSecretInput{
		KeyAgreementAlgorithm: types.KeyAgreementAlgorithmSpecEcdh,
		KeyId:                 sdkaws.String(keyID),
		PublicKey:             ephemeralPublicKeyDER,
	})
	if err != nil {
		return "", fmt.Errorf("aws-kms: derive shared secret: %w", err)
	}

	derivedKey, err := utilities.DeriveECCAESKey(sharedSecretOutput.SharedSecret, payload.Curve)
	if err != nil {
		return "", err
	}

	return local.NewSymmetricRepository().DecryptAES(ctx, base64.StdEncoding.EncodeToString(derivedKey), payload.Ciphertext, &payload.Curve)
}

func (repository *signatureRepository) GenerateEd255Keys(ctx context.Context, size common.SizeAsymetrycKey) (*models.KeyData, error) {
	_ = size
	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return nil, err
	}

	output, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		KeyUsage: types.KeyUsageTypeSignVerify,
		KeySpec:  types.KeySpecEccNistEdwards25519,
		Origin:   types.OriginTypeAwsKms,
	})
	if err != nil {
		return nil, fmt.Errorf("aws-kms: create ed25519 key: %w", err)
	}
	if output.KeyMetadata == nil || output.KeyMetadata.KeyId == nil || output.KeyMetadata.Arn == nil {
		return nil, errors.New("aws-kms: missing key metadata from create ed25519 key response")
	}

	publicKeyOutput, err := client.GetPublicKey(ctx, &kms.GetPublicKeyInput{
		KeyId: output.KeyMetadata.KeyId,
	})
	if err != nil {
		return nil, fmt.Errorf("aws-kms: get ed25519 public key: %w", err)
	}

	keyID := sdkaws.ToString(output.KeyMetadata.KeyId)
	keyRef := sdkaws.ToString(output.KeyMetadata.Arn)
	publicKey := base64.StdEncoding.EncodeToString(publicKeyOutput.PublicKey)
	return &models.KeyData{
		PublicKey: publicKey,
		KeyID:     keyID,
		KeyRef:    keyRef,
		Provider:  "aws-kms",
	}, nil
}

func (repository *signatureRepository) SignEd25519(ctx context.Context, privateKey, text string) (string, error) {
	if _, err := utilities.ParseEd25519PrivateKeyFromBase64(privateKey); err == nil {
		return repository.local.SignEd25519(ctx, privateKey, text)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(privateKey)
	if err != nil {
		return "", err
	}

	output, err := client.Sign(ctx, &kms.SignInput{
		KeyId:            sdkaws.String(keyID),
		Message:          []byte(text),
		MessageType:      types.MessageTypeRaw,
		SigningAlgorithm: types.SigningAlgorithmSpecEd25519Sha512,
	})
	if err != nil {
		return "", fmt.Errorf("aws-kms: sign ed25519: %w", err)
	}
	return base64.StdEncoding.EncodeToString(output.Signature), nil
}

func (repository *signatureRepository) VerifyEd25519(ctx context.Context, publicKey, text, signature string) error {
	if _, err := utilities.ParseEd25519PublicKeyFromBase64(publicKey); err == nil {
		return repository.local.VerifyEd25519(ctx, publicKey, text, signature)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return err
	}

	keyID, err := resolveAWSKMSKeyID(publicKey)
	if err != nil {
		return err
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("aws-kms: decode signature from base64: %w", err)
	}

	output, err := client.Verify(ctx, &kms.VerifyInput{
		KeyId:            sdkaws.String(keyID),
		Message:          []byte(text),
		MessageType:      types.MessageTypeRaw,
		Signature:        signatureBytes,
		SigningAlgorithm: types.SigningAlgorithmSpecEd25519Sha512,
	})
	if err != nil {
		return fmt.Errorf("aws-kms: verify ed25519: %w", err)
	}
	if !output.SignatureValid {
		return errors.New("aws-kms: invalid Ed25519 signature")
	}
	return nil
}

func (repository *signatureRepository) SignRSAPSS(ctx context.Context, privateKey, text string) (string, error) {
	if _, err := utilities.ParseRSAPrivateKeyFromBase64(privateKey); err == nil {
		return repository.local.SignRSAPSS(ctx, privateKey, text)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID(privateKey)
	if err != nil {
		return "", err
	}

	output, err := client.Sign(ctx, &kms.SignInput{
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

func (repository *signatureRepository) VerifyRSAPSS(ctx context.Context, publicKey, text, signature string) error {
	if _, err := utilities.ParseRSAPublicKeyFromBase64(publicKey); err == nil {
		return repository.local.VerifyRSAPSS(ctx, publicKey, text, signature)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return err
	}

	keyID, err := resolveAWSKMSKeyID(publicKey)
	if err != nil {
		return err
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("aws-kms: decode signature from base64: %w", err)
	}

	output, err := client.Verify(ctx, &kms.VerifyInput{
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

func (repository *signatureRepository) SignPKCS1v15_SHA256(ctx context.Context, data string, privateKey *rsa.PrivateKey) (string, error) {
	if privateKey != nil {
		return repository.local.SignPKCS1v15_SHA256(ctx, data, privateKey)
	}

	client, err := newAWSKMSClient(ctx)
	if err != nil {
		return "", err
	}

	keyID, err := resolveAWSKMSKeyID("")
	if err != nil {
		return "", err
	}

	output, err := client.Sign(ctx, &kms.SignInput{
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

func (repository *signatureRepository) VerifySHA256(ctx context.Context, data, signature string, publicKey *rsa.PublicKey) error {
	if publicKey != nil {
		return repository.local.VerifySHA256(ctx, data, signature, publicKey)
	}

	client, err := newAWSKMSClient(ctx)
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

	output, err := client.Verify(ctx, &kms.VerifyInput{
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

func toAWSECCKeySpec(curve common.CurveAsymmetricKey) (types.KeySpec, error) {
	switch curve {
	case common.CurveP256:
		return types.KeySpecEccNistP256, nil
	case common.CurveP384:
		return types.KeySpecEccNistP384, nil
	case common.CurveP521:
		return types.KeySpecEccNistP521, nil
	default:
		return "", fmt.Errorf("aws-kms: unsupported ecc curve: %q", curve)
	}
}

func toAWSSymmetricKeySpec(size common.SizeSymetrycKey) (types.KeySpec, error) {
	switch size {
	case common.Key256Bits:
		return types.KeySpecSymmetricDefault, nil
	default:
		return "", fmt.Errorf("aws-kms: unsupported symmetric key size: %d", size)
	}
}

func looksLikeAWSKMSKeyReference(key string) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return strings.TrimSpace(viper.GetString(defaultKMSARNKey)) != ""
	}
	return strings.HasPrefix(trimmed, "arn:aws:kms:") ||
		strings.HasPrefix(trimmed, "alias/") ||
		strings.HasPrefix(trimmed, "mrk-") ||
		strings.Count(trimmed, "-") >= 4
}
