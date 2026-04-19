// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package code

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options configures the generated certificate, keys, output paths, and algorithm-specific settings.
type Options struct {
	Algorithm         string
	OutputDir         string
	CommonName        string
	DNSNames          []string
	IPAddresses       []string
	Organization      string
	ValidForDays      int
	RSAKeySize        int
	ECCCurve          string
	Salt              string
	CertFileName      string
	KeyFileName       string
	PublicKeyFileName string
	IsCA              bool
}

// Result describes the generated PEM artifacts and the effective generation parameters.
type Result struct {
	Algorithm       string
	OutputDir       string
	CertificatePath string
	PrivateKeyPath  string
	PublicKeyPath   string
}

// Generator coordinates filesystem writes and cryptographic generation helpers.
type Generator struct {
	mkdirAllFn  func(string, os.FileMode) error
	writeFileFn func(string, []byte, os.FileMode) error
	nowFn       func() time.Time
	randReader  io.Reader
}

// NewGenerator creates the default certificate generator.
func NewGenerator() *Generator {
	return &Generator{
		mkdirAllFn:  os.MkdirAll,
		writeFileFn: os.WriteFile,
		nowFn:       time.Now,
		randReader:  rand.Reader,
	}
}

// GenerateCertificates generates PEM certificate assets using the default generator.
func GenerateCertificates(options Options) (Result, error) {
	return NewGenerator().Generate(options)
}

// Generate creates a self-signed certificate together with matching private and public keys.
func (generator *Generator) Generate(options Options) (Result, error) {
	resolvedOptions, err := normalizeOptions(options)
	if err != nil {
		return Result{}, err
	}

	if err := generator.mkdirAllFn(resolvedOptions.OutputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output directory: %w", err)
	}

	randomSource, err := generator.randomSource(resolvedOptions.Salt)
	if err != nil {
		return Result{}, fmt.Errorf("build random source: %w", err)
	}

	privateKey, publicKey, err := buildKeyPair(randomSource, resolvedOptions)
	if err != nil {
		return Result{}, err
	}

	certificatePEM, err := generator.buildCertificate(randomSource, resolvedOptions, publicKey, privateKey)
	if err != nil {
		return Result{}, err
	}

	privateKeyPEM, err := encodePrivateKeyPEM(privateKey)
	if err != nil {
		return Result{}, err
	}

	publicKeyPEM, err := encodePublicKeyPEM(publicKey)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Algorithm:       resolvedOptions.Algorithm,
		OutputDir:       resolvedOptions.OutputDir,
		CertificatePath: filepath.Join(resolvedOptions.OutputDir, resolvedOptions.CertFileName),
		PrivateKeyPath:  filepath.Join(resolvedOptions.OutputDir, resolvedOptions.KeyFileName),
		PublicKeyPath:   filepath.Join(resolvedOptions.OutputDir, resolvedOptions.PublicKeyFileName),
	}

	if err := generator.writeFileFn(result.CertificatePath, certificatePEM, 0o644); err != nil {
		return Result{}, fmt.Errorf("write certificate file: %w", err)
	}
	if err := generator.writeFileFn(result.PrivateKeyPath, privateKeyPEM, 0o600); err != nil {
		return Result{}, fmt.Errorf("write private key file: %w", err)
	}
	if err := generator.writeFileFn(result.PublicKeyPath, publicKeyPEM, 0o644); err != nil {
		return Result{}, fmt.Errorf("write public key file: %w", err)
	}

	return result, nil
}

func defaultOptions() *Options {
	return &Options{
		Algorithm:         algorithmRSA,
		OutputDir:         ".",
		CommonName:        "localhost",
		DNSNames:          []string{"localhost"},
		Organization:      "PointerByte",
		ValidForDays:      365,
		RSAKeySize:        2048,
		ECCCurve:          curveP256,
		CertFileName:      "cert.pem",
		KeyFileName:       "key.pem",
		PublicKeyFileName: "public.pem",
	}
}

func normalizeOptions(options Options) (Options, error) {
	defaults := defaultOptions()

	options.Algorithm = strings.ToLower(strings.TrimSpace(coalesce(options.Algorithm, defaults.Algorithm)))
	options.OutputDir = strings.TrimSpace(coalesce(options.OutputDir, defaults.OutputDir))
	options.CommonName = strings.TrimSpace(coalesce(options.CommonName, defaults.CommonName))
	options.Organization = strings.TrimSpace(coalesce(options.Organization, defaults.Organization))
	options.ECCCurve = strings.ToLower(strings.TrimSpace(coalesce(options.ECCCurve, defaults.ECCCurve)))
	options.CertFileName = strings.TrimSpace(coalesce(options.CertFileName, defaults.CertFileName))
	options.KeyFileName = strings.TrimSpace(coalesce(options.KeyFileName, defaults.KeyFileName))
	options.PublicKeyFileName = strings.TrimSpace(coalesce(options.PublicKeyFileName, defaults.PublicKeyFileName))
	options.Salt = strings.TrimSpace(options.Salt)

	if options.ValidForDays <= 0 {
		options.ValidForDays = defaults.ValidForDays
	}
	if options.RSAKeySize == 0 {
		options.RSAKeySize = defaults.RSAKeySize
	}
	if len(options.DNSNames) == 0 {
		options.DNSNames = append([]string(nil), defaults.DNSNames...)
	}

	if options.OutputDir == "" {
		return Options{}, fmt.Errorf("output directory is required")
	}
	if options.CommonName == "" {
		return Options{}, fmt.Errorf("common name is required")
	}
	if options.ValidForDays <= 0 {
		return Options{}, fmt.Errorf("days must be greater than zero")
	}
	if options.CertFileName == "" || options.KeyFileName == "" || options.PublicKeyFileName == "" {
		return Options{}, fmt.Errorf("certificate, key, and public key file names are required")
	}

	switch options.Algorithm {
	case algorithmRSA:
		if options.RSAKeySize < 2048 {
			return Options{}, fmt.Errorf("rsa-bits must be at least 2048")
		}
	case algorithmECC:
		if _, err := resolveCurve(options.ECCCurve); err != nil {
			return Options{}, err
		}
	case algorithmEd25519:
	default:
		return Options{}, fmt.Errorf("unsupported algorithm %q", options.Algorithm)
	}

	return options, nil
}

