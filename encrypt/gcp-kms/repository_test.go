// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package gcpkms

import (
	"context"
	"crypto"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"testing"

	kms "cloud.google.com/go/kms/apiv1"
	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/PointerByte/QuicksGo/encrypt/common"
	"github.com/PointerByte/QuicksGo/encrypt/local"
	"github.com/PointerByte/QuicksGo/encrypt/utilities"
	"github.com/spf13/viper"
)

type fakeGCPClient struct {
	createCryptoKeyFn        func(context.Context, *kmspb.CreateCryptoKeyRequest) (*kmspb.CryptoKey, error)
	createCryptoKeyVersionFn func(context.Context, *kmspb.CreateCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error)
	getPublicKeyFn           func(context.Context, *kmspb.GetPublicKeyRequest) (*kmspb.PublicKey, error)
	encryptFn                func(context.Context, *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error)
	decryptFn                func(context.Context, *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error)
	asymmetricSignFn         func(context.Context, *kmspb.AsymmetricSignRequest) (*kmspb.AsymmetricSignResponse, error)
	asymmetricDecryptFn      func(context.Context, *kmspb.AsymmetricDecryptRequest) (*kmspb.AsymmetricDecryptResponse, error)
	macSignFn                func(context.Context, *kmspb.MacSignRequest) (*kmspb.MacSignResponse, error)
	macVerifyFn              func(context.Context, *kmspb.MacVerifyRequest) (*kmspb.MacVerifyResponse, error)
	closeFn                  func() error
}

func (fake fakeGCPClient) CreateCryptoKey(ctx context.Context, req *kmspb.CreateCryptoKeyRequest) (*kmspb.CryptoKey, error) {
	return fake.createCryptoKeyFn(ctx, req)
}
func (fake fakeGCPClient) CreateCryptoKeyVersion(ctx context.Context, req *kmspb.CreateCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error) {
	return fake.createCryptoKeyVersionFn(ctx, req)
}
func (fake fakeGCPClient) GetPublicKey(ctx context.Context, req *kmspb.GetPublicKeyRequest) (*kmspb.PublicKey, error) {
	return fake.getPublicKeyFn(ctx, req)
}
func (fake fakeGCPClient) Encrypt(ctx context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) {
	return fake.encryptFn(ctx, req)
}
func (fake fakeGCPClient) Decrypt(ctx context.Context, req *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) {
	return fake.decryptFn(ctx, req)
}
func (fake fakeGCPClient) AsymmetricSign(ctx context.Context, req *kmspb.AsymmetricSignRequest) (*kmspb.AsymmetricSignResponse, error) {
	return fake.asymmetricSignFn(ctx, req)
}
func (fake fakeGCPClient) AsymmetricDecrypt(ctx context.Context, req *kmspb.AsymmetricDecryptRequest) (*kmspb.AsymmetricDecryptResponse, error) {
	return fake.asymmetricDecryptFn(ctx, req)
}
func (fake fakeGCPClient) MacSign(ctx context.Context, req *kmspb.MacSignRequest) (*kmspb.MacSignResponse, error) {
	return fake.macSignFn(ctx, req)
}
func (fake fakeGCPClient) MacVerify(ctx context.Context, req *kmspb.MacVerifyRequest) (*kmspb.MacVerifyResponse, error) {
	return fake.macVerifyFn(ctx, req)
}
func (fake fakeGCPClient) Close() error {
	return fake.closeFn()
}

