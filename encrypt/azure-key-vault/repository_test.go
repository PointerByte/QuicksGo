// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package azurekeyvault

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/PointerByte/QuicksGo/encrypt/common"
	"github.com/PointerByte/QuicksGo/encrypt/local"
	"github.com/PointerByte/QuicksGo/encrypt/utilities"
	"github.com/spf13/viper"
)

type fakeTokenCredential struct{}

func (fakeTokenCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{}, nil
}

type fakeAzureKeysClient struct {
	createKeyFn func(context.Context, string, azkeys.CreateKeyParameters, *azkeys.CreateKeyOptions) (azkeys.CreateKeyResponse, error)
	encryptFn   func(context.Context, string, string, azkeys.KeyOperationParameters, *azkeys.EncryptOptions) (azkeys.EncryptResponse, error)
	decryptFn   func(context.Context, string, string, azkeys.KeyOperationParameters, *azkeys.DecryptOptions) (azkeys.DecryptResponse, error)
	signFn      func(context.Context, string, string, azkeys.SignParameters, *azkeys.SignOptions) (azkeys.SignResponse, error)
	verifyFn    func(context.Context, string, string, azkeys.VerifyParameters, *azkeys.VerifyOptions) (azkeys.VerifyResponse, error)
}

func (fake fakeAzureKeysClient) CreateKey(ctx context.Context, name string, parameters azkeys.CreateKeyParameters, options *azkeys.CreateKeyOptions) (azkeys.CreateKeyResponse, error) {
	return fake.createKeyFn(ctx, name, parameters, options)
}
func (fake fakeAzureKeysClient) Encrypt(ctx context.Context, name, version string, parameters azkeys.KeyOperationParameters, options *azkeys.EncryptOptions) (azkeys.EncryptResponse, error) {
	return fake.encryptFn(ctx, name, version, parameters, options)
}
func (fake fakeAzureKeysClient) Decrypt(ctx context.Context, name, version string, parameters azkeys.KeyOperationParameters, options *azkeys.DecryptOptions) (azkeys.DecryptResponse, error) {
	return fake.decryptFn(ctx, name, version, parameters, options)
}
func (fake fakeAzureKeysClient) Sign(ctx context.Context, name, version string, parameters azkeys.SignParameters, options *azkeys.SignOptions) (azkeys.SignResponse, error) {
	return fake.signFn(ctx, name, version, parameters, options)
}
func (fake fakeAzureKeysClient) Verify(ctx context.Context, name, version string, parameters azkeys.VerifyParameters, options *azkeys.VerifyOptions) (azkeys.VerifyResponse, error) {
	return fake.verifyFn(ctx, name, version, parameters, options)
}

