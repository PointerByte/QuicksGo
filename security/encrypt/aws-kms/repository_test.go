// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package awskms

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/PointerByte/QuicksGo/security/encrypt/common"
	"github.com/PointerByte/QuicksGo/security/encrypt/local"
	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	kms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/spf13/viper"
)

type fakeKMSClient struct {
	createKeyFn    func(context.Context, *kms.CreateKeyInput, ...func(*kms.Options)) (*kms.CreateKeyOutput, error)
	getPublicKeyFn func(context.Context, *kms.GetPublicKeyInput, ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error)
	encryptFn      func(context.Context, *kms.EncryptInput, ...func(*kms.Options)) (*kms.EncryptOutput, error)
	decryptFn      func(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error)
	generateMacFn  func(context.Context, *kms.GenerateMacInput, ...func(*kms.Options)) (*kms.GenerateMacOutput, error)
	verifyMacFn    func(context.Context, *kms.VerifyMacInput, ...func(*kms.Options)) (*kms.VerifyMacOutput, error)
	signFn         func(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error)
	verifyFn       func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error)
}

var testContext = context.Background()

func (fake fakeKMSClient) CreateKey(ctx context.Context, in *kms.CreateKeyInput, optFns ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
	return fake.createKeyFn(ctx, in, optFns...)
}
func (fake fakeKMSClient) GetPublicKey(ctx context.Context, in *kms.GetPublicKeyInput, optFns ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error) {
	return fake.getPublicKeyFn(ctx, in, optFns...)
}
func (fake fakeKMSClient) Encrypt(ctx context.Context, in *kms.EncryptInput, optFns ...func(*kms.Options)) (*kms.EncryptOutput, error) {
	return fake.encryptFn(ctx, in, optFns...)
}
func (fake fakeKMSClient) Decrypt(ctx context.Context, in *kms.DecryptInput, optFns ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	return fake.decryptFn(ctx, in, optFns...)
}
func (fake fakeKMSClient) GenerateMac(ctx context.Context, in *kms.GenerateMacInput, optFns ...func(*kms.Options)) (*kms.GenerateMacOutput, error) {
	return fake.generateMacFn(ctx, in, optFns...)
}
func (fake fakeKMSClient) VerifyMac(ctx context.Context, in *kms.VerifyMacInput, optFns ...func(*kms.Options)) (*kms.VerifyMacOutput, error) {
	return fake.verifyMacFn(ctx, in, optFns...)
}
func (fake fakeKMSClient) Sign(ctx context.Context, in *kms.SignInput, optFns ...func(*kms.Options)) (*kms.SignOutput, error) {
	return fake.signFn(ctx, in, optFns...)
}
func (fake fakeKMSClient) Verify(ctx context.Context, in *kms.VerifyInput, optFns ...func(*kms.Options)) (*kms.VerifyOutput, error) {
	return fake.verifyFn(ctx, in, optFns...)
}

func TestNewRepositoryBuildsAllRepositories(t *testing.T) {
	repository := NewRepository()
	if repository.SymmetricRepository == nil || repository.AsymmetricRepository == nil || repository.SignatureRepository == nil || repository.HashRepository == nil {
		t.Fatal("expected all repositories to be initialized")
	}
}

