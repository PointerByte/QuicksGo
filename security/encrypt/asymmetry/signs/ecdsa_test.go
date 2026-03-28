// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package signs

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	stdrsa "crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
)

func TestSignAndVerifyEd25519(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("expected ed25519 key without error, got %v", err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("expected private key marshal without error, got %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		t.Fatalf("expected public key marshal without error, got %v", err)
	}

	privateKeyB64 := base64.StdEncoding.EncodeToString(privateDER)
	publicKeyB64 := base64.StdEncoding.EncodeToString(publicDER)

	signature, err := SignEd25519(privateKeyB64, "hello signature")
	if err != nil {
		t.Fatalf("expected sign without error, got %v", err)
	}

	if err := VerifyEd25519(publicKeyB64, "hello signature", signature); err != nil {
		t.Fatalf("expected verify without error, got %v", err)
	}
}

func TestEd25519Errors(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("expected ed25519 key without error, got %v", err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("expected private key marshal without error, got %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		t.Fatalf("expected public key marshal without error, got %v", err)
	}

	privateKeyB64 := base64.StdEncoding.EncodeToString(privateDER)
	publicKeyB64 := base64.StdEncoding.EncodeToString(publicDER)

	signature, err := SignEd25519(privateKeyB64, "hello")
	if err != nil {
		t.Fatalf("expected sign without error, got %v", err)
	}

	if _, err := SignEd25519("%%%invalid-base64%%%", "hello"); err == nil || !strings.Contains(err.Error(), "load private key") {
		t.Fatalf("expected invalid private key error, got %v", err)
	}

	if err := VerifyEd25519("%%%invalid-base64%%%", "hello", signature); err == nil || !strings.Contains(err.Error(), "load public key") {
		t.Fatalf("expected invalid public key error, got %v", err)
	}

	if err := VerifyEd25519(publicKeyB64, "hello", "%%%invalid-base64%%%"); err == nil || !strings.Contains(err.Error(), "decode signature from Base64") {
		t.Fatalf("expected invalid signature base64 error, got %v", err)
	}

	if err := VerifyEd25519(publicKeyB64, "tampered", signature); err == nil || !strings.Contains(err.Error(), "invalid Ed25519 signature") {
		t.Fatalf("expected invalid signature error, got %v", err)
	}
}

func TestParseEd25519KeyErrors(t *testing.T) {
	ecPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("expected ecdsa key without error, got %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(&ecPrivateKey.PublicKey)
	if err != nil {
		t.Fatalf("expected ecdsa public key marshal without error, got %v", err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(ecPrivateKey)
	if err != nil {
		t.Fatalf("expected ecdsa private key marshal without error, got %v", err)
	}

	if _, err := ParseEd25519PublicKeyFromBase64(base64.StdEncoding.EncodeToString(publicDER)); err == nil || !strings.Contains(err.Error(), "public key is not an Ed25519 key") {
		t.Fatalf("expected non-ed25519 public key error, got %v", err)
	}

	if _, err := ParseEd25519PrivateKeyFromBase64(base64.StdEncoding.EncodeToString(privateDER)); err == nil || !strings.Contains(err.Error(), "private key is not an Ed25519 key") {
		t.Fatalf("expected non-ed25519 private key error, got %v", err)
	}
}

func mustRSAKeys(t *testing.T) (*stdrsa.PublicKey, string, string) {
	t.Helper()

	privateKey, err := stdrsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("expected rsa key without error, got %v", err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("expected private key marshal without error, got %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("expected public key marshal without error, got %v", err)
	}

	return &privateKey.PublicKey,
		base64.StdEncoding.EncodeToString(privateDER),
		base64.StdEncoding.EncodeToString(publicDER)
}

func TestSignAndVerifyPSS(t *testing.T) {
	_, privKeyB64, pubKeyB64 := mustRSAKeys(t)

	signature, err := SignRSAPSS(privKeyB64, "hello signature")
	if err != nil {
		t.Fatalf("expected sign without error, got %v", err)
	}

	if err := VerifyRSAPSS(pubKeyB64, "hello signature", signature); err != nil {
		t.Fatalf("expected verify without error, got %v", err)
	}
}

func TestSignAndVerifyPSSErrors(t *testing.T) {
	_, privKeyB64, pubKeyB64 := mustRSAKeys(t)
	_, _, otherPubKeyB64 := mustRSAKeys(t)

	_, err := SignRSAPSS("%%%invalid-base64%%%", "hello")
	if err == nil || !strings.Contains(err.Error(), "load private key") {
		t.Fatalf("expected sign private key error, got %v", err)
	}

	signature, err := SignRSAPSS(privKeyB64, "hello")
	if err != nil {
		t.Fatalf("expected sign without error, got %v", err)
	}

	err = VerifyRSAPSS("%%%invalid-base64%%%", "hello", signature)
	if err == nil || !strings.Contains(err.Error(), "load public key") {
		t.Fatalf("expected verify public key error, got %v", err)
	}

	err = VerifyRSAPSS(pubKeyB64, "hello", "%%%invalid-base64%%%")
	if err == nil || !strings.Contains(err.Error(), "decode signature from Base64") {
		t.Fatalf("expected verify signature base64 error, got %v", err)
	}

	err = VerifyRSAPSS(otherPubKeyB64, "hello", signature)
	if err == nil || !strings.Contains(err.Error(), "invalid RSA-PSS signature") {
		t.Fatalf("expected verify invalid signature error, got %v", err)
	}

	err = VerifyRSAPSS(pubKeyB64, "hello tampered", signature)
	if err == nil || !strings.Contains(err.Error(), "invalid RSA-PSS signature") {
		t.Fatalf("expected verify tampered message error, got %v", err)
	}
}

func TestSignAndVerifySHA256(t *testing.T) {
	privateKey, err := stdrsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("expected rsa key without error, got %v", err)
	}

	signature, err := SignSHA256([]byte("hello signature"), privateKey)
	if err != nil {
		t.Fatalf("expected sign without error, got %v", err)
	}

	if err := VerifySHA256([]byte("hello signature"), signature, &privateKey.PublicKey); err != nil {
		t.Fatalf("expected verify without error, got %v", err)
	}
}

func TestSignAndVerifySHA256Errors(t *testing.T) {
	privateKey, err := stdrsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("expected rsa key without error, got %v", err)
	}

	signature, err := SignSHA256([]byte("hello"), privateKey)
	if err != nil {
		t.Fatalf("expected sign without error, got %v", err)
	}

	if _, err := SignSHA256([]byte("hello"), nil); err == nil || !strings.Contains(err.Error(), "private key is required") {
		t.Fatalf("expected nil private key error, got %v", err)
	}

	if err := VerifySHA256([]byte("hello"), signature, nil); err == nil || !strings.Contains(err.Error(), "public key is required") {
		t.Fatalf("expected nil public key error, got %v", err)
	}

	if err := VerifySHA256([]byte("tampered"), signature, &privateKey.PublicKey); err == nil || !strings.Contains(err.Error(), "invalid RSA SHA-256 signature") {
		t.Fatalf("expected invalid signature error, got %v", err)
	}
}