func TestAzureRepositoryProviderFlowsAndHelpers(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousCredential := newAzureCredentialFn
	previousClient := newAzureClientFn
	t.Cleanup(func() {
		newAzureCredentialFn = previousCredential
		newAzureClientFn = previousClient
	})

	privateKey := mustAzureRSAKey(t)
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

	newAzureCredentialFn = func(*azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return fakeTokenCredential{}, nil
	}
	newAzureClientFn = func(string, azcore.TokenCredential) (azureKeysClient, error) {
		return fakeAzureKeysClient{
			createKeyFn: func(_ context.Context, name string, parameters azkeys.CreateKeyParameters, _ *azkeys.CreateKeyOptions) (azkeys.CreateKeyResponse, error) {
				kid := azkeys.ID("https://vault.test/keys/" + name + "/v1")
				key := &azkeys.JSONWebKey{KID: &kid}
				if parameters.Kty != nil && *parameters.Kty == azkeys.KeyTypeRSA {
					key.N = privateKey.PublicKey.N.Bytes()
					key.E = []byte{0x01, 0x00, 0x01}
				}
				return azkeys.CreateKeyResponse{KeyBundle: azkeys.KeyBundle{Key: key}}, nil
			},
			encryptFn: func(_ context.Context, _, _ string, parameters azkeys.KeyOperationParameters, _ *azkeys.EncryptOptions) (azkeys.EncryptResponse, error) {
				if parameters.Algorithm != nil && *parameters.Algorithm == azkeys.EncryptionAlgorithmA256GCM {
					return azkeys.EncryptResponse{KeyOperationResult: azkeys.KeyOperationResult{Result: []byte("cipher"), IV: []byte("iv"), AuthenticationTag: []byte("tag")}}, nil
				}
				return azkeys.EncryptResponse{KeyOperationResult: azkeys.KeyOperationResult{Result: []byte("rsa-cipher")}}, nil
			},
			decryptFn: func(_ context.Context, _, _ string, parameters azkeys.KeyOperationParameters, _ *azkeys.DecryptOptions) (azkeys.DecryptResponse, error) {
				if parameters.Algorithm != nil && *parameters.Algorithm == azkeys.EncryptionAlgorithmA256GCM {
					return azkeys.DecryptResponse{KeyOperationResult: azkeys.KeyOperationResult{Result: []byte("hello")}}, nil
				}
				return azkeys.DecryptResponse{KeyOperationResult: azkeys.KeyOperationResult{Result: []byte("plain")}}, nil
			},
			signFn: func(_ context.Context, _, _ string, parameters azkeys.SignParameters, _ *azkeys.SignOptions) (azkeys.SignResponse, error) {
				return azkeys.SignResponse{KeyOperationResult: azkeys.KeyOperationResult{Result: append([]byte("sig-"), parameters.Value...)}}, nil
			},
			verifyFn: func(_ context.Context, _, _ string, _ azkeys.VerifyParameters, _ *azkeys.VerifyOptions) (azkeys.VerifyResponse, error) {
				valid := true
				return azkeys.VerifyResponse{KeyVerifyResult: azkeys.KeyVerifyResult{Value: &valid}}, nil
			},
		}, nil
	}

	viper.Set(defaultAzureVaultURLKey, "https://vault.test")
	viper.Set(defaultAzureKeyIDKey, "https://vault.test/keys/default-key/v1")
	repository := NewRepository()

	symmetricKey, err := repository.GeneratesSymetrycKey(context.Background(), common.Key256Bits)
	if err != nil || symmetricKey == nil || symmetricKey.Provider != azureProviderName {
		t.Fatalf("GeneratesSymetrycKey() = %#v, %v", symmetricKey, err)
	}
	additional := "aad"
	ciphertext, err := repository.EncryptAES(context.Background(), symmetricKey.KeyRef, "hello", &additional)
	if err != nil {
		t.Fatalf("EncryptAES() error = %v", err)
	}
	plaintext, err := repository.DecryptAES(context.Background(), symmetricKey.KeyRef, ciphertext, &additional)
	if err != nil || plaintext != "hello" {
		t.Fatalf("DecryptAES() = %q, %v", plaintext, err)
	}
	if got := repository.GenerateHMAC(context.Background(), symmetricKey.KeyRef, "message"); got == "" {
		t.Fatal("expected GenerateHMAC() to return a value")
	}
	if !repository.ValidateHMAC(context.Background(), symmetricKey.KeyRef, "message", base64.StdEncoding.EncodeToString([]byte("mac"))) {
		t.Fatal("expected ValidateHMAC() to succeed")
	}
	if repository.Sha256Hex(context.Background(), "message") == "" || repository.Blake3(context.Background(), "message") == "" {
		t.Fatal("expected hash helpers to return values")
	}

	rsaKey, err := repository.GeneratesRSAKey(context.Background(), common.Key2048Bits)
	if err != nil || rsaKey == nil || rsaKey.PublicKey == "" {
		t.Fatalf("GeneratesRSAKey() = %#v, %v", rsaKey, err)
	}
	if _, err := repository.RSA_OAEP_Encode(context.Background(), rsaKey.KeyRef, "payload"); err != nil {
		t.Fatalf("RSA_OAEP_Encode() error = %v", err)
	}
	if plaintext, err := repository.RSA_OAEP_Decode(context.Background(), rsaKey.KeyRef, base64.StdEncoding.EncodeToString([]byte("cipher"))); err != nil || plaintext != "plain" {
		t.Fatalf("RSA_OAEP_Decode() = %q, %v", plaintext, err)
	}
	if _, err := repository.SignRSAPSS(context.Background(), rsaKey.KeyRef, "payload"); err != nil {
		t.Fatalf("SignRSAPSS() error = %v", err)
	}
	if err := repository.VerifyRSAPSS(context.Background(), rsaKey.KeyRef, "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err != nil {
		t.Fatalf("VerifyRSAPSS() error = %v", err)
	}
	if _, err := repository.SignPKCS1v15_SHA256(context.Background(), "payload", nil); err != nil {
		t.Fatalf("SignPKCS1v15_SHA256() error = %v", err)
	}
	if err := repository.VerifySHA256(context.Background(), "payload", base64.StdEncoding.EncodeToString([]byte("sig")), nil); err != nil {
		t.Fatalf("VerifySHA256() error = %v", err)
	}
	if _, err := repository.GeneratesEd255Key(context.Background(), common.Key2048Bits); !errors.Is(err, errAzureEd25519Unsupported) {
		t.Fatalf("GeneratesEd255Key() error = %v", err)
	}
	if _, err := repository.GeneratesECCKey(context.Background(), common.CurveP256); !errors.Is(err, errAzureECCUnsupported) {
		t.Fatalf("GeneratesECCKey() error = %v", err)
	}

	localRepository := local.NewRepository()
	localSymmetricKey, err := localRepository.GeneratesSymetrycKey(context.Background(), common.Key256Bits)
	if err != nil {
		t.Fatalf("local GeneratesSymetrycKey() error = %v", err)
	}
	if _, err := repository.EncryptAES(context.Background(), localSymmetricKey.KeyID, "hello", &additional); err != nil {
		t.Fatalf("EncryptAES() local fallback error = %v", err)
	}
	localCiphertext, err := localRepository.EncryptAES(context.Background(), localSymmetricKey.KeyID, "hello", &additional)
	if err != nil {
		t.Fatalf("local EncryptAES() error = %v", err)
	}
	if _, err := repository.DecryptAES(context.Background(), localSymmetricKey.KeyID, localCiphertext, &additional); err != nil {
		t.Fatalf("DecryptAES() local fallback error = %v", err)
	}
	localMac := repository.GenerateHMAC(context.Background(), "secret", "message")
	if localMac == "" || !repository.ValidateHMAC(context.Background(), "secret", "message", localMac) {
		t.Fatal("expected local HMAC fallback to succeed")
	}
	localRSAPrivate := mustAzureRSAPrivateBase64(t, privateKey)
	localRSAPublic := mustAzureRSAPublicBase64(t, &privateKey.PublicKey)
	localRSACiphertext, err := repository.RSA_OAEP_Encode(context.Background(), localRSAPublic, "payload")
	if err != nil {
		t.Fatalf("RSA_OAEP_Encode() local fallback error = %v", err)
	}
	if _, err := repository.RSA_OAEP_Decode(context.Background(), localRSAPrivate, localRSACiphertext); err != nil {
		t.Fatalf("RSA_OAEP_Decode() local fallback error = %v", err)
	}
	localECCPrivate := mustAzureECCPrivateBase64(t, ecdh.P256())
	localECCPublic := mustAzureECCPublicBase64(t, localECCPrivate)
	localEccCiphertext, err := repository.ECC_Encode(context.Background(), localECCPublic, "payload")
	if err != nil {
		t.Fatalf("ECC_Encode() local fallback error = %v", err)
	}
	if plaintext, err := repository.ECC_Decode(context.Background(), localECCPrivate, localEccCiphertext); err != nil || plaintext != "payload" {
		t.Fatalf("ECC_Decode() local fallback = %q, %v", plaintext, err)
	}
	localPSSSignature, err := repository.SignRSAPSS(context.Background(), localRSAPrivate, "payload")
	if err != nil {
		t.Fatalf("SignRSAPSS() local fallback error = %v", err)
	}
	if err := repository.VerifyRSAPSS(context.Background(), localRSAPublic, "payload", localPSSSignature); err != nil {
		t.Fatalf("VerifyRSAPSS() local fallback error = %v", err)
	}
	localSHA256Signature, err := repository.SignPKCS1v15_SHA256(context.Background(), "payload", privateKey)
	if err != nil {
		t.Fatalf("SignPKCS1v15_SHA256() local fallback error = %v", err)
	}
	if err := repository.VerifySHA256(context.Background(), "payload", localSHA256Signature, &privateKey.PublicKey); err != nil {
		t.Fatalf("VerifySHA256() local fallback error = %v", err)
	}
	localEdPrivate := base64.StdEncoding.EncodeToString(edPrivateDER)
	localEdPublic := base64.StdEncoding.EncodeToString(edPublicDER)
	localEdSignature, err := repository.SignEd25519(context.Background(), localEdPrivate, "payload")
	if err != nil {
		t.Fatalf("SignEd25519() local fallback error = %v", err)
	}
	if err := repository.VerifyEd25519(context.Background(), localEdPublic, "payload", localEdSignature); err != nil {
		t.Fatalf("VerifyEd25519() local fallback error = %v", err)
	}

	if got, err := resolveAzureVaultURL(); err != nil || got != "https://vault.test" {
		t.Fatalf("resolveAzureVaultURL() = %q, %v", got, err)
	}
	if got := configuredAzureKeyID(); got == "" {
		t.Fatal("expected configuredAzureKeyID() value")
	}
	if !looksLikeAzureKeyReference(symmetricKey.KeyRef) || looksLikeAzureKeyReference("local") {
		t.Fatal("unexpected looksLikeAzureKeyReference() result")
	}
	if got := utilities.BytesFromOptionalString(nil); got != nil {
		t.Fatal("expected utilities.BytesFromOptionalString(nil) to return nil")
	}
	if boolValue(nil) || !boolValue(ptr(true)) {
		t.Fatal("unexpected boolValue() result")
	}
	if _, _, err := azureMetadataFromBundle(azkeys.KeyBundle{}, "", "name"); err == nil {
		t.Fatal("expected azureMetadataFromBundle() error")
	}
	if _, err := rsaPublicKeyFromAzureBundle(azkeys.KeyBundle{}); err == nil {
		t.Fatal("expected rsaPublicKeyFromAzureBundle() error")
	}
	if utilities.IsLocalAESKey("%%%") {
		t.Fatal("expected utilities.IsLocalAESKey() false for invalid base64")
	}

	_ = publicDER
}

