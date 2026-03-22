// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package rsa

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	stdrsa "crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseRSAPublicKeyFromBase64(t *testing.T) {
	pubKey, privKeyB64, pubKeyB64 := mustRSAKeys(t)
	parsed, err := ParseRSAPublicKeyFromBase64(pubKeyB64)
	if err != nil {
		t.Fatalf("expected public key parse without error, got %v", err)
	}

	if parsed.N.Cmp(pubKey.N) != 0 {
		t.Fatal("expected parsed public key to match original")
	}

	_, err = ParseRSAPublicKeyFromBase64("%%%invalid-base64%%%")
	if err == nil || !strings.Contains(err.Error(), "decode public key from Base64") {
		t.Fatalf("expected public key base64 error, got %v", err)
	}

	_, err = ParseRSAPublicKeyFromBase64(privKeyB64)
	if err == nil || !strings.Contains(err.Error(), "parse public key") {
		t.Fatalf("expected public key parse error, got %v", err)
	}

	ecPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("expected ecdsa key without error, got %v", err)
	}

	ecPublicDER, err := x509.MarshalPKIXPublicKey(&ecPrivateKey.PublicKey)
	if err != nil {
		t.Fatalf("expected ecdsa public key marshal without error, got %v", err)
	}

	_, err = ParseRSAPublicKeyFromBase64(base64.StdEncoding.EncodeToString(ecPublicDER))
	if err == nil || !strings.Contains(err.Error(), "public key is not an RSA key") {
		t.Fatalf("expected non-rsa public key error, got %v", err)
	}
}

func TestParseRSAPrivateKeyFromBase64(t *testing.T) {
	privKey, privKeyB64, _ := mustRSAKeys(t)
	parsed, err := ParseRSAPrivateKeyFromBase64(privKeyB64)
	if err != nil {
		t.Fatalf("expected private key parse without error, got %v", err)
	}

	if parsed.N.Cmp(privKey.N) != 0 {
		t.Fatal("expected parsed private key to match original")
	}

	_, err = ParseRSAPrivateKeyFromBase64("%%%invalid-base64%%%")
	if err == nil || !strings.Contains(err.Error(), "decode private key from Base64") {
		t.Fatalf("expected private key base64 error, got %v", err)
	}

	ecPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("expected ecdsa key without error, got %v", err)
	}

	ecPrivateDER, err := x509.MarshalPKCS8PrivateKey(ecPrivateKey)
	if err != nil {
		t.Fatalf("expected ecdsa pkcs8 marshal without error, got %v", err)
	}

	_, err = ParseRSAPrivateKeyFromBase64(base64.StdEncoding.EncodeToString(ecPrivateDER))
	if err == nil || !strings.Contains(err.Error(), "private key is not an RSA key") {
		t.Fatalf("expected non-rsa private key error, got %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(&parsed.PublicKey)
	if err != nil {
		t.Fatalf("expected rsa public key marshal without error, got %v", err)
	}

	_, err = ParseRSAPrivateKeyFromBase64(base64.StdEncoding.EncodeToString(publicDER))
	if err == nil || !strings.Contains(err.Error(), "parse private key") {
		t.Fatalf("expected public key as private key parse error, got %v", err)
	}
}

func TestEncodeDecodeVariants(t *testing.T) {
	_, privKeyB64, pubKeyB64 := mustRSAKeys(t)

	tests := []struct {
		name   string
		encode func(string, string) (string, error)
		decode func(string, string) (string, error)
	}{
		{name: "oaep sha256", encode: Encode, decode: Decode},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cipherText, err := test.encode(pubKeyB64, "hello rsa")
			if err != nil {
				t.Fatalf("expected encode without error, got %v", err)
			}

			plainText, err := test.decode(privKeyB64, cipherText)
			if err != nil {
				t.Fatalf("expected decode without error, got %v", err)
			}

			if plainText != "hello rsa" {
				t.Fatalf("expected plaintext %q, got %q", "hello rsa", plainText)
			}
		})
	}
}