func TestDelegatedLocalHelpers(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousLoad := loadAWSConfigFn
	previousClient := newKMSClientFn
	t.Cleanup(func() {
		loadAWSConfigFn = previousLoad
		newKMSClientFn = previousClient
	})

	loadAWSConfigFn = func(context.Context) (sdkaws.Config, error) { return sdkaws.Config{}, nil }
	newKMSClientFn = func(cfg sdkaws.Config) kmsClient {
		return fakeKMSClient{
			createKeyFn: func(_ context.Context, in *kms.CreateKeyInput, _ ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
				if in.KeySpec != types.KeySpecSymmetricDefault {
					t.Fatalf("CreateKey() KeySpec = %q, want %q", in.KeySpec, types.KeySpecSymmetricDefault)
				}
				return &kms.CreateKeyOutput{
					KeyMetadata: &types.KeyMetadata{
						Arn:   sdkaws.String("arn:aws:kms:test-symmetric"),
						KeyId: sdkaws.String("kms-symmetric-id"),
					},
				}, nil
			},
			encryptFn: func(_ context.Context, in *kms.EncryptInput, _ ...func(*kms.Options)) (*kms.EncryptOutput, error) {
				if got := in.EncryptionContext["additional"]; got != "aad" {
					t.Fatalf("Encrypt() encryption context additional = %q, want aad", got)
				}
				return &kms.EncryptOutput{CiphertextBlob: []byte("cipher")}, nil
			},
			decryptFn: func(_ context.Context, in *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
				if got := in.EncryptionContext["additional"]; got != "aad" {
					t.Fatalf("Decrypt() encryption context additional = %q, want aad", got)
				}
				return &kms.DecryptOutput{Plaintext: []byte("hello")}, nil
			},
			generateMacFn: func(_ context.Context, in *kms.GenerateMacInput, _ ...func(*kms.Options)) (*kms.GenerateMacOutput, error) {
				if in.MacAlgorithm != types.MacAlgorithmSpecHmacSha256 {
					t.Fatalf("GenerateMac() algorithm = %q, want %q", in.MacAlgorithm, types.MacAlgorithmSpecHmacSha256)
				}
				return &kms.GenerateMacOutput{Mac: []byte("mac")}, nil
			},
			verifyMacFn: func(_ context.Context, in *kms.VerifyMacInput, _ ...func(*kms.Options)) (*kms.VerifyMacOutput, error) {
				if in.MacAlgorithm != types.MacAlgorithmSpecHmacSha256 {
					t.Fatalf("VerifyMac() algorithm = %q, want %q", in.MacAlgorithm, types.MacAlgorithmSpecHmacSha256)
				}
				return &kms.VerifyMacOutput{MacValid: true}, nil
			},
		}
	}

	repository := NewRepository()
	key, err := repository.GeneratesSymetrycKey(testContext, common.Key256Bits)
	if err != nil {
		t.Fatalf("GeneratesSymetrycKey() error = %v", err)
	}
	if key == nil || key.Key != "" || key.KeyID != "kms-symmetric-id" || key.KeyRef != "arn:aws:kms:test-symmetric" || key.Provider != "aws-kms" {
		t.Fatalf("GeneratesSymetrycKey() = %#v, want KMS symmetric key metadata", key)
	}

	additional := "aad"
	ciphertext, err := repository.EncryptAES(testContext, key.KeyRef, "hello", &additional)
	if err != nil {
		t.Fatalf("EncryptAES() error = %v", err)
	}
	plaintext, err := repository.DecryptAES(testContext, key.KeyRef, ciphertext, "aad")
	if err != nil {
		t.Fatalf("DecryptAES() error = %v", err)
	}
	if plaintext != "hello" {
		t.Fatalf("DecryptAES() = %q, want %q", plaintext, "hello")
	}
	if got := repository.GenerateHMAC(testContext, "message", key.KeyRef); got == "" {
		t.Fatal("expected GenerateHMAC() to return a KMS MAC")
	}
	if !repository.ValidateHMAC(testContext, "message", key.KeyRef, base64.StdEncoding.EncodeToString([]byte("mac"))) {
		t.Fatal("expected ValidateHMAC() to succeed with KMS MAC")
	}
	if repository.Sha256Hex(testContext, "message") == "" || repository.Blake3(testContext, "message") == "" {
		t.Fatal("expected digest helpers to return values")
	}
	localRepository := local.NewSymmetricRepository()
	localKey, err := localRepository.GeneratesSymetrycKey(testContext, common.Key256Bits)
	if err != nil {
		t.Fatalf("local GeneratesSymetrycKey() error = %v", err)
	}
	if _, err := repository.EncryptAES(testContext, localKey.Key, "hello", &additional); err != nil {
		t.Fatalf("EncryptAES() local fallback error = %v", err)
	}
}

