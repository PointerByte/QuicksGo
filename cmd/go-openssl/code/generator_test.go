// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package code

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCertificatesByAlgorithm(t *testing.T) {
	tests := []struct {
		name      string
		algorithm string
		curve     string
		assertKey func(t *testing.T, privateKey any)
	}{
		{
			name:      "rsa",
			algorithm: algorithmRSA,
			assertKey: func(t *testing.T, privateKey any) {
				t.Helper()
				if _, ok := privateKey.(*rsa.PrivateKey); !ok {
					t.Fatalf("expected rsa private key, got %T", privateKey)
				}
			},
		},
		{
			name:      "ecc",
			algorithm: algorithmECC,
			curve:     curveP384,
			assertKey: func(t *testing.T, privateKey any) {
				t.Helper()
				key, ok := privateKey.(*ecdsa.PrivateKey)
				if !ok {
					t.Fatalf("expected ecdsa private key, got %T", privateKey)
				}
				if key.Curve.Params().Name != "P-384" {
					t.Fatalf("expected P-384 curve, got %s", key.Curve.Params().Name)
				}
			},
		},
		{
			name:      "ed25519",
			algorithm: algorithmEd25519,
			assertKey: func(t *testing.T, privateKey any) {
				t.Helper()
				if _, ok := privateKey.(ed25519.PrivateKey); !ok {
					t.Fatalf("expected ed25519 private key, got %T", privateKey)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputDir := filepath.Join(t.TempDir(), test.name)
			result, err := GenerateCertificates(Options{
				Algorithm:    test.algorithm,
				ECCCurve:     test.curve,
				OutputDir:    outputDir,
				CommonName:   "localhost",
				ValidForDays: 10,
				Salt:         "salt-value",
			})
			if err != nil {
				t.Fatalf("GenerateCertificates returned error: %v", err)
			}

			certificate := parseCertificateFile(t, result.CertificatePath)
			if certificate.Subject.CommonName != "localhost" {
				t.Fatalf("expected common name localhost, got %q", certificate.Subject.CommonName)
			}

			privateKey := parsePrivateKeyFile(t, result.PrivateKeyPath)
			test.assertKey(t, privateKey)

			if _, err := os.Stat(result.PublicKeyPath); err != nil {
				t.Fatalf("expected public key file to exist, got %v", err)
			}
		})
	}
}

func TestGenerateCertificatesSignedByCA(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "certs")
	caResult, err := GenerateCertificates(Options{
		Algorithm:         algorithmECC,
		ECCCurve:          curveP521,
		OutputDir:         outputDir,
		CommonName:        "dragon-cmk-ca.chest-max.svc.cluster.local",
		Organization:      "PointerByte",
		ValidForDays:      365,
		IsCA:              true,
		CertFileName:      "ca.pem",
		KeyFileName:       "ca-key.pem",
		PublicKeyFileName: "ca-public.pem",
	})
	if err != nil {
		t.Fatalf("GenerateCertificates CA returned error: %v", err)
	}

	result, err := GenerateCertificates(Options{
		Algorithm:         algorithmECC,
		ECCCurve:          curveP521,
		OutputDir:         outputDir,
		CommonName:        "dragon-cmk.chest-max.svc.cluster.local",
		DNSNames:          []string{"dragon-cmk.chest-max.svc.cluster.local"},
		Organization:      "PointerByte",
		ValidForDays:      365,
		CertFileName:      "cert.pem",
		KeyFileName:       "key.pem",
		PublicKeyFileName: "public.pem",
		SignedBy:          caResult.CertificatePath,
		CAKeyFile:         caResult.PrivateKeyPath,
	})
	if err != nil {
		t.Fatalf("GenerateCertificates signed certificate returned error: %v", err)
	}

	caCertificate := parseCertificateFile(t, caResult.CertificatePath)
	certificate := parseCertificateFile(t, result.CertificatePath)
	if !caCertificate.IsCA {
		t.Fatal("expected generated CA certificate to be marked as CA")
	}
	if certificate.Issuer.CommonName != caCertificate.Subject.CommonName {
		t.Fatalf("expected issuer %q, got %q", caCertificate.Subject.CommonName, certificate.Issuer.CommonName)
	}
	if err := certificate.CheckSignatureFrom(caCertificate); err != nil {
		t.Fatalf("expected certificate to verify against generated CA, got %v", err)
	}

	if _, err := os.Stat(result.PrivateKeyPath); err != nil {
		t.Fatalf("expected private key file to exist, got %v", err)
	}
	if _, err := os.Stat(result.PublicKeyPath); err != nil {
		t.Fatalf("expected public key file to exist, got %v", err)
	}
}

func TestGenerateCertificatesErrors(t *testing.T) {
	if _, err := GenerateCertificates(Options{Algorithm: "dsa", OutputDir: t.TempDir(), CommonName: "localhost"}); err == nil {
		t.Fatal("expected unsupported algorithm error")
	}

	if _, err := GenerateCertificates(Options{
		Algorithm:  algorithmECC,
		ECCCurve:   "p111",
		OutputDir:  t.TempDir(),
		CommonName: "localhost",
	}); err == nil {
		t.Fatal("expected unsupported ecc curve error")
	}

	if _, err := GenerateCertificates(Options{
		Algorithm:  algorithmRSA,
		RSAKeySize: 1024,
		OutputDir:  t.TempDir(),
		CommonName: "localhost",
	}); err == nil {
		t.Fatal("expected rsa key size error")
	}

	if _, err := GenerateCertificates(Options{
		OutputDir:  t.TempDir(),
		CommonName: "localhost",
		SignedBy:   "ca.pem",
	}); err == nil {
		t.Fatal("expected signed-by without ca-key error")
	}
}

func parseCertificateFile(t *testing.T, path string) *x509.Certificate {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected certificate file, got %v", err)
	}

	block, _ := pem.Decode(content)
	if block == nil {
		t.Fatal("expected certificate PEM block")
	}

	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("expected certificate parsing without error, got %v", err)
	}
	return certificate
}

func parsePrivateKeyFile(t *testing.T, path string) any {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected private key file, got %v", err)
	}

	block, _ := pem.Decode(content)
	if block == nil {
		t.Fatal("expected private key PEM block")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("expected private key parsing without error, got %v", err)
	}
	return privateKey
}