func TestAzureRepositoryErrorBranches(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousCredential := newAzureCredentialFn
	previousClient := newAzureClientFn
	t.Cleanup(func() {
		newAzureCredentialFn = previousCredential
		newAzureClientFn = previousClient
	})

	if _, err := NewSymmetricRepository().GeneratesSymetrycKey(context.Background(), common.Key128Bits); err == nil {
		t.Fatal("expected unsupported symmetric size error")
	}
	if _, err := NewAsymmetricRepository().GeneratesRSAKey(context.Background(), 0); err == nil {
		t.Fatal("expected unsupported rsa size error")
	}
	if _, err := resolveAzureKeyReference(""); err == nil {
		t.Fatal("expected resolveAzureKeyReference() error")
	}
	if _, err := resolveAzureKeyReference("not-a-url"); err == nil {
		t.Fatal("expected resolveAzureKeyReference() invalid URL error")
	}

	newAzureCredentialFn = func(*azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return nil, errors.New("credential boom")
	}
	if _, _, err := newAzureKeysClient(context.Background(), "https://vault.test"); err == nil {
		t.Fatal("expected newAzureKeysClient() credential error")
	}

	newAzureCredentialFn = func(*azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return fakeTokenCredential{}, nil
	}
	newAzureClientFn = func(string, azcore.TokenCredential) (azureKeysClient, error) {
		return fakeAzureKeysClient{
			createKeyFn: func(context.Context, string, azkeys.CreateKeyParameters, *azkeys.CreateKeyOptions) (azkeys.CreateKeyResponse, error) {
				return azkeys.CreateKeyResponse{}, errors.New("create boom")
			},
			encryptFn: func(context.Context, string, string, azkeys.KeyOperationParameters, *azkeys.EncryptOptions) (azkeys.EncryptResponse, error) {
				return azkeys.EncryptResponse{}, errors.New("encrypt boom")
			},
			decryptFn: func(context.Context, string, string, azkeys.KeyOperationParameters, *azkeys.DecryptOptions) (azkeys.DecryptResponse, error) {
				return azkeys.DecryptResponse{}, errors.New("decrypt boom")
			},
			signFn: func(context.Context, string, string, azkeys.SignParameters, *azkeys.SignOptions) (azkeys.SignResponse, error) {
				return azkeys.SignResponse{}, errors.New("sign boom")
			},
			verifyFn: func(context.Context, string, string, azkeys.VerifyParameters, *azkeys.VerifyOptions) (azkeys.VerifyResponse, error) {
				return azkeys.VerifyResponse{}, errors.New("verify boom")
			},
		}, nil
	}

	viper.Set(defaultAzureVaultURLKey, "https://vault.test")
	viper.Set(defaultAzureKeyIDKey, "https://vault.test/keys/default-key/v1")
	symmetricRepository := NewSymmetricRepository()
	hashRepository := NewHashRepository()
	asymmetricRepository := NewAsymmetricRepository()
	signatureRepository := NewSignatureRepository()

	if _, err := symmetricRepository.GeneratesSymetrycKey(context.Background(), common.Key256Bits); err == nil {
		t.Fatal("expected GeneratesSymetrycKey() provider error")
	}
	if _, err := symmetricRepository.EncryptAES(context.Background(), "", "payload", nil); err == nil {
		t.Fatal("expected EncryptAES() key reference error")
	}
	if _, err := symmetricRepository.EncryptAES(context.Background(), "https://vault.test/keys/default-key/v1", "payload", nil); err == nil {
		t.Fatal("expected EncryptAES() provider error")
	}
	if _, err := symmetricRepository.DecryptAES(context.Background(), "https://vault.test/keys/default-key/v1", "%%%", nil); err == nil {
		t.Fatal("expected DecryptAES() payload decode error")
	}
	invalidJSON := base64.StdEncoding.EncodeToString([]byte("{"))
	if _, err := symmetricRepository.DecryptAES(context.Background(), "https://vault.test/keys/default-key/v1", invalidJSON, nil); err == nil {
		t.Fatal("expected DecryptAES() json decode error")
	}
	payloadBytes, err := json.Marshal(azureAEADPayload{Result: "%%%", IV: base64.StdEncoding.EncodeToString([]byte("iv")), Tag: base64.StdEncoding.EncodeToString([]byte("tag"))})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if _, err := symmetricRepository.DecryptAES(context.Background(), "https://vault.test/keys/default-key/v1", base64.StdEncoding.EncodeToString(payloadBytes), nil); err == nil {
		t.Fatal("expected DecryptAES() ciphertext decode error")
	}
	if got := hashRepository.GenerateHMAC(context.Background(), "https://vault.test/keys/default-key/v1", "message"); got != "" {
		t.Fatalf("GenerateHMAC() = %q, want empty string on provider error", got)
	}
	if hashRepository.ValidateHMAC(context.Background(), "https://vault.test/keys/default-key/v1", "message", "%%%") {
		t.Fatal("expected ValidateHMAC() to fail on invalid signature")
	}
	if hashRepository.ValidateHMAC(context.Background(), "https://vault.test/keys/default-key/v1", "message", base64.StdEncoding.EncodeToString([]byte("sig"))) {
		t.Fatal("expected ValidateHMAC() to fail on provider error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Encode(context.Background(), "", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() key reference error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Encode(context.Background(), "https://vault.test/keys/default-key/v1", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() provider error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode(context.Background(), "https://vault.test/keys/default-key/v1", "%%%"); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() decode error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode(context.Background(), "https://vault.test/keys/default-key/v1", base64.StdEncoding.EncodeToString([]byte("cipher"))); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() provider error")
	}
	if _, err := asymmetricRepository.GeneratesECCKey(context.Background(), common.CurveP256); !errors.Is(err, errAzureECCUnsupported) {
		t.Fatalf("GeneratesECCKey() error = %v", err)
	}
	if _, err := asymmetricRepository.ECC_Encode(context.Background(), "https://vault.test/keys/default-key/v1", "payload"); !errors.Is(err, errAzureECCUnsupported) {
		t.Fatalf("ECC_Encode() error = %v", err)
	}
	if _, err := asymmetricRepository.ECC_Decode(context.Background(), "https://vault.test/keys/default-key/v1", "payload"); !errors.Is(err, errAzureECCUnsupported) {
		t.Fatalf("ECC_Decode() error = %v", err)
	}
	if _, err := signatureRepository.SignEd25519(context.Background(), "https://vault.test/keys/default-key/v1", "payload"); !errors.Is(err, errAzureEd25519Unsupported) {
		t.Fatalf("SignEd25519() error = %v", err)
	}
	if err := signatureRepository.VerifyEd25519(context.Background(), "https://vault.test/keys/default-key/v1", "payload", "sig"); !errors.Is(err, errAzureEd25519Unsupported) {
		t.Fatalf("VerifyEd25519() error = %v", err)
	}
	if _, err := signatureRepository.SignRSAPSS(context.Background(), "", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() key reference error")
	}
	if _, err := signatureRepository.SignRSAPSS(context.Background(), "https://vault.test/keys/default-key/v1", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() provider error")
	}
	if err := signatureRepository.VerifyRSAPSS(context.Background(), "https://vault.test/keys/default-key/v1", "payload", "%%%"); err == nil {
		t.Fatal("expected VerifyRSAPSS() decode error")
	}
	if err := signatureRepository.VerifyRSAPSS(context.Background(), "https://vault.test/keys/default-key/v1", "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected VerifyRSAPSS() provider error")
	}
	if _, err := signatureRepository.SignPKCS1v15_SHA256(context.Background(), "payload", nil); err == nil {
		t.Fatal("expected SignPKCS1v15_SHA256() provider error")
	}
	if err := signatureRepository.VerifySHA256(context.Background(), "payload", "%%%", nil); err == nil {
		t.Fatal("expected VerifySHA256() decode error")
	}
	if got, err := resolveAzureKeyReference("https://vault.test/keys/name/version"); err != nil || got.Name != "name" || got.Version != "version" {
		t.Fatalf("resolveAzureKeyReference() = %#v, %v", got, err)
	}
	viper.Reset()
	viper.Set(defaultAzureKeyIDKey, "https://vault.test/keys/from-config/v2")
	if got, err := resolveAzureVaultURL(); err != nil || got != "https://vault.test" {
		t.Fatalf("resolveAzureVaultURL() from key id = %q, %v", got, err)
	}
	viper.Reset()
	viper.Set(legacyAzureVaultURLKey, "https://legacy.vault")
	if got, err := resolveAzureVaultURL(); err != nil || got != "https://legacy.vault" {
		t.Fatalf("resolveAzureVaultURL() legacy = %q, %v", got, err)
	}
	viper.Reset()
	viper.Set(legacyAzureKeyIDKey, "https://vault.test/keys/legacy/v1")
	if got := configuredAzureKeyID(); got != "https://vault.test/keys/legacy/v1" {
		t.Fatalf("configuredAzureKeyID() = %q", got)
	}
	viper.Reset()
	if looksLikeAzureKeyReference("") {
		t.Fatal("expected looksLikeAzureKeyReference(\"\") to be false without config")
	}
	if _, err := azureRSAKeySize(common.Key3072Bits); err != nil {
		t.Fatalf("azureRSAKeySize(3072) error = %v", err)
	}
	if _, err := azureRSAKeySize(common.Key4096Bits); err != nil {
		t.Fatalf("azureRSAKeySize(4096) error = %v", err)
	}
	kid := azkeys.ID("https://vault.test/keys/name/version")
	if gotID, gotRef, err := azureMetadataFromBundle(azkeys.KeyBundle{Key: &azkeys.JSONWebKey{KID: &kid}}, "", "ignored"); err != nil || gotID != "name" || gotRef == "" {
		t.Fatalf("azureMetadataFromBundle() = %q, %q, %v", gotID, gotRef, err)
	}
	if gotID, gotRef, err := azureMetadataFromBundle(azkeys.KeyBundle{}, "https://vault.test", "name"); err != nil || gotID != "name" || gotRef != "https://vault.test/keys/name" {
		t.Fatalf("azureMetadataFromBundle() fallback = %q, %q, %v", gotID, gotRef, err)
	}
}