func TestEncodeProducesDifferentCiphertextsForSamePlaintext(t *testing.T) {
	_, _, pubKeyB64 := mustRSAKeys(t)

	cipherText1, err := Encode(pubKeyB64, "same message")
	if err != nil {
		t.Fatalf("expected first encode without error, got %v", err)
	}

	cipherText2, err := Encode(pubKeyB64, "same message")
	if err != nil {
		t.Fatalf("expected second encode without error, got %v", err)
	}

	if cipherText1 == cipherText2 {
		t.Fatal("expected RSA-OAEP ciphertexts to differ due to random padding")
	}
}

func TestEncodeDecodeEmptyPlaintext(t *testing.T) {
	_, privKeyB64, pubKeyB64 := mustRSAKeys(t)

	cipherText, err := Encode(pubKeyB64, "")
	if err != nil {
		t.Fatalf("expected encode without error, got %v", err)
	}

	plainText, err := Decode(privKeyB64, cipherText)
	if err != nil {
		t.Fatalf("expected decode without error, got %v", err)
	}

	if plainText != "" {
		t.Fatalf("expected empty plaintext, got %q", plainText)
	}
}

func TestEncodeDecodeErrors(t *testing.T) {
	_, privKeyB64, pubKeyB64 := mustRSAKeys(t)
	otherPriv, otherPrivKeyB64, _ := mustRSAKeys(t)

	longText := strings.Repeat("A", 300)

	tests := []struct {
		name        string
		encode      func(string, string) (string, error)
		decode      func(string, string) (string, error)
		encodeError string
		decodeError string
	}{
		{name: "oaep sha256", encode: Encode, decode: Decode, encodeError: "load public key", decodeError: "load private key"},
	}

	for _, test := range tests {
		t.Run(test.name+" invalid public key", func(t *testing.T) {
			_, err := test.encode("%%%invalid-base64%%%", "hello")
			if err == nil || !strings.Contains(err.Error(), test.encodeError) {
				t.Fatalf("expected encode key error, got %v", err)
			}
		})

		t.Run(test.name+" invalid private key", func(t *testing.T) {
			_, err := test.decode("%%%invalid-base64%%%", "cipher")
			if err == nil || !strings.Contains(err.Error(), test.decodeError) {
				t.Fatalf("expected decode key error, got %v", err)
			}
		})

		t.Run(test.name+" invalid ciphertext base64", func(t *testing.T) {
			_, err := test.decode(privKeyB64, "%%%invalid-base64%%%")
			if err == nil || !strings.Contains(err.Error(), "decode Base64 ciphertext") {
				t.Fatalf("expected ciphertext base64 error, got %v", err)
			}
		})

		t.Run(test.name+" decrypt error", func(t *testing.T) {
			cipherText, err := test.encode(pubKeyB64, "hello")
			if err != nil {
				t.Fatalf("expected encode without error, got %v", err)
			}

			_, err = test.decode(otherPrivKeyB64, cipherText)
			if err == nil || !strings.Contains(err.Error(), "decrypt with RSA-OAEP") {
				t.Fatalf("expected decrypt error, got %v", err)
			}
		})

		t.Run(test.name+" tampered ciphertext", func(t *testing.T) {
			cipherText, err := test.encode(pubKeyB64, "hello")
			if err != nil {
				t.Fatalf("expected encode without error, got %v", err)
			}

			cipherBytes, err := base64.StdEncoding.DecodeString(cipherText)
			if err != nil {
				t.Fatalf("expected ciphertext base64 decode without error, got %v", err)
			}

			cipherBytes[len(cipherBytes)-1] ^= 0xFF
			tampered := base64.StdEncoding.EncodeToString(cipherBytes)

			_, err = test.decode(privKeyB64, tampered)
			if err == nil || !strings.Contains(err.Error(), "decrypt with RSA-OAEP") {
				t.Fatalf("expected tampered decrypt error, got %v", err)
			}
		})

		t.Run(test.name+" encrypt error", func(t *testing.T) {
			_ = otherPriv
			_, err := test.encode(pubKeyB64, longText)
			if err == nil || !strings.Contains(err.Error(), "encrypt with RSA-OAEP") {
				t.Fatalf("expected encrypt error, got %v", err)
			}
		})
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
