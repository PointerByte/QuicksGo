// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package jwt

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	rsautil "github.com/PointerByte/QuicksGo/security/encrypt/asymmetry/rsa"
	signutil "github.com/PointerByte/QuicksGo/security/encrypt/asymmetry/signs"
	"github.com/spf13/viper"
)

var (
	ErrNilStrategy            = errors.New("jwt: signing strategy is required")
	ErrMissingSecret          = errors.New("jwt: secret is required")
	ErrMissingPrivateKey      = errors.New("jwt: private key is required")
	ErrMissingPublicKey       = errors.New("jwt: public key is required")
	ErrMissingEdDSAPrivateKey = errors.New("jwt: ed25519 private key is required")
	ErrMissingEdDSAPublicKey  = errors.New("jwt: ed25519 public key is required")
	ErrInvalidToken           = errors.New("jwt: invalid token format")
	ErrInvalidSignature       = errors.New("jwt: invalid token signature")
	ErrUnexpectedAlg          = errors.New("jwt: unexpected algorithm")
	ErrNilDestination         = errors.New("jwt: destination is required")
	ErrNilValidator           = errors.New("jwt: validator cannot be nil")
	ErrMissingValidation      = errors.New("jwt: validation callback is required")
	ErrUnsupportedAlg         = errors.New("jwt: unsupported algorithm")
)

const (
	DefaultAlgorithmKey       = "jwt.algorithm"
	DefaultHMACSecretKey      = "jwt.hmac.secret"
	DefaultRSAPrivateKeyKey   = "jwt.rsa.private_key"
	DefaultRSAPublicKeyKey    = "jwt.rsa.public_key"
	DefaultEdDSAPrivateKeyKey = "jwt.eddsa.private_key"
	DefaultEdDSAPublicKeyKey  = "jwt.eddsa.public_key"
)

// SetJWTAsymmetricKeys stores the configured asymmetric key pair in viper using
// the standard JWT config keys for the selected algorithm.
// Existing values are overwritten with Set, while previously unset values are
// registered with SetDefault so package defaults remain discoverable.
// Supported algorithms are "rsa" and "eddsa", matched case-insensitively.
func SetJWTAsymmetricKeys(priv, pub, alg string) error {
	switch strings.ToLower(strings.TrimSpace(alg)) {
	case "rsa":
		setJWTConfigValue(DefaultRSAPrivateKeyKey, priv)
		setJWTConfigValue(DefaultRSAPublicKeyKey, pub)
	case "eddsa":
		setJWTConfigValue(DefaultEdDSAPrivateKeyKey, priv)
		setJWTConfigValue(DefaultEdDSAPublicKeyKey, pub)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedAlg, alg)
	}
	return nil
}

// Strategy defines how a JWT is signed and how its signature is verified.
// Different algorithms can be supported by providing alternative implementations.
type Strategy interface {
	Algorithm() string
	Sign(signingInput []byte) ([]byte, error)
	Verify(signingInput []byte, signature []byte) error
}

// Validator represents an additional validation step executed after the token
// signature has been verified and its claims have been decoded.
type Validator func(ctx context.Context, token Token) error

// Option configures a Service instance during construction.
type Option func(*Service) error

// Service is a facade over JWT creation and validation.
// It uses a Strategy for cryptographic concerns and a validation pipeline for
// domain-specific checks.
type Service struct {
	strategy   Strategy
	validators []Validator
}

// Header represents the standard JWT header section.
type Header struct {
	Type      string `json:"typ"`
	Algorithm string `json:"alg"`
}

// Token contains the parsed pieces of a validated JWT.
type Token struct {
	Raw       string
	Header    Header
	Claims    json.RawMessage
	Signature string
}

type hmacSHA256Strategy struct {
	secret []byte
}

type rsaSHA256Strategy struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

type rsaPSSSHA256Strategy struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

type ed25519Strategy struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// HMACServiceInput configures NewHMACService.
// SecretEnv keeps its original name for compatibility and is treated as a
// viper key when Secret is empty.
// Validator is optional.
type HMACServiceInput struct {
	Secret    string
	SecretEnv string
	Validator Validator
}

// RSAServiceInput configures NewRSAService.
// PrivateKeyEnv and PublicKeyEnv keep their original names for compatibility
// and are treated as viper keys when the corresponding key is nil.
// Validator is optional.
type RSAServiceInput struct {
	PrivateKey    *rsa.PrivateKey
	PublicKey     *rsa.PublicKey
	PrivateKeyEnv string
	PublicKeyEnv  string
	Validator     Validator
}