func TestGeneratesSymetrycKeyUsesConfiguredKeyMetadata(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousLoad := loadAWSConfigFn
	previousClient := newKMSClientFn
	t.Cleanup(func() {
		loadAWSConfigFn = previousLoad
		newKMSClientFn = previousClient
	})

	loadAWSConfigFn = func(context.Context) (sdkaws.Config, error) { return sdkaws.Config{}, nil }
	newKMSClientFn = func(cfg sdkaws.Config) kmsClient {
		return fakeKMSClient{
			createKeyFn: func(_ context.Context, _ *kms.CreateKeyInput, _ ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
				return &kms.CreateKeyOutput{
					KeyMetadata: &types.KeyMetadata{
						Arn:   sdkaws.String("arn:aws:kms:us-east-1:123456789012:key/test-key"),
						KeyId: sdkaws.String("test-key-id"),
					},
				}, nil
			},
		}
	}

	repository := NewRepository()
	key, err := repository.GeneratesSymetrycKey(testContext, common.Key256Bits)
	if err != nil {
		t.Fatalf("GeneratesSymetrycKey() error = %v", err)
	}
	if key == nil || key.Provider != "aws-kms" || key.KeyID != "test-key-id" || key.KeyRef != "arn:aws:kms:us-east-1:123456789012:key/test-key" {
		t.Fatalf("GeneratesSymetrycKey() = %#v, want aws-kms metadata", key)
	}
}

func TestResolveAWSKMSKeyIDAndKeySpec(t *testing.T) {
	t.Cleanup(viper.Reset)
	if _, err := resolveAWSKMSKeyID(""); err == nil {
		t.Fatal("expected resolveAWSKMSKeyID() error")
	}
	viper.Set(defaultKMSARNKey, "arn:aws:kms:test")
	if got, err := resolveAWSKMSKeyID(""); err != nil || got != "arn:aws:kms:test" {
		t.Fatalf("resolveAWSKMSKeyID() = %q, %v", got, err)
	}
	if got, err := resolveAWSKMSKeyID("direct"); err != nil || got != "direct" {
		t.Fatalf("resolveAWSKMSKeyID() direct = %q, %v", got, err)
	}
	if _, err := toAWSRSAKeySpec(common.Key3072Bits); err != nil {
		t.Fatalf("toAWSRSAKeySpec(3072) error = %v", err)
	}
	if _, err := toAWSRSAKeySpec(common.Key4096Bits); err != nil {
		t.Fatalf("toAWSRSAKeySpec(4096) error = %v", err)
	}
	if _, err := toAWSRSAKeySpec(0); err == nil {
		t.Fatal("expected toAWSRSAKeySpec() error")
	}
	if _, err := toAWSSymmetricKeySpec(common.Key256Bits); err != nil {
		t.Fatalf("toAWSSymmetricKeySpec(256) error = %v", err)
	}
	if _, err := toAWSSymmetricKeySpec(common.Key128Bits); err == nil {
		t.Fatal("expected toAWSSymmetricKeySpec(128) error")
	}
}

func TestNewAWSKMSClientError(t *testing.T) {
	previous := loadAWSConfigFn
	t.Cleanup(func() { loadAWSConfigFn = previous })

	loadAWSConfigFn = func(context.Context) (sdkaws.Config, error) {
		return sdkaws.Config{}, errors.New("boom")
	}

	if _, err := newAWSKMSClient(context.Background()); err == nil {
		t.Fatal("expected newAWSKMSClient() error")
	}
}

