// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package azurekeyvault

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"testing"

	"github.com/PointerByte/QuicksGo/security/encrypt/common"
	"github.com/spf13/viper"
)

func TestNewRepositoryBuildsAllRepositories(t *testing.T) {
	repository, ok := NewRepository().(*repository)
	if !ok {
		t.Fatalf("NewRepository() type = %T, want *repository", NewRepository())
	}
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

	if repository.GenerateHMAC("message", "secret") == "" {
		t.Fatal("GenerateHMAC() returned empty value")
	}
	if repository.Sha256Hex("message") == "" || repository.Blake3("message") == "" {
		t.Fatal("expected digest helpers to return values")
	}
}

func TestAsymmetricAndSignatureErrorsAndFallbacks(t *testing.T) {
	t.Cleanup(viper.Reset)
	repository := NewRepository()

	if _, _, err := repository.GeneratesRSAKey(common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesRSAKey() error")
	}
	if _, _, err := repository.GeneratesEd255Key(common.Key2048Bits); err == nil {
		t.Fatal("expected GeneratesEd255Key() error")
	}
	if _, err := repository.SignEd25519("", "payload"); err == nil {
		t.Fatal("expected SignEd25519() error")
	}
	if err := repository.VerifyEd25519("", "payload", "sig"); err == nil {
		t.Fatal("expected VerifyEd25519() error")
	}

	if _, err := repository.RSA_OAEP_Encode("", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() key id error")
	}
	if _, err := repository.RSA_OAEP_Decode("", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() key id error")
	}
	if _, err := repository.SignRSAPSS("", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() key id error")
	}
	if err := repository.VerifyRSAPSS("", "payload", "sig"); err == nil {
		t.Fatal("expected VerifyRSAPSS() key id error")
	}
	if _, err := repository.SignSHA256("payload", nil); err == nil {
		t.Fatal("expected SignSHA256() key id error")
	}
	if err := repository.VerifySHA256("payload", "sig", nil); err == nil {
		t.Fatal("expected VerifySHA256() key id error")
	}

	viper.Set(defaultKeyIDKey, "azure-key")
	if _, err := repository.RSA_OAEP_Encode("", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() not implemented error")
	}
	if _, err := repository.RSA_OAEP_Decode("", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() not implemented error")
	}
	if _, err := repository.SignRSAPSS("", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() not implemented error")
	}
	if err := repository.VerifyRSAPSS("", "payload", "sig"); err == nil {
		t.Fatal("expected VerifyRSAPSS() not implemented error")
	}
	if _, err := repository.SignSHA256("payload", nil); err == nil {
		t.Fatal("expected SignSHA256() not implemented error")
	}
	if err := repository.VerifySHA256("payload", "sig", nil); err == nil {
		t.Fatal("expected VerifySHA256() not implemented error")
	}

	privateKey := mustRSAKey(t)
	publicKey := &privateKey.PublicKey
	publicB64 := mustMarshalPKIXRSAPublicKey(t, publicKey)
	privateB64 := mustMarshalPKCS8RSAPrivateKey(t, privateKey)

	ciphertext, err := repository.RSA_OAEP_Encode(publicB64, "hello")
	if err != nil {
		t.Fatalf("RSA_OAEP_Encode() error = %v", err)
	}
	plaintext, err := repository.RSA_OAEP_Decode(privateB64, ciphertext)
	if err != nil {
		t.Fatalf("RSA_OAEP_Decode() error = %v", err)
	}
	if plaintext != "hello" {
		t.Fatalf("RSA_OAEP_Decode() = %q, want %q", plaintext, "hello")
	}

	signature, err := repository.SignRSAPSS(privateB64, "payload")
	if err != nil {
		t.Fatalf("SignRSAPSS() error = %v", err)
	}
	if err := repository.VerifyRSAPSS(publicB64, "payload", signature); err != nil {
		t.Fatalf("VerifyRSAPSS() error = %v", err)
	}

	signature, err = repository.SignSHA256("payload", privateKey)
	if err != nil {
		t.Fatalf("SignSHA256() error = %v", err)
	}
	if err := repository.VerifySHA256("payload", signature, publicKey); err != nil {
		t.Fatalf("VerifySHA256() error = %v", err)
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