// Ed25519ServiceInput configures NewEd25519Service.
type Ed25519ServiceInput struct {
	PrivateKey    ed25519.PrivateKey
	PublicKey     ed25519.PublicKey
	PrivateKeyEnv string
	PublicKeyEnv  string
	Validator     Validator
}

// ConfigServiceInput configures NewConfiguredService.
// When Algorithm is empty, the constructor reads it from viper.
// Validator is optional.
type ConfigServiceInput struct {
	Algorithm          string
	AlgorithmKey       string
	HMACSecretKey      string
	RSAPrivateKeyKey   string
	RSAPublicKeyKey    string
	EdDSAPrivateKeyKey string
	EdDSAPublicKeyKey  string
	Validator          Validator
}

// New builds a JWT service from the provided options.
// A signing strategy must be configured, either directly with WithStrategy or
// through a convenience option such as WithHMACSHA256.
func New(options ...Option) (*Service, error) {
	service := &Service{}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(service); err != nil {
			return nil, err
		}
	}
	if service.strategy == nil {
		return nil, ErrNilStrategy
	}
	return service, nil
}

// NewConfiguredService builds a JWT service based on the configured algorithm.
// It reads values from viper using the provided keys or the package defaults.
func NewConfiguredService(input ConfigServiceInput) (*Service, error) {
	algorithm := strings.ToUpper(strings.TrimSpace(input.Algorithm))
	if algorithm == "" {
		algorithm = strings.ToUpper(strings.TrimSpace(viper.GetString(stringOrDefault(input.AlgorithmKey, DefaultAlgorithmKey))))
	}

	switch algorithm {
	case "HS256":
		return NewHMACService(HMACServiceInput{
			SecretEnv: stringOrDefault(input.HMACSecretKey, DefaultHMACSecretKey),
			Validator: input.Validator,
		})
	case "RS256":
		return NewRSAService(RSAServiceInput{
			PrivateKeyEnv: stringOrDefault(input.RSAPrivateKeyKey, DefaultRSAPrivateKeyKey),
			PublicKeyEnv:  stringOrDefault(input.RSAPublicKeyKey, DefaultRSAPublicKeyKey),
			Validator:     input.Validator,
		})
	case "PS256":
		return NewRSAPSSService(RSAServiceInput{
			PrivateKeyEnv: stringOrDefault(input.RSAPrivateKeyKey, DefaultRSAPrivateKeyKey),
			PublicKeyEnv:  stringOrDefault(input.RSAPublicKeyKey, DefaultRSAPublicKeyKey),
			Validator:     input.Validator,
		})
	case "EDDSA":
		return NewEd25519Service(Ed25519ServiceInput{
			PrivateKeyEnv: stringOrDefault(input.EdDSAPrivateKeyKey, DefaultEdDSAPrivateKeyKey),
			PublicKeyEnv:  stringOrDefault(input.EdDSAPublicKeyKey, DefaultEdDSAPublicKeyKey),
			Validator:     input.Validator,
		})
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlg, algorithm)
	}
}

// NewRSAPSSService builds a JWT service for PS256 signatures.
func NewRSAPSSService(input RSAServiceInput) (*Service, error) {
	privateKey := input.PrivateKey
	privateKeyConfig := stringOrDefault(input.PrivateKeyEnv, DefaultRSAPrivateKeyKey)
	if privateKey == nil {
		value := viper.GetString(privateKeyConfig)
		if value != "" {
			parsedKey, err := rsautil.ParseRSAPrivateKeyFromBase64(value)
			if err != nil {
				return nil, fmt.Errorf("jwt: parse rsa private key from key %s: %w", privateKeyConfig, err)
			}
			privateKey = parsedKey
		}
	}

	publicKey := input.PublicKey
	publicKeyConfig := stringOrDefault(input.PublicKeyEnv, DefaultRSAPublicKeyKey)
	if publicKey == nil {
		value := viper.GetString(publicKeyConfig)
		if value != "" {
			parsedKey, err := rsautil.ParseRSAPublicKeyFromBase64(value)
			if err != nil {
				return nil, fmt.Errorf("jwt: parse rsa public key from key %s: %w", publicKeyConfig, err)
			}
			publicKey = parsedKey
		}
	}

	options := []Option{WithRSAPSSSHA256(privateKey, publicKey)}
	if input.Validator != nil {
		options = append(options, WithValidator(input.Validator))
	}
	return New(options...)
}