func TestGCPRepositoryProviderFlowsAndHelpers(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousClient := newGCPClientFn
	t.Cleanup(func() { newGCPClientFn = previousClient })

	privateKey := mustGCPRSAKey(t)
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

	newGCPClientFn = func(context.Context) (gcpKMSClient, error) {
		return fakeGCPClient{
			createCryptoKeyFn: func(_ context.Context, req *kmspb.CreateCryptoKeyRequest) (*kmspb.CryptoKey, error) {
				name := req.Parent + "/cryptoKeys/" + req.CryptoKeyId
				primary := &kmspb.CryptoKeyVersion{Name: name + "/cryptoKeyVersions/1"}
				if req.CryptoKey.GetPurpose() == kmspb.CryptoKey_ENCRYPT_DECRYPT {
					return &kmspb.CryptoKey{Name: name, Primary: primary}, nil
				}
				return &kmspb.CryptoKey{Name: name}, nil
			},
			createCryptoKeyVersionFn: func(_ context.Context, req *kmspb.CreateCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error) {
				return &kmspb.CryptoKeyVersion{Name: req.Parent + "/cryptoKeyVersions/1"}, nil
			},
			getPublicKeyFn: func(_ context.Context, req *kmspb.GetPublicKeyRequest) (*kmspb.PublicKey, error) {
				if req.Name == "projects/test/locations/global/keyRings/ring/cryptoKeys/ed/cryptoKeyVersions/1" {
					return &kmspb.PublicKey{Pem: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: edPublicDER}))}, nil
				}
				return &kmspb.PublicKey{Pem: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))}, nil
			},
			encryptFn: func(_ context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) {
				return &kmspb.EncryptResponse{Ciphertext: []byte("cipher")}, nil
			},
			decryptFn: func(_ context.Context, req *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) {
				return &kmspb.DecryptResponse{Plaintext: []byte("hello")}, nil
			},
			asymmetricSignFn: func(_ context.Context, req *kmspb.AsymmetricSignRequest) (*kmspb.AsymmetricSignResponse, error) {
				if len(req.Data) > 0 {
					return &kmspb.AsymmetricSignResponse{Signature: ed25519.Sign(edPrivate, req.Data)}, nil
				}
				hashed := req.Digest.GetSha256()
				var (
					signature []byte
					err       error
				)
				if req.Name == "projects/test/locations/global/keyRings/ring/cryptoKeys/rsa-pss/cryptoKeyVersions/1" {
					signature, err = rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, hashed, nil)
				} else {
					signature, err = rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed)
				}
				if err != nil {
					return nil, err
				}
				return &kmspb.AsymmetricSignResponse{Signature: signature}, nil
			},
			asymmetricDecryptFn: func(_ context.Context, req *kmspb.AsymmetricDecryptRequest) (*kmspb.AsymmetricDecryptResponse, error) {
				return &kmspb.AsymmetricDecryptResponse{Plaintext: []byte("plain")}, nil
			},
			macSignFn: func(_ context.Context, req *kmspb.MacSignRequest) (*kmspb.MacSignResponse, error) {
				return &kmspb.MacSignResponse{Mac: []byte("mac")}, nil
			},
			macVerifyFn: func(_ context.Context, req *kmspb.MacVerifyRequest) (*kmspb.MacVerifyResponse, error) {
				return &kmspb.MacVerifyResponse{Success: true}, nil
			},
			closeFn: func() error { return nil },
		}, nil
	}

	viper.Set(defaultGCPKeyIDKey, "projects/test/locations/global/keyRings/ring/cryptoKeys/default/cryptoKeyVersions/1")
	repository := NewRepository()
	keyName := "projects/test/locations/global/keyRings/ring/cryptoKeys/sym"
	additional := "aad"

	key, err := repository.GeneratesSymetrycKey(context.Background(), common.Key256Bits)
	if err != nil || key == nil || key.Provider != gcpProviderName {
		t.Fatalf("GeneratesSymetrycKey() = %#v, %v", key, err)
	}
	ciphertext, err := repository.EncryptAES(context.Background(), keyName, "hello", &additional)
	if err != nil {
		t.Fatalf("EncryptAES() error = %v", err)
	}
	if plaintext, err := repository.DecryptAES(context.Background(), keyName, ciphertext, &additional); err != nil || plaintext != "hello" {
		t.Fatalf("DecryptAES() = %q, %v", plaintext, err)
	}
	if got := repository.GenerateHMAC(context.Background(), viper.GetString(defaultGCPKeyIDKey), "message"); got == "" {
		t.Fatal("expected GenerateHMAC() to return a value")
	}
	if !repository.ValidateHMAC(context.Background(), viper.GetString(defaultGCPKeyIDKey), "message", base64.StdEncoding.EncodeToString([]byte("mac"))) {
		t.Fatal("expected ValidateHMAC() to succeed")
	}
	if repository.Sha256Hex(context.Background(), "message") == "" || repository.Blake3(context.Background(), "message") == "" {
		t.Fatal("expected hash helpers to return values")
	}

	rsaKey, err := repository.GeneratesRSAKey(context.Background(), common.Key2048Bits)
	if err != nil || rsaKey == nil || rsaKey.PublicKey == "" {
		t.Fatalf("GeneratesRSAKey() = %#v, %v", rsaKey, err)
	}
	if _, err := repository.RSA_OAEP_Encode(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/rsa/cryptoKeyVersions/1", "payload"); err != nil {
		t.Fatalf("RSA_OAEP_Encode() error = %v", err)
	}
	if plaintext, err := repository.RSA_OAEP_Decode(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/rsa/cryptoKeyVersions/1", base64.StdEncoding.EncodeToString([]byte("cipher"))); err != nil || plaintext != "plain" {
		t.Fatalf("RSA_OAEP_Decode() = %q, %v", plaintext, err)
	}
	if _, err := repository.GeneratesEd255Key(context.Background(), common.Key2048Bits); err != nil {
		t.Fatalf("GeneratesEd255Key() error = %v", err)
	}
	if _, err := repository.GeneratesECCKey(context.Background(), common.CurveP256); !errors.Is(err, errGCPECCUnsupported) {
		t.Fatalf("GeneratesECCKey() error = %v", err)
	}
	edSignature, err := repository.SignEd25519(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/ed/cryptoKeyVersions/1", "payload")
	if err != nil {
		t.Fatalf("SignEd25519() error = %v", err)
	}
	if err := repository.VerifyEd25519(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/ed/cryptoKeyVersions/1", "payload", edSignature); err != nil {
		t.Fatalf("VerifyEd25519() error = %v", err)
	}
	rsaPSSSignature, err := repository.SignRSAPSS(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/rsa-pss/cryptoKeyVersions/1", "payload")
	if err != nil {
		t.Fatalf("SignRSAPSS() error = %v", err)
	}
	if err := repository.VerifyRSAPSS(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/rsa-pss/cryptoKeyVersions/1", "payload", rsaPSSSignature); err != nil {
		t.Fatalf("VerifyRSAPSS() error = %v", err)
	}
	rsaSignature, err := repository.SignPKCS1v15_SHA256(context.Background(), "payload", nil)
	if err != nil {
		t.Fatalf("SignPKCS1v15_SHA256() error = %v", err)
	}
	if err := repository.VerifySHA256(context.Background(), "payload", rsaSignature, nil); err != nil {
		t.Fatalf("VerifySHA256() error = %v", err)
	}

	localRepository := local.NewRepository()
	localSymmetricKey, err := localRepository.GeneratesSymetrycKey(context.Background(), common.Key256Bits)
	if err != nil {
		t.Fatalf("local GeneratesSymetrycKey() error = %v", err)
	}
	localCiphertext, err := localRepository.EncryptAES(context.Background(), localSymmetricKey.KeyID, "hello", &additional)
	if err != nil {
		t.Fatalf("local EncryptAES() error = %v", err)
	}
	if _, err := repository.DecryptAES(context.Background(), localSymmetricKey.KeyID, localCiphertext, &additional); err != nil {
		t.Fatalf("DecryptAES() local fallback error = %v", err)
	}
	localRSAPrivate := mustGCPRSAPrivateBase64(t, privateKey)
	localRSAPublic := mustGCPRSAPublicBase64(t, &privateKey.PublicKey)
	localRSACiphertext, err := repository.RSA_OAEP_Encode(context.Background(), localRSAPublic, "payload")
	if err != nil {
		t.Fatalf("RSA_OAEP_Encode() local fallback error = %v", err)
	}
	if _, err := repository.RSA_OAEP_Decode(context.Background(), localRSAPrivate, localRSACiphertext); err != nil {
		t.Fatalf("RSA_OAEP_Decode() local fallback error = %v", err)
	}
	localECCPrivate := mustGCPECCPrivateBase64(t, ecdh.P256())
	localECCPublic := mustGCPECCPublicBase64(t, localECCPrivate)
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

	if got := configuredGCPKeyID(); got == "" {
		t.Fatal("expected configuredGCPKeyID() value")
	}
	if got, err := resolveGCPKeyRingName("projects/test/locations/global/keyRings/ring/cryptoKeys/key"); err != nil || got != "projects/test/locations/global/keyRings/ring" {
		t.Fatalf("resolveGCPKeyRingName() = %q, %v", got, err)
	}
	if got, err := resolveGCPCryptoKeyName("projects/test/locations/global/keyRings/ring/cryptoKeys/key/cryptoKeyVersions/1"); err != nil || got != "projects/test/locations/global/keyRings/ring/cryptoKeys/key" {
		t.Fatalf("resolveGCPCryptoKeyName() = %q, %v", got, err)
	}
	if !looksLikeGCPKMSKeyReference("projects/test/locations/global/keyRings/ring/cryptoKeys/key") || looksLikeGCPKMSKeyReference("local") {
		t.Fatal("unexpected looksLikeGCPKMSKeyReference() result")
	}
	if got := utilities.BytesFromOptionalString(nil); got != nil {
		t.Fatal("expected utilities.BytesFromOptionalString(nil) to return nil")
	}
	if utilities.IsLocalAESKey("%%%") {
		t.Fatal("expected utilities.IsLocalAESKey() false for invalid base64")
	}
}

func TestGCPRepositoryErrorBranches(t *testing.T) {
	t.Cleanup(viper.Reset)
	previousClient := newGCPClientFn
	t.Cleanup(func() { newGCPClientFn = previousClient })

	if _, err := NewSymmetricRepository().GeneratesSymetrycKey(context.Background(), common.Key128Bits); err == nil {
		t.Fatal("expected unsupported symmetric size error")
	}
	if _, err := NewAsymmetricRepository().GeneratesRSAKey(context.Background(), 0); err == nil {
		t.Fatal("expected unsupported rsa size error")
	}
	if _, err := resolveGCPKeyRingName(""); err == nil {
		t.Fatal("expected resolveGCPKeyRingName() error")
	}
	if _, err := resolveGCPCryptoKeyName(""); err == nil {
		t.Fatal("expected resolveGCPCryptoKeyName() error")
	}
	if _, err := resolveGCPCryptoKeyVersionName("projects/test/locations/global/keyRings/ring/cryptoKeys/key"); err == nil {
		t.Fatal("expected resolveGCPCryptoKeyVersionName() error")
	}
	if _, err := gcpRSADecryptAlgorithm(0); err == nil {
		t.Fatal("expected gcpRSADecryptAlgorithm() error")
	}
	if _, err := gcpRSADecryptAlgorithm(common.Key3072Bits); err != nil {
		t.Fatalf("gcpRSADecryptAlgorithm(3072) error = %v", err)
	}
	if _, err := gcpRSADecryptAlgorithm(common.Key4096Bits); err != nil {
		t.Fatalf("gcpRSADecryptAlgorithm(4096) error = %v", err)
	}
	viper.Set(defaultGCPKeyIDKey, "projects/test/locations/global/keyRings/ring/cryptoKeys/default/cryptoKeyVersions/1")
	if !looksLikeGCPKMSKeyReference("") {
		t.Fatal("expected looksLikeGCPKMSKeyReference(\"\") to be true with config")
	}

	newGCPClientFn = func(context.Context) (gcpKMSClient, error) {
		return nil, errors.New("client boom")
	}
	if _, err := newGCPClient(context.Background()); err == nil {
		t.Fatal("expected newGCPClient() error")
	}

	newGCPClientFn = func(context.Context) (gcpKMSClient, error) {
		return fakeGCPClient{
			createCryptoKeyFn: func(context.Context, *kmspb.CreateCryptoKeyRequest) (*kmspb.CryptoKey, error) {
				return nil, errors.New("create boom")
			},
			createCryptoKeyVersionFn: func(context.Context, *kmspb.CreateCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error) {
				return nil, errors.New("create version boom")
			},
			getPublicKeyFn: func(context.Context, *kmspb.GetPublicKeyRequest) (*kmspb.PublicKey, error) {
				return &kmspb.PublicKey{Pem: "bad pem"}, nil
			},
			encryptFn: func(context.Context, *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) {
				return nil, errors.New("encrypt boom")
			},
			decryptFn: func(context.Context, *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) {
				return nil, errors.New("decrypt boom")
			},
			asymmetricSignFn: func(context.Context, *kmspb.AsymmetricSignRequest) (*kmspb.AsymmetricSignResponse, error) {
				return nil, errors.New("sign boom")
			},
			asymmetricDecryptFn: func(context.Context, *kmspb.AsymmetricDecryptRequest) (*kmspb.AsymmetricDecryptResponse, error) {
				return nil, errors.New("decrypt boom")
			},
			macSignFn: func(context.Context, *kmspb.MacSignRequest) (*kmspb.MacSignResponse, error) {
				return nil, errors.New("mac sign boom")
			},
			macVerifyFn: func(context.Context, *kmspb.MacVerifyRequest) (*kmspb.MacVerifyResponse, error) {
				return nil, errors.New("mac verify boom")
			},
			closeFn: func() error { return nil },
		}, nil
	}

	viper.Set(defaultGCPKeyIDKey, "projects/test/locations/global/keyRings/ring/cryptoKeys/default/cryptoKeyVersions/1")
	symmetricRepository := NewSymmetricRepository()
	hashRepository := NewHashRepository()
	asymmetricRepository := NewAsymmetricRepository()
	signatureRepository := NewSignatureRepository()

	if _, err := symmetricRepository.GeneratesSymetrycKey(context.Background(), common.Key256Bits); err == nil {
		t.Fatal("expected GeneratesSymetrycKey() provider error")
	}
	if _, err := symmetricRepository.EncryptAES(context.Background(), "", "payload", nil); err == nil {
		t.Fatal("expected EncryptAES() key name error")
	}
	if _, err := symmetricRepository.EncryptAES(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/key", "payload", nil); err == nil {
		t.Fatal("expected EncryptAES() provider error")
	}
	if _, err := symmetricRepository.DecryptAES(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/key", "%%%", nil); err == nil {
		t.Fatal("expected DecryptAES() decode error")
	}
	if _, err := symmetricRepository.DecryptAES(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/key", base64.StdEncoding.EncodeToString([]byte("cipher")), nil); err == nil {
		t.Fatal("expected DecryptAES() provider error")
	}
	if _, err := asymmetricRepository.GeneratesRSAKey(context.Background(), common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesRSAKey() provider error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Encode(context.Background(), "", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() version error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Encode(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/key/cryptoKeyVersions/1", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() provider error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode(context.Background(), "", "%%%"); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() decode error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/key/cryptoKeyVersions/1", base64.StdEncoding.EncodeToString([]byte("cipher"))); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() provider error")
	}
	if _, err := asymmetricRepository.GeneratesECCKey(context.Background(), common.CurveP256); !errors.Is(err, errGCPECCUnsupported) {
		t.Fatalf("GeneratesECCKey() error = %v", err)
	}
	if _, err := asymmetricRepository.ECC_Encode(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/key/cryptoKeyVersions/1", "payload"); !errors.Is(err, errGCPECCUnsupported) {
		t.Fatalf("ECC_Encode() error = %v", err)
	}
	if _, err := asymmetricRepository.ECC_Decode(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/key/cryptoKeyVersions/1", "payload"); !errors.Is(err, errGCPECCUnsupported) {
		t.Fatalf("ECC_Decode() error = %v", err)
	}
	if got := hashRepository.GenerateHMAC(context.Background(), viper.GetString(defaultGCPKeyIDKey), "message"); got != "" {
		t.Fatalf("GenerateHMAC() = %q, want empty string on provider error", got)
	}
	if hashRepository.ValidateHMAC(context.Background(), viper.GetString(defaultGCPKeyIDKey), "message", "%%%") {
		t.Fatal("expected ValidateHMAC() to fail on invalid MAC")
	}
	if _, err := signatureRepository.GeneratesEd255Key(context.Background(), common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesEd255Key() provider error")
	}
	if _, err := signatureRepository.SignEd25519(context.Background(), "", "payload"); err == nil {
		t.Fatal("expected SignEd25519() version error")
	}
	if err := signatureRepository.VerifyEd25519(context.Background(), "", "payload", "%%%"); err == nil {
		t.Fatal("expected VerifyEd25519() decode error")
	}
	if _, err := signatureRepository.SignEd25519(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/ed/cryptoKeyVersions/1", "payload"); err == nil {
		t.Fatal("expected SignEd25519() provider error")
	}
	if err := signatureRepository.VerifyEd25519(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/ed/cryptoKeyVersions/1", "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected VerifyEd25519() wrong public key error")
	}
	if _, err := signatureRepository.SignRSAPSS(context.Background(), "", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() version error")
	}
	if err := signatureRepository.VerifyRSAPSS(context.Background(), "", "payload", "%%%"); err == nil {
		t.Fatal("expected VerifyRSAPSS() decode error")
	}
	if _, err := signatureRepository.SignRSAPSS(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/rsa-pss/cryptoKeyVersions/1", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() provider error")
	}
	if err := signatureRepository.VerifyRSAPSS(context.Background(), "projects/test/locations/global/keyRings/ring/cryptoKeys/rsa-pss/cryptoKeyVersions/1", "payload", base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected VerifyRSAPSS() wrong public key error")
	}
	if _, err := signatureRepository.SignPKCS1v15_SHA256(context.Background(), "payload", nil); err == nil {
		t.Fatal("expected SignPKCS1v15_SHA256() provider error")
	}
	if err := signatureRepository.VerifySHA256(context.Background(), "payload", "%%%", nil); err == nil {
		t.Fatal("expected VerifySHA256() decode error")
	}
	if err := signatureRepository.VerifySHA256(context.Background(), "payload", base64.StdEncoding.EncodeToString([]byte("sig")), nil); err == nil {
		t.Fatal("expected VerifySHA256() wrong public key error")
	}
	if _, err := ensureGCPVersion(context.Background(), fakeGCPClient{
		createCryptoKeyFn: func(context.Context, *kmspb.CreateCryptoKeyRequest) (*kmspb.CryptoKey, error) { return nil, nil },
		createCryptoKeyVersionFn: func(context.Context, *kmspb.CreateCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error) {
			return &kmspb.CryptoKeyVersion{}, nil
		},
		getPublicKeyFn: func(context.Context, *kmspb.GetPublicKeyRequest) (*kmspb.PublicKey, error) { return nil, nil },
		encryptFn:      func(context.Context, *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) { return nil, nil },
		decryptFn:      func(context.Context, *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) { return nil, nil },
		asymmetricSignFn: func(context.Context, *kmspb.AsymmetricSignRequest) (*kmspb.AsymmetricSignResponse, error) {
			return nil, nil
		},
		asymmetricDecryptFn: func(context.Context, *kmspb.AsymmetricDecryptRequest) (*kmspb.AsymmetricDecryptResponse, error) {
			return nil, nil
		},
		macSignFn:   func(context.Context, *kmspb.MacSignRequest) (*kmspb.MacSignResponse, error) { return nil, nil },
		macVerifyFn: func(context.Context, *kmspb.MacVerifyRequest) (*kmspb.MacVerifyResponse, error) { return nil, nil },
		closeFn:     func() error { return nil },
	}, "name", kmspb.CryptoKeyVersion_RSA_DECRYPT_OAEP_2048_SHA256, nil); err == nil {
		t.Fatal("expected ensureGCPVersion() missing metadata error")
	}
	if _, err := fetchGCPPublicKey(context.Background(), fakeGCPClient{
		getPublicKeyFn: func(context.Context, *kmspb.GetPublicKeyRequest) (*kmspb.PublicKey, error) {
			return &kmspb.PublicKey{Pem: "bad pem"}, nil
		},
		createCryptoKeyFn: func(context.Context, *kmspb.CreateCryptoKeyRequest) (*kmspb.CryptoKey, error) { return nil, nil },
		createCryptoKeyVersionFn: func(context.Context, *kmspb.CreateCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error) {
			return nil, nil
		},
		encryptFn: func(context.Context, *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) { return nil, nil },
		decryptFn: func(context.Context, *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) { return nil, nil },
		asymmetricSignFn: func(context.Context, *kmspb.AsymmetricSignRequest) (*kmspb.AsymmetricSignResponse, error) {
			return nil, nil
		},
		asymmetricDecryptFn: func(context.Context, *kmspb.AsymmetricDecryptRequest) (*kmspb.AsymmetricDecryptResponse, error) {
			return nil, nil
		},
		macSignFn:   func(context.Context, *kmspb.MacSignRequest) (*kmspb.MacSignResponse, error) { return nil, nil },
		macVerifyFn: func(context.Context, *kmspb.MacVerifyRequest) (*kmspb.MacVerifyResponse, error) { return nil, nil },
		closeFn:     func() error { return nil },
	}, "name"); err == nil {
		t.Fatal("expected fetchGCPPublicKey() invalid PEM error")
	}
}

func TestGCPClientAdapterMethodsEnterWrappedCalls(t *testing.T) {
	adapter := &gcpClientAdapter{KeyManagementClient: (*kms.KeyManagementClient)(nil)}
	assertGCPPanic(t, func() { _, _ = adapter.CreateCryptoKey(context.Background(), &kmspb.CreateCryptoKeyRequest{}) })
	assertGCPPanic(t, func() {
		_, _ = adapter.CreateCryptoKeyVersion(context.Background(), &kmspb.CreateCryptoKeyVersionRequest{})
	})
	assertGCPPanic(t, func() { _, _ = adapter.GetPublicKey(context.Background(), &kmspb.GetPublicKeyRequest{}) })
	assertGCPPanic(t, func() { _, _ = adapter.Encrypt(context.Background(), &kmspb.EncryptRequest{}) })
	assertGCPPanic(t, func() { _, _ = adapter.Decrypt(context.Background(), &kmspb.DecryptRequest{}) })
	assertGCPPanic(t, func() { _, _ = adapter.AsymmetricSign(context.Background(), &kmspb.AsymmetricSignRequest{}) })
	assertGCPPanic(t, func() { _, _ = adapter.AsymmetricDecrypt(context.Background(), &kmspb.AsymmetricDecryptRequest{}) })
	assertGCPPanic(t, func() { _, _ = adapter.MacSign(context.Background(), &kmspb.MacSignRequest{}) })
	assertGCPPanic(t, func() { _, _ = adapter.MacVerify(context.Background(), &kmspb.MacVerifyRequest{}) })
}

func mustGCPRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	return privateKey
}

func mustGCPRSAPrivateBase64(t *testing.T, privateKey *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustGCPRSAPublicBase64(t *testing.T, publicKey *rsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustGCPECCPrivateBase64(t *testing.T, curve ecdh.Curve) string {
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

func mustGCPECCPublicBase64(t *testing.T, privateKeyBase64 string) string {
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

func assertGCPPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}