func coalesce(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (generator *Generator) randomSource(salt string) (io.Reader, error) {
	if salt == "" {
		return generator.randReader, nil
	}

	seed := make([]byte, 32)
	if _, err := io.ReadFull(generator.randReader, seed); err != nil {
		return nil, err
	}
	return &saltedReader{
		seed:    seed,
		salt:    []byte(salt),
		counter: 0,
	}, nil
}

func buildKeyPair(randomSource io.Reader, options Options) (any, any, error) {
	switch options.Algorithm {
	case algorithmRSA:
		privateKey, err := rsa.GenerateKey(randomSource, options.RSAKeySize)
		if err != nil {
			return nil, nil, fmt.Errorf("generate rsa key: %w", err)
		}
		return privateKey, &privateKey.PublicKey, nil
	case algorithmECC:
		curve, err := resolveCurve(options.ECCCurve)
		if err != nil {
			return nil, nil, err
		}
		privateKey, err := ecdsa.GenerateKey(curve, randomSource)
		if err != nil {
			return nil, nil, fmt.Errorf("generate ecc key: %w", err)
		}
		return privateKey, &privateKey.PublicKey, nil
	case algorithmEd25519:
		publicKey, privateKey, err := ed25519.GenerateKey(randomSource)
		if err != nil {
			return nil, nil, fmt.Errorf("generate ed25519 key: %w", err)
		}
		return privateKey, publicKey, nil
	default:
		return nil, nil, fmt.Errorf("unsupported algorithm %q", options.Algorithm)
	}
}

func resolveCurve(name string) (elliptic.Curve, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case curveP256:
		return elliptic.P256(), nil
	case curveP384:
		return elliptic.P384(), nil
	case curveP521:
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported ecc curve %q", name)
	}
}

func (generator *Generator) buildCertificate(randomSource io.Reader, options Options, publicKey any, privateKey any) ([]byte, error) {
	serialNumber, err := rand.Int(randomSource, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial number: %w", err)
	}

	notBefore := generator.nowFn().UTC()
	notAfter := notBefore.Add(time.Duration(options.ValidForDays) * 24 * time.Hour)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   options.CommonName,
			Organization: []string{options.Organization},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              sanitizeStrings(options.DNSNames),
		IPAddresses:           parseIPAddresses(options.IPAddresses),
		IsCA:                  options.IsCA,
	}

	if options.Algorithm == algorithmRSA {
		template.KeyUsage |= x509.KeyUsageKeyEncipherment
	}
	if options.IsCA {
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	der, err := x509.CreateCertificate(randomSource, template, template, publicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

func sanitizeStrings(values []string) []string {
	var sanitized []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			sanitized = append(sanitized, value)
		}
	}
	return sanitized
}

func parseIPAddresses(values []string) []net.IP {
	var ips []net.IP
	for _, value := range values {
		if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
			ips = append(ips, ip)
		}
	}
	return ips
}

func encodePrivateKeyPEM(privateKey any) ([]byte, error) {
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}), nil
}

func encodePublicKeyPEM(publicKey any) ([]byte, error) {
	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER}), nil
}

type saltedReader struct {
	seed    []byte
	salt    []byte
	counter uint64
	buffer  []byte
}

func (reader *saltedReader) Read(p []byte) (int, error) {
	total := 0
	for total < len(p) {
		if len(reader.buffer) == 0 {
			reader.buffer = reader.nextBlock()
		}

		copied := copy(p[total:], reader.buffer)
		total += copied
		reader.buffer = reader.buffer[copied:]
	}
	return total, nil
}

func (reader *saltedReader) nextBlock() []byte {
	hash := sha256.New()
	hash.Write(reader.seed)
	hash.Write(reader.salt)
	hash.Write([]byte{
		byte(reader.counter >> 56),
		byte(reader.counter >> 48),
		byte(reader.counter >> 40),
		byte(reader.counter >> 32),
		byte(reader.counter >> 24),
		byte(reader.counter >> 16),
		byte(reader.counter >> 8),
		byte(reader.counter),
	})
	reader.counter++
	return hash.Sum(nil)
}