// NewEd25519Service builds a JWT service for EdDSA signatures.
func NewEd25519Service(input Ed25519ServiceInput) (*Service, error) {
	privateKey := input.PrivateKey
	privateKeyConfig := stringOrDefault(input.PrivateKeyEnv, DefaultEdDSAPrivateKeyKey)
	if privateKey == nil {
		value := viper.GetString(privateKeyConfig)
		if value != "" {
			parsedKey, err := signutil.ParseEd25519PrivateKeyFromBase64(value)
			if err != nil {
				return nil, fmt.Errorf("jwt: parse ed25519 private key from key %s: %w", privateKeyConfig, err)
			}
			privateKey = parsedKey
		}
	}

	publicKey := input.PublicKey
	publicKeyConfig := stringOrDefault(input.PublicKeyEnv, DefaultEdDSAPublicKeyKey)
	if publicKey == nil {
		value := viper.GetString(publicKeyConfig)
		if value != "" {
			parsedKey, err := signutil.ParseEd25519PublicKeyFromBase64(value)
			if err != nil {
				return nil, fmt.Errorf("jwt: parse ed25519 public key from key %s: %w", publicKeyConfig, err)
			}
			publicKey = parsedKey
		}
	}

	options := []Option{WithEd25519(privateKey, publicKey)}
	if input.Validator != nil {
		options = append(options, WithValidator(input.Validator))
	}
	return New(options...)
}

// NewHMACService builds a JWT service for HS256 signatures.
// The secret can be provided directly or loaded from viper.
func NewHMACService(input HMACServiceInput) (*Service, error) {
	secret := input.Secret
	secretKey := stringOrDefault(input.SecretEnv, DefaultHMACSecretKey)
	if secret == "" {
		secret = viper.GetString(secretKey)
	}

	options := []Option{WithHMACSHA256(secret)}
	if input.Validator != nil {
		options = append(options, WithValidator(input.Validator))
	}
	return New(options...)
}

// NewRSAService builds a JWT service for RS256 signatures.
// Keys can be provided directly or loaded from Base64-encoded viper values
// containing PKCS#8 private and X.509 public keys.
func NewRSAService(input RSAServiceInput) (*Service, error) {
	privateKey := input.PrivateKey
	privateKeyConfig := stringOrDefault(input.PrivateKeyEnv, DefaultRSAPrivateKeyKey)
	if privateKey == nil {
		value := viper.GetString(privateKeyConfig)
		if value != "" {
			parsedKey, err := rsautil.ParseRSAPrivateKeyFromBase64(value)
			if err != nil {
				return nil, fmt.Errorf("jwt: parse rsa private key from key %s: %w", privateKeyConfig, err)
			}
			privateKey = parsedKey
		}
	}

	publicKey := input.PublicKey
	publicKeyConfig := stringOrDefault(input.PublicKeyEnv, DefaultRSAPublicKeyKey)
	if publicKey == nil {
		value := viper.GetString(publicKeyConfig)
		if value != "" {
			parsedKey, err := rsautil.ParseRSAPublicKeyFromBase64(value)
			if err != nil {
				return nil, fmt.Errorf("jwt: parse rsa public key from key %s: %w", publicKeyConfig, err)
			}
			publicKey = parsedKey
		}
	}

	options := []Option{WithRSASHA256(privateKey, publicKey)}
	if input.Validator != nil {
		options = append(options, WithValidator(input.Validator))
	}
	return New(options...)
}

// NewHMACSHA256 returns an HMAC-SHA256 signing strategy.
func NewHMACSHA256(secret string) Strategy {
	return &hmacSHA256Strategy{secret: []byte(secret)}
}

// NewRSASHA256 returns an RSA-SHA256 signing strategy.
// The private key is used to sign tokens and the public key is used to verify them.
func NewRSASHA256(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) Strategy {
	return &rsaSHA256Strategy{
		privateKey: privateKey,
		publicKey:  publicKey,
	}
}

// NewRSAPSSSHA256 returns an RSA-PSS SHA-256 signing strategy.
func NewRSAPSSSHA256(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) Strategy {
	return &rsaPSSSHA256Strategy{
		privateKey: privateKey,
		publicKey:  publicKey,
	}
}

// NewEd25519 returns an Ed25519 signing strategy.
func NewEd25519(privateKey ed25519.PrivateKey, publicKey ed25519.PublicKey) Strategy {
	return &ed25519Strategy{
		privateKey: privateKey,
		publicKey:  publicKey,
	}
}

// WithHMACSHA256 configures the service to use HMAC-SHA256 signatures.
func WithHMACSHA256(secret string) Option {
	return func(service *Service) error {
		if secret == "" {
			return ErrMissingSecret
		}

		service.strategy = NewHMACSHA256(secret)
		return nil
	}
}