func TestAsymmetricAndSignatureProviderFlows(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousLoad := loadAWSConfigFn
	previousClient := newKMSClientFn
	t.Cleanup(func() {
		loadAWSConfigFn = previousLoad
		newKMSClientFn = previousClient
	})

	privateKey := mustRSAKey(t)
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	edPublic, edPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}
	edPublicDER, err := x509.MarshalPKIXPublicKey(edPublic)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	edPrivateDER, err := x509.MarshalPKCS8PrivateKey(edPrivate)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}
	localEdPrivate := base64.StdEncoding.EncodeToString(edPrivateDER)
	localEdPublic := base64.StdEncoding.EncodeToString(edPublicDER)

	loadAWSConfigFn = func(context.Context) (sdkaws.Config, error) { return sdkaws.Config{}, nil }
	newKMSClientFn = func(cfg sdkaws.Config) kmsClient {
		return fakeKMSClient{
			createKeyFn: func(_ context.Context, in *kms.CreateKeyInput, _ ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
				if in.KeySpec == types.KeySpecEccNistEdwards25519 {
					return &kms.CreateKeyOutput{KeyMetadata: &types.KeyMetadata{Arn: sdkaws.String("arn:aws:kms:ed25519"), KeyId: sdkaws.String("ed25519-key-id")}}, nil
				}
				return &kms.CreateKeyOutput{KeyMetadata: &types.KeyMetadata{Arn: sdkaws.String("arn:aws:kms:test"), KeyId: sdkaws.String("key-id")}}, nil
			},
			getPublicKeyFn: func(_ context.Context, in *kms.GetPublicKeyInput, _ ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error) {
				if sdkaws.ToString(in.KeyId) == "ed25519-key-id" {
					return &kms.GetPublicKeyOutput{PublicKey: edPublicDER}, nil
				}
				return &kms.GetPublicKeyOutput{PublicKey: publicDER}, nil
			},
			encryptFn: func(context.Context, *kms.EncryptInput, ...func(*kms.Options)) (*kms.EncryptOutput, error) {
				return &kms.EncryptOutput{CiphertextBlob: []byte("cipher")}, nil
			},
			decryptFn: func(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error) {
				return &kms.DecryptOutput{Plaintext: []byte("plain")}, nil
			},
			generateMacFn: func(context.Context, *kms.GenerateMacInput, ...func(*kms.Options)) (*kms.GenerateMacOutput, error) {
				return &kms.GenerateMacOutput{Mac: []byte("mac")}, nil
			},
			verifyMacFn: func(context.Context, *kms.VerifyMacInput, ...func(*kms.Options)) (*kms.VerifyMacOutput, error) {
				return &kms.VerifyMacOutput{MacValid: true}, nil
			},
			signFn: func(_ context.Context, in *kms.SignInput, _ ...func(*kms.Options)) (*kms.SignOutput, error) {
				if in.SigningAlgorithm == types.SigningAlgorithmSpecEd25519Sha512 {
					return &kms.SignOutput{Signature: []byte("ed-sig-" + string(in.Message))}, nil
				}
				return &kms.SignOutput{Signature: []byte("sig-" + string(in.Message))}, nil
			},
			verifyFn: func(_ context.Context, in *kms.VerifyInput, _ ...func(*kms.Options)) (*kms.VerifyOutput, error) {
				if in.SigningAlgorithm == types.SigningAlgorithmSpecEd25519Sha512 {
					return &kms.VerifyOutput{SignatureValid: true}, nil
				}
				return &kms.VerifyOutput{SignatureValid: true}, nil
			},
		}
	}

	asymmetricRepository := NewAsymmetricRepository()
	signatureRepository := NewSignatureRepository()

	keyData, err := asymmetricRepository.GeneratesRSAKey(testContext, common.Key2048Bits)
	if err != nil {
		t.Fatalf("GeneratesRSAKey() error = %v", err)
	}
	if keyData == nil || keyData.PublicKey == "" || keyData.PrivateKey != "" || keyData.KeyID == "" || keyData.KeyRef == "" || keyData.Provider != "aws-kms" {
		t.Fatalf("GeneratesRSAKey() = %#v, want public key metadata", keyData)
	}

	ciphertext, err := asymmetricRepository.RSA_OAEP_Encode(testContext, keyData.KeyRef, "payload")
	if err != nil {
		t.Fatalf("RSA_OAEP_Encode() error = %v", err)
	}
	if ciphertext == "" {
		t.Fatal("expected ciphertext")
	}
	plaintext, err := asymmetricRepository.RSA_OAEP_Decode(testContext, keyData.KeyRef, base64.StdEncoding.EncodeToString([]byte("cipher")))
	if err != nil {
		t.Fatalf("RSA_OAEP_Decode() error = %v", err)
	}
	if plaintext != "plain" {
		t.Fatalf("RSA_OAEP_Decode() = %q, want %q", plaintext, "plain")
	}

	signature, err := signatureRepository.SignRSAPSS(testContext, keyData.KeyRef, "payload")
	if err != nil {
		t.Fatalf("SignRSAPSS() error = %v", err)
	}
	if signature == "" {
		t.Fatal("expected signature")
	}
	if err := signatureRepository.VerifyRSAPSS(testContext, keyData.KeyRef, "payload", base64.StdEncoding.EncodeToString([]byte("signature"))); err != nil {
		t.Fatalf("VerifyRSAPSS() error = %v", err)
	}

	viper.Set(defaultKMSARNKey, keyData.KeyRef)
	signature, err = signatureRepository.SignSHA256(testContext, "payload", nil)
	if err != nil {
		t.Fatalf("SignSHA256() error = %v", err)
	}
	if err := signatureRepository.VerifySHA256(testContext, "payload", signature, nil); err != nil {
		t.Fatalf("VerifySHA256() error = %v", err)
	}

	edKeyData, err := signatureRepository.GeneratesEd255Key(testContext, common.Key2048Bits)
	if err != nil {
		t.Fatalf("GeneratesEd255Key() error = %v", err)
	}
	if edKeyData == nil || edKeyData.PublicKey == "" || edKeyData.PrivateKey != "" || edKeyData.KeyID == "" || edKeyData.KeyRef == "" || edKeyData.Provider != "aws-kms" {
		t.Fatalf("GeneratesEd255Key() = %#v, want public key metadata", edKeyData)
	}
	edSignature, err := signatureRepository.SignEd25519(testContext, edKeyData.KeyRef, "payload")
	if err != nil {
		t.Fatalf("SignEd25519() error = %v", err)
	}
	if err := signatureRepository.VerifyEd25519(testContext, edKeyData.KeyRef, "payload", edSignature); err != nil {
		t.Fatalf("VerifyEd25519() error = %v", err)
	}

	edSignature, err = signatureRepository.SignEd25519(testContext, localEdPrivate, "payload")
	if err != nil {
		t.Fatalf("SignEd25519() local fallback error = %v", err)
	}
	if err := signatureRepository.VerifyEd25519(testContext, localEdPublic, "payload", edSignature); err != nil {
		t.Fatalf("VerifyEd25519() local fallback error = %v", err)
	}
}

