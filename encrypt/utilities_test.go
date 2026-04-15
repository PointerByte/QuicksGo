// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package encrypt

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestParseRSAKeysFromBase64(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}

	gotPublic, err := ParseRSAPublicKeyFromBase64(base64.StdEncoding.EncodeToString(publicDER))
	if err != nil {
		t.Fatalf("ParseRSAPublicKeyFromBase64() error = %v", err)
	}
	if gotPublic.N.Cmp(privateKey.PublicKey.N) != 0 || gotPublic.E != privateKey.PublicKey.E {
		t.Fatal("ParseRSAPublicKeyFromBase64() returned unexpected key")
	}

	gotPrivate, err := ParseRSAPrivateKeyFromBase64(base64.StdEncoding.EncodeToString(privateDER))
	if err != nil {
		t.Fatalf("ParseRSAPrivateKeyFromBase64() error = %v", err)
	}
	if gotPrivate.N.Cmp(privateKey.N) != 0 || gotPrivate.E != privateKey.E {
		t.Fatal("ParseRSAPrivateKeyFromBase64() returned unexpected key")
	}
}

func TestParseRSAKeysFromBase64Errors(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
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

	pkcs1Private := x509.MarshalPKCS1PrivateKey(privateKey)
	pkcs1Public := x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)

	tests := []struct {
		name string
		fn   func(string) error
		in   string
	}{
		{name: "public bad base64", fn: func(s string) error { _, err := ParseRSAPublicKeyFromBase64(s); return err }, in: "%%%"},
		{name: "public parse failure", fn: func(s string) error { _, err := ParseRSAPublicKeyFromBase64(s); return err }, in: base64.StdEncoding.EncodeToString(pkcs1Public)},
		{name: "public wrong type", fn: func(s string) error { _, err := ParseRSAPublicKeyFromBase64(s); return err }, in: base64.StdEncoding.EncodeToString(edPublicDER)},
		{name: "private bad base64", fn: func(s string) error { _, err := ParseRSAPrivateKeyFromBase64(s); return err }, in: "%%%"},
		{name: "private parse failure", fn: func(s string) error { _, err := ParseRSAPrivateKeyFromBase64(s); return err }, in: base64.StdEncoding.EncodeToString(pkcs1Private)},
		{name: "private wrong type", fn: func(s string) error { _, err := ParseRSAPrivateKeyFromBase64(s); return err }, in: base64.StdEncoding.EncodeToString(edPrivateDER)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.fn(test.in); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseEd25519KeysFromBase64(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}

	gotPublic, err := ParseEd25519PublicKeyFromBase64(base64.StdEncoding.EncodeToString(publicDER))
	if err != nil {
		t.Fatalf("ParseEd25519PublicKeyFromBase64() error = %v", err)
	}
	if string(gotPublic) != string(publicKey) {
		t.Fatal("ParseEd25519PublicKeyFromBase64() returned unexpected key")
	}

	gotPrivate, err := ParseEd25519PrivateKeyFromBase64(base64.StdEncoding.EncodeToString(privateDER))
	if err != nil {
		t.Fatalf("ParseEd25519PrivateKeyFromBase64() error = %v", err)
	}
	if string(gotPrivate) != string(privateKey) {
		t.Fatal("ParseEd25519PrivateKeyFromBase64() returned unexpected key")
	}
}

func TestParseEd25519KeysFromBase64Errors(t *testing.T) {
	rsaPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}

	rsaPublicDER, err := x509.MarshalPKIXPublicKey(&rsaPrivateKey.PublicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	rsaPrivateDER, err := x509.MarshalPKCS8PrivateKey(rsaPrivateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}

	tests := []struct {
		name string
		fn   func(string) error
		in   string
	}{
		{name: "public bad base64", fn: func(s string) error { _, err := ParseEd25519PublicKeyFromBase64(s); return err }, in: "%%%"},
		{name: "public parse failure", fn: func(s string) error { _, err := ParseEd25519PublicKeyFromBase64(s); return err }, in: base64.StdEncoding.EncodeToString([]byte("bad"))},
		{name: "public wrong type", fn: func(s string) error { _, err := ParseEd25519PublicKeyFromBase64(s); return err }, in: base64.StdEncoding.EncodeToString(rsaPublicDER)},
		{name: "private bad base64", fn: func(s string) error { _, err := ParseEd25519PrivateKeyFromBase64(s); return err }, in: "%%%"},
		{name: "private parse failure", fn: func(s string) error { _, err := ParseEd25519PrivateKeyFromBase64(s); return err }, in: base64.StdEncoding.EncodeToString([]byte("bad"))},
		{name: "private wrong type", fn: func(s string) error { _, err := ParseEd25519PrivateKeyFromBase64(s); return err }, in: base64.StdEncoding.EncodeToString(rsaPrivateDER)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.fn(test.in); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
