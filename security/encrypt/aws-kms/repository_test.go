// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package awskms

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/PointerByte/QuicksGo/security/encrypt/common"
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
	signFn         func(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error)
	verifyFn       func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error)
}

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
	repository := NewRepository()
	key, err := repository.GeneratesSymetrycKey(common.Key256Bits)
	if err != nil {
		t.Fatalf("GeneratesSymetrycKey() error = %v", err)
	}
	ciphertext, err := repository.EncryptAES(key, "hello", "aad")
	if err != nil {
		t.Fatalf("EncryptAES() error = %v", err)
	}
	plaintext, err := repository.DecryptAES(key, ciphertext, "aad")
	if err != nil {
		t.Fatalf("DecryptAES() error = %v", err)
	}
	if plaintext != "hello" {
		t.Fatalf("DecryptAES() = %q, want %q", plaintext, "hello")
	}
	if repository.GenerateHMAC("message", "secret") == "" || repository.Sha256Hex("message") == "" || repository.Blake3("message") == "" {
		t.Fatal("expected delegated helpers to return values")
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

	loadAWSConfigFn = func(context.Context) (sdkaws.Config, error) { return sdkaws.Config{}, nil }
	newKMSClientFn = func(cfg sdkaws.Config) kmsClient {
		return fakeKMSClient{
			createKeyFn: func(context.Context, *kms.CreateKeyInput, ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
				return &kms.CreateKeyOutput{KeyMetadata: &types.KeyMetadata{Arn: sdkaws.String("arn:aws:kms:test"), KeyId: sdkaws.String("key-id")}}, nil
			},
			getPublicKeyFn: func(context.Context, *kms.GetPublicKeyInput, ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error) {
				return &kms.GetPublicKeyOutput{PublicKey: publicDER}, nil
			},
			encryptFn: func(context.Context, *kms.EncryptInput, ...func(*kms.Options)) (*kms.EncryptOutput, error) {
				return &kms.EncryptOutput{CiphertextBlob: []byte("cipher")}, nil
			},
			decryptFn: func(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error) {
				return &kms.DecryptOutput{Plaintext: []byte("plain")}, nil
			},
			signFn: func(_ context.Context, in *kms.SignInput, _ ...func(*kms.Options)) (*kms.SignOutput, error) {
				return &kms.SignOutput{Signature: []byte("sig-" + string(in.Message))}, nil
			},
			verifyFn: func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error) {
				return &kms.VerifyOutput{SignatureValid: true}, nil
			},
		}
	}

	asymmetricRepository := NewAsymmetricRepository()
	signatureRepository := NewSignatureRepository()

	priv, pub, err := asymmetricRepository.GeneratesRSAKey(common.Key2048Bits)
	if err != nil {
		t.Fatalf("GeneratesRSAKey() error = %v", err)
	}
	if priv != "" || pub == "" || viper.GetString(defaultKMSARNKey) == "" {
		t.Fatal("expected GeneratesRSAKey() to return public key and persist arn")
	}

	ciphertext, err := asymmetricRepository.RSA_OAEP_Encode("", "payload")
	if err != nil {
		t.Fatalf("RSA_OAEP_Encode() error = %v", err)
	}
	if ciphertext == "" {
		t.Fatal("expected ciphertext")
	}
	plaintext, err := asymmetricRepository.RSA_OAEP_Decode("", base64.StdEncoding.EncodeToString([]byte("cipher")))
	if err != nil {
		t.Fatalf("RSA_OAEP_Decode() error = %v", err)
	}
	if plaintext != "plain" {
		t.Fatalf("RSA_OAEP_Decode() = %q, want %q", plaintext, "plain")
	}

	signature, err := signatureRepository.SignRSAPSS("", "payload")
	if err != nil {
		t.Fatalf("SignRSAPSS() error = %v", err)
	}
	if signature == "" {
		t.Fatal("expected signature")
	}
	if err := signatureRepository.VerifyRSAPSS("", "payload", base64.StdEncoding.EncodeToString([]byte("signature"))); err != nil {
		t.Fatalf("VerifyRSAPSS() error = %v", err)
	}

	signature, err = signatureRepository.SignSHA256("payload", nil)
	if err != nil {
		t.Fatalf("SignSHA256() error = %v", err)
	}
	if err := signatureRepository.VerifySHA256("payload", signature, nil); err != nil {
		t.Fatalf("VerifySHA256() error = %v", err)
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
			signFn: func(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error) {
				return nil, errors.New("sign boom")
			},
			verifyFn: func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error) {
				return &kms.VerifyOutput{SignatureValid: false}, nil
			},
		}
	}

	asymmetricRepository := NewAsymmetricRepository()
	signatureRepository := NewSignatureRepository()

	if _, _, err := asymmetricRepository.GeneratesRSAKey(0); err == nil {
		t.Fatal("expected GeneratesRSAKey() key spec error")
	}
	if _, _, err := asymmetricRepository.GeneratesRSAKey(common.Key2048Bits); err == nil {
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
			signFn: func(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error) { return nil, nil },
			verifyFn: func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error) {
				return nil, nil
			},
		}
	}
	if _, _, err := asymmetricRepository.GeneratesRSAKey(common.Key2048Bits); err == nil {
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
			signFn: func(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error) { return nil, nil },
			verifyFn: func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error) {
				return nil, nil
			},
		}
	}
	if _, _, err := asymmetricRepository.GeneratesRSAKey(common.Key2048Bits); err == nil {
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
			signFn: func(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error) {
				return nil, errors.New("sign boom")
			},
			verifyFn: func(context.Context, *kms.VerifyInput, ...func(*kms.Options)) (*kms.VerifyOutput, error) {
				return &kms.VerifyOutput{SignatureValid: false}, nil
			},
		}
	}
	if _, err := asymmetricRepository.RSA_OAEP_Encode("", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() key id error")
	}
	viper.Set(defaultKMSARNKey, "arn:aws:kms:test")
	if _, err := asymmetricRepository.RSA_OAEP_Encode("", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() provider error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode("", "%%%"); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() decode error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode("", base64.StdEncoding.EncodeToString([]byte("cipher"))); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() provider error")
	}

	if _, _, err := signatureRepository.GeneratesEd255Key(common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesEd255Key() error")
	}
	if _, err := signatureRepository.SignEd25519("", "payload"); err == nil {
		t.Fatal("expected SignEd25519() error")
	}
	if err := signatureRepository.VerifyEd25519("", "payload", "sig"); err == nil {
		t.Fatal("expected VerifyEd25519() error")
	}
	if _, err := signatureRepository.SignRSAPSS("", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() provider error")
	}
	if err := signatureRepository.VerifyRSAPSS("", "payload", "%%%"); err == nil {
		t.Fatal("expected VerifyRSAPSS() decode error")
	}
	if err := signatureRepository.VerifyRSAPSS("", "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected VerifyRSAPSS() invalid signature error")
	}
	if _, err := signatureRepository.SignSHA256("payload", nil); err == nil {
		t.Fatal("expected SignSHA256() provider error")
	}
	if err := signatureRepository.VerifySHA256("payload", "%%%", nil); err == nil {
		t.Fatal("expected VerifySHA256() decode error")
	}
	if err := signatureRepository.VerifySHA256("payload", base64.StdEncoding.EncodeToString([]byte("sig")), nil); err == nil {
		t.Fatal("expected VerifySHA256() invalid signature error")
	}

	privateKey := mustRSAKey(t)
	publicKey := &privateKey.PublicKey
	publicB64 := mustMarshalPKIXRSAPublicKey(t, publicKey)
	privateB64 := mustMarshalPKCS8RSAPrivateKey(t, privateKey)

	if _, err := asymmetricRepository.RSA_OAEP_Encode(publicB64, "payload"); err != nil {
		t.Fatalf("RSA_OAEP_Encode() local fallback error = %v", err)
	}
	ciphertext, err := asymmetricRepository.RSA_OAEP_Encode(publicB64, "payload")
	if err != nil {
		t.Fatalf("RSA_OAEP_Encode() local fallback error = %v", err)
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode(privateB64, ciphertext); err != nil {
		t.Fatalf("RSA_OAEP_Decode() local fallback error = %v", err)
	}
	signature, err := signatureRepository.SignRSAPSS(privateB64, "payload")
	if err != nil {
		t.Fatalf("SignRSAPSS() local fallback error = %v", err)
	}
	if err := signatureRepository.VerifyRSAPSS(publicB64, "payload", signature); err != nil {
		t.Fatalf("VerifyRSAPSS() local fallback error = %v", err)
	}
	signature, err = signatureRepository.SignSHA256("payload", privateKey)
	if err != nil {
		t.Fatalf("SignSHA256() local fallback error = %v", err)
	}
	if err := signatureRepository.VerifySHA256("payload", signature, publicKey); err != nil {
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

	asymmetricRepository := NewAsymmetricRepository()
	signatureRepository := NewSignatureRepository()

	if _, _, err := asymmetricRepository.GeneratesRSAKey(common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesRSAKey() config error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Encode("arn", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() config error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode("arn", base64.StdEncoding.EncodeToString([]byte("cipher"))); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() config error")
	}
	if _, err := signatureRepository.SignRSAPSS("arn", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() config error")
	}
	if err := signatureRepository.VerifyRSAPSS("arn", "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected VerifyRSAPSS() config error")
	}
	if _, err := signatureRepository.SignSHA256("payload", nil); err == nil {
		t.Fatal("expected SignSHA256() config error")
	}
	if err := signatureRepository.VerifySHA256("payload", base64.StdEncoding.EncodeToString([]byte("sig")), nil); err == nil {
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