func TestAWSKMSProviderErrorsAndFallbacks(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousLoad := loadAWSConfigFn
	previousClient := newKMSClientFn
	t.Cleanup(func() {
		loadAWSConfigFn = previousLoad
		newKMSClientFn = previousClient
	})

	loadAWSConfigFn = func(context.Context) (sdkaws.Config, error) { return sdkaws.Config{}, nil }
	newKMSClientFn = func(cfg sdkaws.Config) kmsClient {
		return fakeKMSClient{
			createKeyFn: func(context.Context, *kms.CreateKeyInput, ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
				return nil, errors.New("create boom")
			},
			getPublicKeyFn: func(context.Context, *kms.GetPublicKeyInput, ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error) {
				return nil, errors.New("get boom")
			},
			encryptFn: func(context.Context, *kms.EncryptInput, ...func(*kms.Options)) (*kms.EncryptOutput, error) {
				return nil, errors.New("encrypt boom")
			},
			decryptFn: func(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error) {
				return nil, errors.New("decrypt boom")
			},
			generateMacFn: func(context.Context, *kms.GenerateMacInput, ...func(*kms.Options)) (*kms.GenerateMacOutput, error) {
				return nil, errors.New("generate mac boom")
			},
			verifyMacFn: func(context.Context, *kms.VerifyMacInput, ...func(*kms.Options)) (*kms.VerifyMacOutput, error) {
				return nil, errors.New("verify mac boom")
			},
			signFn: func(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error) {
				return nil, errors.New("sign boom")
			},
			verifyFn: func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error) {
				return &kms.VerifyOutput{SignatureValid: false}, nil
			},
		}
	}

	asymmetricRepository := NewAsymmetricRepository()
	symmetricRepository := NewSymmetricRepository()
	signatureRepository := NewSignatureRepository()

	if _, err := symmetricRepository.GeneratesSymetrycKey(testContext, common.Key128Bits); err == nil {
		t.Fatal("expected GeneratesSymetrycKey() symmetric key spec error")
	}
	if _, err := symmetricRepository.GeneratesSymetrycKey(testContext, common.Key256Bits); err == nil {
		t.Fatal("expected GeneratesSymetrycKey() create error")
	}
	if _, err := asymmetricRepository.GeneratesRSAKey(testContext, 0); err == nil {
		t.Fatal("expected GeneratesRSAKey() key spec error")
	}
	if _, err := asymmetricRepository.GeneratesRSAKey(testContext, common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesRSAKey() create error")
	}

	newKMSClientFn = func(cfg sdkaws.Config) kmsClient {
		return fakeKMSClient{
			createKeyFn: func(context.Context, *kms.CreateKeyInput, ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
				return &kms.CreateKeyOutput{KeyMetadata: &types.KeyMetadata{}}, nil
			},
			getPublicKeyFn: func(context.Context, *kms.GetPublicKeyInput, ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error) {
				return nil, nil
			},
			encryptFn: func(context.Context, *kms.EncryptInput, ...func(*kms.Options)) (*kms.EncryptOutput, error) {
				return nil, nil
			},
			decryptFn: func(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error) {
				return nil, nil
			},
			generateMacFn: func(context.Context, *kms.GenerateMacInput, ...func(*kms.Options)) (*kms.GenerateMacOutput, error) {
				return nil, nil
			},
			verifyMacFn: func(context.Context, *kms.VerifyMacInput, ...func(*kms.Options)) (*kms.VerifyMacOutput, error) {
				return nil, nil
			},
			signFn: func(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error) { return nil, nil },
			verifyFn: func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error) {
				return nil, nil
			},
		}
	}
	if _, err := symmetricRepository.GeneratesSymetrycKey(testContext, common.Key256Bits); err == nil {
		t.Fatal("expected GeneratesSymetrycKey() missing metadata error")
	}
	if _, err := asymmetricRepository.GeneratesRSAKey(testContext, common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesRSAKey() missing metadata error")
	}

	newKMSClientFn = func(cfg sdkaws.Config) kmsClient {
		return fakeKMSClient{
			createKeyFn: func(context.Context, *kms.CreateKeyInput, ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
				return &kms.CreateKeyOutput{KeyMetadata: &types.KeyMetadata{Arn: sdkaws.String("arn"), KeyId: sdkaws.String("id")}}, nil
			},
			getPublicKeyFn: func(context.Context, *kms.GetPublicKeyInput, ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error) {
				return nil, errors.New("public boom")
			},
			encryptFn: func(context.Context, *kms.EncryptInput, ...func(*kms.Options)) (*kms.EncryptOutput, error) {
				return nil, nil
			},
			decryptFn: func(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error) {
				return nil, nil
			},
			generateMacFn: func(context.Context, *kms.GenerateMacInput, ...func(*kms.Options)) (*kms.GenerateMacOutput, error) {
				return nil, nil
			},
			verifyMacFn: func(context.Context, *kms.VerifyMacInput, ...func(*kms.Options)) (*kms.VerifyMacOutput, error) {
				return nil, nil
			},
			signFn: func(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error) { return nil, nil },
			verifyFn: func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error) {
				return nil, nil
			},
		}
	}
	if _, err := asymmetricRepository.GeneratesRSAKey(testContext, common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesRSAKey() get public key error")
	}

	newKMSClientFn = func(cfg sdkaws.Config) kmsClient {
		return fakeKMSClient{
			createKeyFn: func(context.Context, *kms.CreateKeyInput, ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
				return nil, errors.New("create boom")
			},
			getPublicKeyFn: func(context.Context, *kms.GetPublicKeyInput, ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error) {
				return nil, errors.New("get boom")
			},
			encryptFn: func(context.Context, *kms.EncryptInput, ...func(*kms.Options)) (*kms.EncryptOutput, error) {
				return nil, errors.New("encrypt boom")
			},
			decryptFn: func(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error) {
				return nil, errors.New("decrypt boom")
			},
			generateMacFn: func(context.Context, *kms.GenerateMacInput, ...func(*kms.Options)) (*kms.GenerateMacOutput, error) {
				return nil, errors.New("generate mac boom")
			},
			verifyMacFn: func(context.Context, *kms.VerifyMacInput, ...func(*kms.Options)) (*kms.VerifyMacOutput, error) {
				return nil, errors.New("verify mac boom")
			},
			signFn: func(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error) {
				return nil, errors.New("sign boom")
			},
			verifyFn: func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error) {
				return &kms.VerifyOutput{SignatureValid: false}, nil
			},
		}
	}
	if _, err := asymmetricRepository.RSA_OAEP_Encode(testContext, "", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() key id error")
	}
	viper.Set(defaultKMSARNKey, "arn:aws:kms:test")
	if _, err := symmetricRepository.EncryptAES(testContext, "", "payload", nil); err == nil {
		t.Fatal("expected EncryptAES() provider error")
	}
	if _, err := symmetricRepository.DecryptAES(testContext, "", "%%%", "aad"); err == nil {
		t.Fatal("expected DecryptAES() decode error")
	}
	if _, err := symmetricRepository.DecryptAES(testContext, "", base64.StdEncoding.EncodeToString([]byte("cipher")), "aad"); err == nil {
		t.Fatal("expected DecryptAES() provider error")
	}
	if got := NewHashRepository().GenerateHMAC(testContext, "message", "arn:aws:kms:test"); got != "" {
		t.Fatalf("GenerateHMAC() = %q, want empty string on provider error", got)
	}
	if NewHashRepository().ValidateHMAC(testContext, "message", "arn:aws:kms:test", base64.StdEncoding.EncodeToString([]byte("mac"))) {
		t.Fatal("expected ValidateHMAC() to fail on provider error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Encode(testContext, "", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() provider error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode(testContext, "", "%%%"); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() decode error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode(testContext, "", base64.StdEncoding.EncodeToString([]byte("cipher"))); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() provider error")
	}

	if _, err := signatureRepository.GeneratesEd255Key(testContext, common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesEd255Key() create error")
	}
	if _, err := signatureRepository.SignEd25519(testContext, "", "payload"); err == nil {
		t.Fatal("expected SignEd25519() key id error")
	}
	if err := signatureRepository.VerifyEd25519(testContext, "", "payload", "sig"); err == nil {
		t.Fatal("expected VerifyEd25519() key id error")
	}
	if _, err := signatureRepository.SignEd25519(testContext, "arn:aws:kms:test", "payload"); err == nil {
		t.Fatal("expected SignEd25519() provider error")
	}
	if err := signatureRepository.VerifyEd25519(testContext, "arn:aws:kms:test", "payload", "%%%"); err == nil {
		t.Fatal("expected VerifyEd25519() decode error")
	}
	if err := signatureRepository.VerifyEd25519(testContext, "arn:aws:kms:test", "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected VerifyEd25519() provider error")
	}
	if _, err := signatureRepository.SignRSAPSS(testContext, "", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() provider error")
	}
	if err := signatureRepository.VerifyRSAPSS(testContext, "", "payload", "%%%"); err == nil {
		t.Fatal("expected VerifyRSAPSS() decode error")
	}
	if err := signatureRepository.VerifyRSAPSS(testContext, "", "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected VerifyRSAPSS() invalid signature error")
	}
	if _, err := signatureRepository.SignSHA256(testContext, "payload", nil); err == nil {
		t.Fatal("expected SignSHA256() provider error")
	}
	if err := signatureRepository.VerifySHA256(testContext, "payload", "%%%", nil); err == nil {
		t.Fatal("expected VerifySHA256() decode error")
	}
	if err := signatureRepository.VerifySHA256(testContext, "payload", base64.StdEncoding.EncodeToString([]byte("sig")), nil); err == nil {
		t.Fatal("expected VerifySHA256() invalid signature error")
	}

	privateKey := mustRSAKey(t)
	publicKey := &privateKey.PublicKey
	publicB64 := mustMarshalPKIXRSAPublicKey(t, publicKey)
	privateB64 := mustMarshalPKCS8RSAPrivateKey(t, privateKey)

	if _, err := asymmetricRepository.RSA_OAEP_Encode(testContext, publicB64, "payload"); err != nil {
		t.Fatalf("RSA_OAEP_Encode() local fallback error = %v", err)
	}
	ciphertext, err := asymmetricRepository.RSA_OAEP_Encode(testContext, publicB64, "payload")
	if err != nil {
		t.Fatalf("RSA_OAEP_Encode() local fallback error = %v", err)
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode(testContext, privateB64, ciphertext); err != nil {
		t.Fatalf("RSA_OAEP_Decode() local fallback error = %v", err)
	}
	signature, err := signatureRepository.SignRSAPSS(testContext, privateB64, "payload")
	if err != nil {
		t.Fatalf("SignRSAPSS() local fallback error = %v", err)
	}
	if err := signatureRepository.VerifyRSAPSS(testContext, publicB64, "payload", signature); err != nil {
		t.Fatalf("VerifyRSAPSS() local fallback error = %v", err)
	}
	signature, err = signatureRepository.SignSHA256(testContext, "payload", privateKey)
	if err != nil {
		t.Fatalf("SignSHA256() local fallback error = %v", err)
	}
	if err := signatureRepository.VerifySHA256(testContext, "payload", signature, publicKey); err != nil {
		t.Fatalf("VerifySHA256() local fallback error = %v", err)
	}
}