// WithRSASHA256 configures the service to use RSA-SHA256 signatures.
func WithRSASHA256(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) Option {
	return func(service *Service) error {
		if privateKey == nil {
			return ErrMissingPrivateKey
		}
		if publicKey == nil {
			return ErrMissingPublicKey
		}

		service.strategy = NewRSASHA256(privateKey, publicKey)
		return nil
	}
}

// WithRSAPSSSHA256 configures the service to use RSA-PSS SHA-256 signatures.
func WithRSAPSSSHA256(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) Option {
	return func(service *Service) error {
		if privateKey == nil {
			return ErrMissingPrivateKey
		}
		if publicKey == nil {
			return ErrMissingPublicKey
		}

		service.strategy = NewRSAPSSSHA256(privateKey, publicKey)
		return nil
	}
}

// WithEd25519 configures the service to use Ed25519 signatures.
func WithEd25519(privateKey ed25519.PrivateKey, publicKey ed25519.PublicKey) Option {
	return func(service *Service) error {
		if len(privateKey) == 0 {
			return ErrMissingEdDSAPrivateKey
		}
		if len(publicKey) == 0 {
			return ErrMissingEdDSAPublicKey
		}

		service.strategy = NewEd25519(privateKey, publicKey)
		return nil
	}
}

// WithStrategy injects a custom signing strategy into the service.
func WithStrategy(strategy Strategy) Option {
	return func(service *Service) error {
		if strategy == nil {
			return ErrNilStrategy
		}
		service.strategy = strategy
		return nil
	}
}

// WithValidator registers a validator that will run on every Decode call after
// signature verification and claim decoding succeed.
func WithValidator(validator Validator) Option {
	return func(service *Service) error {
		if validator == nil {
			return ErrNilValidator
		}
		service.validators = append(service.validators, validator)
		return nil
	}
}

// Create builds and signs a JWT using the configured signing strategy.
// The token header is generated automatically, so callers only provide claims.
func (service *Service) Create(claims any) (string, error) {
	if service == nil || service.strategy == nil {
		return "", ErrNilStrategy
	}

	headerJSON, err := json.Marshal(Header{
		Type:      "JWT",
		Algorithm: service.strategy.Algorithm(),
	})
	if err != nil {
		return "", fmt.Errorf("jwt: encode header: %w", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("jwt: encode claims: %w", err)
	}

	headerPart := encodeSegment(headerJSON)
	claimsPart := encodeSegment(claimsJSON)
	signingInput := headerPart + "." + claimsPart

	signature, err := service.strategy.Sign([]byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("jwt: sign token: %w", err)
	}
	return signingInput + "." + encodeSegment(signature), nil
}

// ValidateSignature verifies the JWT structure, algorithm, and signature
// without decoding its claims into a destination value.
func (service *Service) ValidateSignature(token string) error {
	_, err := service.parseAndValidate(token)
	return err
}

// Read validates the token and unmarshals its claims into destination using a
// background context and the service-level validators.
func (service *Service) Read(token string, destination any) error {
	_, err := service.Decode(context.Background(), token, destination)
	return err
}

// Decode validates the token signature, unmarshals its claims into destination,
// and runs both service-level and per-call validators.
func (service *Service) Decode(ctx context.Context, token string, destination any, validators ...Validator) (Token, error) {
	if destination == nil {
		return Token{}, ErrNilDestination
	}

	parsedToken, err := service.parseAndValidate(token)
	if err != nil {
		return Token{}, err
	}

	if err := json.Unmarshal(parsedToken.Claims, destination); err != nil {
		return Token{}, fmt.Errorf("jwt: decode claims: %w", err)
	}

	allValidators := append(append([]Validator{}, service.validators...), validators...)
	for _, validator := range allValidators {
		if validator == nil {
			return Token{}, ErrMissingValidation
		}
		if err := validator(ctx, parsedToken); err != nil {
			return Token{}, err
		}
	}
	return parsedToken, nil
}

func (service *Service) parseAndValidate(token string) (Token, error) {
	if service == nil || service.strategy == nil {
		return Token{}, ErrNilStrategy
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Token{}, ErrInvalidToken
	}

	headerBytes, err := decodeSegment(parts[0])
	if err != nil {
		return Token{}, fmt.Errorf("jwt: decode header: %w", err)
	}

	var header Header
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return Token{}, fmt.Errorf("jwt: parse header: %w", err)
	}

	if header.Algorithm != service.strategy.Algorithm() {
		return Token{}, fmt.Errorf("%w: expected %s, got %s", ErrUnexpectedAlg, service.strategy.Algorithm(), header.Algorithm)
	}

	claimsBytes, err := decodeSegment(parts[1])
	if err != nil {
		return Token{}, fmt.Errorf("jwt: decode claims: %w", err)
	}

	signatureBytes, err := decodeSegment(parts[2])
	if err != nil {
		return Token{}, fmt.Errorf("jwt: decode signature: %w", err)
	}

	signingInput := []byte(parts[0] + "." + parts[1])
	if err := service.strategy.Verify(signingInput, signatureBytes); err != nil {
		return Token{}, err
	}
	return Token{
		Raw:       token,
		Header:    header,
		Claims:    claimsBytes,
		Signature: parts[2],
	}, nil
}