func TestAzureRepositoryAdditionalErrorBranches(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousCredential := newAzureCredentialFn
	previousClient := newAzureClientFn
	t.Cleanup(func() {
		newAzureCredentialFn = previousCredential
		newAzureClientFn = previousClient
	})

	newAzureCredentialFn = func(*azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return fakeTokenCredential{}, nil
	}
	newAzureClientFn = func(string, azcore.TokenCredential) (azureKeysClient, error) {
		return nil, errors.New("client boom")
	}
	if _, _, err := newAzureKeysClient(context.Background(), "https://vault.test"); err == nil {
		t.Fatal("expected newAzureKeysClient() client error")
	}

	newAzureClientFn = func(string, azcore.TokenCredential) (azureKeysClient, error) {
		return fakeAzureKeysClient{
			createKeyFn: func(context.Context, string, azkeys.CreateKeyParameters, *azkeys.CreateKeyOptions) (azkeys.CreateKeyResponse, error) {
				return azkeys.CreateKeyResponse{}, nil
			},
			encryptFn: func(context.Context, string, string, azkeys.KeyOperationParameters, *azkeys.EncryptOptions) (azkeys.EncryptResponse, error) {
				return azkeys.EncryptResponse{}, nil
			},
			decryptFn: func(context.Context, string, string, azkeys.KeyOperationParameters, *azkeys.DecryptOptions) (azkeys.DecryptResponse, error) {
				return azkeys.DecryptResponse{}, errors.New("decrypt boom")
			},
			signFn: func(context.Context, string, string, azkeys.SignParameters, *azkeys.SignOptions) (azkeys.SignResponse, error) {
				return azkeys.SignResponse{KeyOperationResult: azkeys.KeyOperationResult{Result: []byte("sig")}}, nil
			},
			verifyFn: func(context.Context, string, string, azkeys.VerifyParameters, *azkeys.VerifyOptions) (azkeys.VerifyResponse, error) {
				valid := false
				return azkeys.VerifyResponse{KeyVerifyResult: azkeys.KeyVerifyResult{Value: &valid}}, nil
			},
		}, nil
	}

	viper.Set(defaultAzureVaultURLKey, "https://vault.test")
	viper.Set(defaultAzureKeyIDKey, "https://vault.test/keys/default-key/v1")
	symmetricRepository := NewSymmetricRepository()
	signatureRepository := NewSignatureRepository()

	payload, err := json.Marshal(azureAEADPayload{
		Result: base64.StdEncoding.EncodeToString([]byte("cipher")),
		IV:     "%%%",
		Tag:    base64.StdEncoding.EncodeToString([]byte("tag")),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if _, err := symmetricRepository.DecryptAES(context.Background(), "https://vault.test/keys/default-key/v1", base64.StdEncoding.EncodeToString(payload), nil); err == nil {
		t.Fatal("expected DecryptAES() iv decode error")
	}
	payload, err = json.Marshal(azureAEADPayload{
		Result: base64.StdEncoding.EncodeToString([]byte("cipher")),
		IV:     base64.StdEncoding.EncodeToString([]byte("iv")),
		Tag:    "%%%",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if _, err := symmetricRepository.DecryptAES(context.Background(), "https://vault.test/keys/default-key/v1", base64.StdEncoding.EncodeToString(payload), nil); err == nil {
		t.Fatal("expected DecryptAES() tag decode error")
	}
	payload, err = json.Marshal(azureAEADPayload{
		Result: base64.StdEncoding.EncodeToString([]byte("cipher")),
		IV:     base64.StdEncoding.EncodeToString([]byte("iv")),
		Tag:    base64.StdEncoding.EncodeToString([]byte("tag")),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if _, err := symmetricRepository.DecryptAES(context.Background(), "https://vault.test/keys/default-key/v1", base64.StdEncoding.EncodeToString(payload), nil); err == nil {
		t.Fatal("expected DecryptAES() provider error")
	}
	if err := signatureRepository.VerifyRSAPSS(context.Background(), "https://vault.test/keys/default-key/v1", "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected VerifyRSAPSS() invalid signature error")
	}
	if err := signatureRepository.VerifySHA256(context.Background(), "payload", base64.StdEncoding.EncodeToString([]byte("sig")), nil); err == nil {
		t.Fatal("expected VerifySHA256() invalid signature error")
	}
}

func TestAzureRepositoryMetadataFallbackPaths(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousCredential := newAzureCredentialFn
	previousClient := newAzureClientFn
	t.Cleanup(func() {
		newAzureCredentialFn = previousCredential
		newAzureClientFn = previousClient
	})

	privateKey := mustAzureRSAKey(t)
	newAzureCredentialFn = func(*azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return fakeTokenCredential{}, nil
	}
	newAzureClientFn = func(string, azcore.TokenCredential) (azureKeysClient, error) {
		return fakeAzureKeysClient{
			createKeyFn: func(_ context.Context, _ string, parameters azkeys.CreateKeyParameters, _ *azkeys.CreateKeyOptions) (azkeys.CreateKeyResponse, error) {
				key := &azkeys.JSONWebKey{}
				if parameters.Kty != nil && *parameters.Kty == azkeys.KeyTypeRSA {
					key.N = privateKey.PublicKey.N.Bytes()
					key.E = []byte{0x01, 0x00, 0x01}
				}
				return azkeys.CreateKeyResponse{KeyBundle: azkeys.KeyBundle{Key: key}}, nil
			},
			encryptFn: func(context.Context, string, string, azkeys.KeyOperationParameters, *azkeys.EncryptOptions) (azkeys.EncryptResponse, error) {
				return azkeys.EncryptResponse{}, nil
			},
			decryptFn: func(context.Context, string, string, azkeys.KeyOperationParameters, *azkeys.DecryptOptions) (azkeys.DecryptResponse, error) {
				return azkeys.DecryptResponse{}, nil
			},
			signFn: func(context.Context, string, string, azkeys.SignParameters, *azkeys.SignOptions) (azkeys.SignResponse, error) {
				return azkeys.SignResponse{}, nil
			},
			verifyFn: func(context.Context, string, string, azkeys.VerifyParameters, *azkeys.VerifyOptions) (azkeys.VerifyResponse, error) {
				return azkeys.VerifyResponse{}, nil
			},
		}, nil
	}

	viper.Set(defaultAzureVaultURLKey, "https://vault.test")
	if key, err := NewSymmetricRepository().GeneratesSymetrycKey(context.Background(), common.Key256Bits); err != nil || key.KeyRef != "https://vault.test/keys/"+key.KeyID {
		t.Fatalf("GeneratesSymetrycKey() fallback metadata = %#v, %v", key, err)
	}
	if key, err := NewAsymmetricRepository().GeneratesRSAKey(context.Background(), common.Key2048Bits); err != nil || key.KeyRef != "https://vault.test/keys/"+key.KeyID {
		t.Fatalf("GeneratesRSAKey() fallback metadata = %#v, %v", key, err)
	}
}

func mustAzureRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	return privateKey
}

func mustAzureRSAPrivateBase64(t *testing.T, privateKey *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustAzureRSAPublicBase64(t *testing.T, publicKey *rsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustAzureECCPrivateBase64(t *testing.T, curve ecdh.Curve) string {
	t.Helper()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ecdh.GenerateKey() error = %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustAzureECCPublicBase64(t *testing.T, privateKeyBase64 string) string {
	t.Helper()
	privateKey, err := utilities.ParseECDHPrivateKeyFromBase64(privateKeyBase64)
	if err != nil {
		t.Fatalf("ParseECDHPrivateKeyFromBase64() error = %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(privateKey.PublicKey())
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}