func TestAWSKMSOperationsReturnLoadConfigErrors(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousLoad := loadAWSConfigFn
	t.Cleanup(func() { loadAWSConfigFn = previousLoad })

	loadAWSConfigFn = func(context.Context) (sdkaws.Config, error) {
		return sdkaws.Config{}, errors.New("config boom")
	}

	symmetricRepository := NewSymmetricRepository()
	asymmetricRepository := NewAsymmetricRepository()
	signatureRepository := NewSignatureRepository()

	if _, err := symmetricRepository.GeneratesSymetrycKey(testContext, common.Key256Bits); err == nil {
		t.Fatal("expected GeneratesSymetrycKey() config error")
	}
	if _, err := asymmetricRepository.GeneratesRSAKey(testContext, common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesRSAKey() config error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Encode(testContext, "arn", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() config error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode(testContext, "arn", base64.StdEncoding.EncodeToString([]byte("cipher"))); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() config error")
	}
	if _, err := signatureRepository.SignRSAPSS(testContext, "arn", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() config error")
	}
	if _, err := signatureRepository.SignEd25519(testContext, "arn", "payload"); err == nil {
		t.Fatal("expected SignEd25519() config error")
	}
	if err := signatureRepository.VerifyEd25519(testContext, "arn", "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected VerifyEd25519() config error")
	}
	if err := signatureRepository.VerifyRSAPSS(testContext, "arn", "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected VerifyRSAPSS() config error")
	}
	if _, err := signatureRepository.SignSHA256(testContext, "payload", nil); err == nil {
		t.Fatal("expected SignSHA256() config error")
	}
	if err := signatureRepository.VerifySHA256(testContext, "payload", base64.StdEncoding.EncodeToString([]byte("sig")), nil); err == nil {
		t.Fatal("expected VerifySHA256() config error")
	}
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	return privateKey
}

func mustMarshalPKCS8RSAPrivateKey(t *testing.T, privateKey *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustMarshalPKIXRSAPublicKey(t *testing.T, publicKey *rsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}
