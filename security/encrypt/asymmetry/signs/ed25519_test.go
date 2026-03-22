// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package signs

import (
	"crypto/rand"
	stdrsa "crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
)

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