func (strategy *hmacSHA256Strategy) Algorithm() string {
	return "HS256"
}

func (strategy *hmacSHA256Strategy) Sign(signingInput []byte) ([]byte, error) {
	if len(strategy.secret) == 0 {
		return nil, ErrMissingSecret
	}

	mac := hmac.New(sha256.New, strategy.secret)
	if _, err := mac.Write(signingInput); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

func (strategy *hmacSHA256Strategy) Verify(signingInput []byte, signature []byte) error {
	expectedSignature, err := strategy.Sign(signingInput)
	if err != nil {
		return err
	}

	if !hmac.Equal(signature, expectedSignature) {
		return ErrInvalidSignature
	}
	return nil
}

func (strategy *rsaSHA256Strategy) Algorithm() string {
	return "RS256"
}

func (strategy *rsaSHA256Strategy) Sign(signingInput []byte) ([]byte, error) {
	if strategy.privateKey == nil {
		return nil, ErrMissingPrivateKey
	}
	return signutil.SignSHA256(signingInput, strategy.privateKey)
}

func (strategy *rsaSHA256Strategy) Verify(signingInput []byte, signature []byte) error {
	if strategy.publicKey == nil {
		return ErrMissingPublicKey
	}
	if err := signutil.VerifySHA256(signingInput, signature, strategy.publicKey); err != nil {
		return ErrInvalidSignature
	}

	return nil
}

func (strategy *rsaPSSSHA256Strategy) Algorithm() string {
	return "PS256"
}

func (strategy *rsaPSSSHA256Strategy) Sign(signingInput []byte) ([]byte, error) {
	if strategy.privateKey == nil {
		return nil, ErrMissingPrivateKey
	}

	signatureB64, err := signutil.SignRSAPSS(mustMarshalRSAPrivateKey(strategy.privateKey), string(signingInput))
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(signatureB64)
}

func (strategy *rsaPSSSHA256Strategy) Verify(signingInput []byte, signature []byte) error {
	if strategy.publicKey == nil {
		return ErrMissingPublicKey
	}

	signatureB64 := base64.StdEncoding.EncodeToString(signature)
	if err := signutil.VerifyRSAPSS(mustMarshalRSAPublicKey(strategy.publicKey), string(signingInput), signatureB64); err != nil {
		return ErrInvalidSignature
	}
	return nil
}

func (strategy *ed25519Strategy) Algorithm() string {
	return "EdDSA"
}

func (strategy *ed25519Strategy) Sign(signingInput []byte) ([]byte, error) {
	if len(strategy.privateKey) == 0 {
		return nil, ErrMissingEdDSAPrivateKey
	}

	signatureB64, err := signutil.SignEd25519(mustMarshalEd25519PrivateKey(strategy.privateKey), string(signingInput))
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(signatureB64)
}

func (strategy *ed25519Strategy) Verify(signingInput []byte, signature []byte) error {
	if len(strategy.publicKey) == 0 {
		return ErrMissingEdDSAPublicKey
	}

	signatureB64 := base64.StdEncoding.EncodeToString(signature)
	if err := signutil.VerifyEd25519(mustMarshalEd25519PublicKey(strategy.publicKey), string(signingInput), signatureB64); err != nil {
		return ErrInvalidSignature
	}
	return nil
}

func encodeSegment(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func decodeSegment(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}

func stringOrDefault(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func setJWTConfigValue(key string, value string) {
	if viper.IsSet(key) {
		viper.Set(key, value)
		return
	}
	viper.SetDefault(key, value)
}

func mustMarshalRSAPrivateKey(privateKey *rsa.PrivateKey) string {
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustMarshalRSAPublicKey(publicKey *rsa.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustMarshalEd25519PrivateKey(privateKey ed25519.PrivateKey) string {
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustMarshalEd25519PublicKey(publicKey ed25519.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}
