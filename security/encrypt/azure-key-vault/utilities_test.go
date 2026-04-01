// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package azurekeyvault

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestParseKeysFromBase64(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	publicDER, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	privateDER, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	if _, err := ParseRSAPublicKeyFromBase64(base64.StdEncoding.EncodeToString(publicDER)); err != nil {
		t.Fatalf("ParseRSAPublicKeyFromBase64() error = %v", err)
	}
	if _, err := ParseRSAPrivateKeyFromBase64(base64.StdEncoding.EncodeToString(privateDER)); err != nil {
		t.Fatalf("ParseRSAPrivateKeyFromBase64() error = %v", err)
	}

	edPublic, edPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}
	edPublicDER, _ := x509.MarshalPKIXPublicKey(edPublic)
	edPrivateDER, _ := x509.MarshalPKCS8PrivateKey(edPrivate)
	if _, err := ParseEd25519PublicKeyFromBase64(base64.StdEncoding.EncodeToString(edPublicDER)); err != nil {
		t.Fatalf("ParseEd25519PublicKeyFromBase64() error = %v", err)
	}
	if _, err := ParseEd25519PrivateKeyFromBase64(base64.StdEncoding.EncodeToString(edPrivateDER)); err != nil {
		t.Fatalf("ParseEd25519PrivateKeyFromBase64() error = %v", err)
	}
}

func TestParseKeysFromBase64Errors(t *testing.T) {
	if _, err := ParseRSAPublicKeyFromBase64("%%%"); err == nil {
		t.Fatal("expected ParseRSAPublicKeyFromBase64() error")
	}
	if _, err := ParseRSAPrivateKeyFromBase64("%%%"); err == nil {
		t.Fatal("expected ParseRSAPrivateKeyFromBase64() error")
	}
	if _, err := ParseEd25519PublicKeyFromBase64("%%%"); err == nil {
		t.Fatal("expected ParseEd25519PublicKeyFromBase64() error")
	}
	if _, err := ParseEd25519PrivateKeyFromBase64("%%%"); err == nil {
		t.Fatal("expected ParseEd25519PrivateKeyFromBase64() error")
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	publicDER, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	privateDER, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	edPublic, edPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}
	edPublicDER, _ := x509.MarshalPKIXPublicKey(edPublic)
	edPrivateDER, _ := x509.MarshalPKCS8PrivateKey(edPrivate)

	if _, err := ParseRSAPublicKeyFromBase64(base64.StdEncoding.EncodeToString([]byte("bad"))); err == nil {
		t.Fatal("expected ParseRSAPublicKeyFromBase64() parse error")
	}
	if _, err := ParseRSAPrivateKeyFromBase64(base64.StdEncoding.EncodeToString([]byte("bad"))); err == nil {
		t.Fatal("expected ParseRSAPrivateKeyFromBase64() parse error")
	}
	if _, err := ParseEd25519PublicKeyFromBase64(base64.StdEncoding.EncodeToString([]byte("bad"))); err == nil {
		t.Fatal("expected ParseEd25519PublicKeyFromBase64() parse error")
	}
	if _, err := ParseEd25519PrivateKeyFromBase64(base64.StdEncoding.EncodeToString([]byte("bad"))); err == nil {
		t.Fatal("expected ParseEd25519PrivateKeyFromBase64() parse error")
	}
	if _, err := ParseRSAPublicKeyFromBase64(base64.StdEncoding.EncodeToString(edPublicDER)); err == nil {
		t.Fatal("expected ParseRSAPublicKeyFromBase64() wrong type error")
	}
	if _, err := ParseRSAPrivateKeyFromBase64(base64.StdEncoding.EncodeToString(edPrivateDER)); err == nil {
		t.Fatal("expected ParseRSAPrivateKeyFromBase64() wrong type error")
	}
	if _, err := ParseEd25519PublicKeyFromBase64(base64.StdEncoding.EncodeToString(publicDER)); err == nil {
		t.Fatal("expected ParseEd25519PublicKeyFromBase64() wrong type error")
	}
	if _, err := ParseEd25519PrivateKeyFromBase64(base64.StdEncoding.EncodeToString(privateDER)); err == nil {
		t.Fatal("expected ParseEd25519PrivateKeyFromBase64() wrong type error")
	}
}
