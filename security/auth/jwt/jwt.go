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
	"os"
	"strings"
	"time"

	"github.com/PointerByte/QuicksGo/encrypt"
	"github.com/PointerByte/QuicksGo/encrypt/local"
	"github.com/PointerByte/QuicksGo/encrypt/utilities"
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
	ErrMissingAlgorithm       = errors.New("jwt: algorithm is required")
	ErrMissingSignFunc        = errors.New("jwt: custom sign function is required")
	ErrMissingVerifyFunc      = errors.New("jwt: custom verify function is required")
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

type contextStrategy interface {
	SignContext(ctx context.Context, signingInput []byte) ([]byte, error)
	VerifyContext(ctx context.Context, signingInput []byte, signature []byte) error
}

// SignFunc signs a JWT signing input and returns the raw signature bytes.
type SignFunc func(ctx context.Context, signingInput []byte) ([]byte, error)

// VerifyFunc validates raw signature bytes for a JWT signing input.
type VerifyFunc func(ctx context.Context, signingInput []byte, signature []byte) error

// Validator represents an additional validation step executed after the token
// signature has been verified and its claims have been decoded.
type Validator func(ctx context.Context, token Token) error

// Option configures a Service instance during construction.
type Option func(*Service) error

// Service is a facade over JWT creation and validation.
// It uses a Strategy for cryptographic concerns and a validation pipeline for
// domain-specific checks.
type Service struct {
	// strategy signs tokens and verifies token signatures.
	strategy Strategy
	// validators run after signature verification and claim decoding.
	validators []Validator
	// contextTimeout limits signing, verification, and validation operations.
	contextTimeout time.Duration
	// contextTimeoutOn indicates whether contextTimeout should be applied.
	contextTimeoutOn bool
}

// Header represents the standard JWT header section.
type Header struct {
	// Type identifies the token type, usually "JWT".
	Type string `json:"typ"`
	// Algorithm identifies the signing algorithm used by the token.
	Algorithm string `json:"alg"`
}

// Token contains the parsed pieces of a validated JWT.
type Token struct {
	// Raw is the original compact JWT string.
	Raw string
	// Header contains the decoded JWT header.
	Header Header
	// Claims contains the decoded JWT claims payload.
	Claims json.RawMessage
	// Signature is the encoded JWT signature segment.
	Signature string
}

type hmacSHA256Strategy struct {
	// secret is the shared key used to sign and verify HS256 tokens.
	secret []byte
}

type rsaSHA256Strategy struct {
	// signutil performs RSA SHA-256 signing and verification.
	signutil encrypt.SignatureRepository
	// privateKey signs RS256 tokens.
	privateKey *rsa.PrivateKey
	// publicKey verifies RS256 token signatures.
	publicKey *rsa.PublicKey
}

type rsaPSSSHA256Strategy struct {
	// signutil performs RSA-PSS SHA-256 signing and verification.
	signutil encrypt.SignatureRepository
	// privateKey signs PS256 tokens.
	privateKey *rsa.PrivateKey
	// publicKey verifies PS256 token signatures.
	publicKey *rsa.PublicKey
}

type ed25519Strategy struct {
	// signutil performs Ed25519 signing and verification.
	signutil encrypt.SignatureRepository
	// privateKey signs EdDSA tokens.
	privateKey ed25519.PrivateKey
	// publicKey verifies EdDSA token signatures.
	publicKey ed25519.PublicKey
}

type customStrategy struct {
	// algorithm is the JWT alg header value reported by the strategy.
	algorithm string
	// sign creates a signature for the JWT signing input.
	sign SignFunc
	// verify validates a signature for the JWT signing input.
	verify VerifyFunc
}

// HMACServiceInput configures NewHMACService.
// SecretEnv keeps its original name for compatibility and is treated as a
// viper key when Secret is empty.
// Validator is optional.
type HMACServiceInput struct {
	// Secret is the HS256 shared key used directly when provided.
	Secret string
	// SecretEnv is the viper key used to read the shared key when Secret is empty.
	SecretEnv string
	// Validator optionally adds a post-signature validation callback.
	Validator Validator
	// Timeout optionally limits service operations when greater than zero.
	Timeout time.Duration
}

// RSAServiceInput configures NewRSAService.
// PrivateKeyEnv and PublicKeyEnv keep their original names for compatibility
// and are treated as viper keys when the corresponding key is nil.
// Validator is optional.
type RSAServiceInput struct {
	// PrivateKey signs RS256 tokens when provided.
	PrivateKey *rsa.PrivateKey
	// PublicKey verifies RS256 token signatures when provided.
	PublicKey *rsa.PublicKey
	// PrivateKeyEnv is the viper key used to read the private key when PrivateKey is nil.
	PrivateKeyEnv string
	// PublicKeyEnv is the viper key used to read the public key when PublicKey is nil.
	PublicKeyEnv string
	// Validator optionally adds a post-signature validation callback.
	Validator Validator
	// Timeout optionally limits service operations when greater than zero.
	Timeout time.Duration
}

// Ed25519ServiceInput configures NewEd25519Service.
type Ed25519ServiceInput struct {
	// PrivateKey signs EdDSA tokens when provided.
	PrivateKey ed25519.PrivateKey
	// PublicKey verifies EdDSA token signatures when provided.
	PublicKey ed25519.PublicKey
	// PrivateKeyEnv is the viper key used to read the private key when PrivateKey is nil.
	PrivateKeyEnv string
	// PublicKeyEnv is the viper key used to read the public key when PublicKey is nil.
	PublicKeyEnv string
	// Validator optionally adds a post-signature validation callback.
	Validator Validator
	// Timeout optionally limits service operations when greater than zero.
	Timeout time.Duration
}

// ConfigServiceInput configures NewConfiguredService.
// When Algorithm is empty, the constructor reads it from viper.
// Validator is optional.
type ConfigServiceInput struct {
	// Algorithm selects the signing strategy; when empty it is read from viper.
	Algorithm string
	// algorithmKey is the viper key used to read Algorithm when Algorithm is empty.
	algorithmKey string
	// HMACSecretKey is the viper key used to read the HS256 shared key.
	HMACSecretKey *string
	// RSAPrivateKeyKey is the viper key used to read the RSA private key.
	RSAPrivateKeyKey *string
	// RSAPublicKeyKey is the viper key used to read the RSA public key.
	RSAPublicKeyKey *string
	// EdDSAPrivateKeyKey is the viper key used to read the Ed25519 private key.
	EdDSAPrivateKeyKey *string
	// EdDSAPublicKeyKey is the viper key used to read the Ed25519 public key.
	EdDSAPublicKeyKey *string
	// Validator optionally adds a post-signature validation callback.
	Validator Validator
	// Timeout optionally limits service operations when greater than zero.
	Timeout time.Duration
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
		algorithm = strings.ToUpper(strings.TrimSpace(viper.GetString(stringOrDefault(input.algorithmKey, DefaultAlgorithmKey))))
	}

	switch algorithm {
	case "HS256":
		return NewHMACService(HMACServiceInput{
			SecretEnv: stringPtrOrDefault(input.HMACSecretKey, DefaultHMACSecretKey),
			Validator: input.Validator,
			Timeout:   input.Timeout,
		})
	case "RS256":
		return NewRSAService(RSAServiceInput{
			PrivateKeyEnv: stringPtrOrDefault(input.RSAPrivateKeyKey, DefaultRSAPrivateKeyKey),
			PublicKeyEnv:  stringPtrOrDefault(input.RSAPublicKeyKey, DefaultRSAPublicKeyKey),
			Validator:     input.Validator,
			Timeout:       input.Timeout,
		})
	case "PS256":
		return NewRSAPSSService(RSAServiceInput{
			PrivateKeyEnv: stringPtrOrDefault(input.RSAPrivateKeyKey, DefaultRSAPrivateKeyKey),
			PublicKeyEnv:  stringPtrOrDefault(input.RSAPublicKeyKey, DefaultRSAPublicKeyKey),
			Validator:     input.Validator,
			Timeout:       input.Timeout,
		})
	case "EDDSA":
		return NewEd25519Service(Ed25519ServiceInput{
			PrivateKeyEnv: stringPtrOrDefault(input.EdDSAPrivateKeyKey, DefaultEdDSAPrivateKeyKey),
			PublicKeyEnv:  stringPtrOrDefault(input.EdDSAPublicKeyKey, DefaultEdDSAPublicKeyKey),
			Validator:     input.Validator,
			Timeout:       input.Timeout,
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
			parsedKey, err := parseRSAPrivateKeyFromConfig(value)
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
			parsedKey, err := parseRSAPublicKeyFromConfig(value)
			if err != nil {
				return nil, fmt.Errorf("jwt: parse rsa public key from key %s: %w", publicKeyConfig, err)
			}
			publicKey = parsedKey
		}
	}

	options := []Option{WithRSAPSSSHA256(privateKey, publicKey)}
	if input.Timeout > 0 {
		options = append(options, WithContextTimeout(input.Timeout))
	}
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
			parsedKey, err := parseEd25519PrivateKeyFromConfig(value)
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
			parsedKey, err := parseEd25519PublicKeyFromConfig(value)
			if err != nil {
				return nil, fmt.Errorf("jwt: parse ed25519 public key from key %s: %w", publicKeyConfig, err)
			}
			publicKey = parsedKey
		}
	}

	options := []Option{WithEd25519(privateKey, publicKey)}
	if input.Timeout > 0 {
		options = append(options, WithContextTimeout(input.Timeout))
	}
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
	if input.Timeout > 0 {
		options = append(options, WithContextTimeout(input.Timeout))
	}
	if input.Validator != nil {
		options = append(options, WithValidator(input.Validator))
	}
	return New(options...)
}

// NewRSAService builds a JWT service for RS256 signatures.
// Keys can be provided directly or loaded from viper values. Configured values
// may point to PEM files containing PKCS#8 private and X.509 public keys, or
// keep the previous Base64-encoded DER format for compatibility.
func NewRSAService(input RSAServiceInput) (*Service, error) {
	privateKey := input.PrivateKey
	privateKeyConfig := stringOrDefault(input.PrivateKeyEnv, DefaultRSAPrivateKeyKey)
	if privateKey == nil {
		value := viper.GetString(privateKeyConfig)
		if value != "" {
			parsedKey, err := parseRSAPrivateKeyFromConfig(value)
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
			parsedKey, err := parseRSAPublicKeyFromConfig(value)
			if err != nil {
				return nil, fmt.Errorf("jwt: parse rsa public key from key %s: %w", publicKeyConfig, err)
			}
			publicKey = parsedKey
		}
	}

	options := []Option{WithRSASHA256(privateKey, publicKey)}
	if input.Timeout > 0 {
		options = append(options, WithContextTimeout(input.Timeout))
	}
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
		signutil:   encrypt.NewRepository(local.NewRepository()),
		privateKey: privateKey,
		publicKey:  publicKey,
	}
}

// NewRSAPSSSHA256 returns an RSA-PSS SHA-256 signing strategy.
func NewRSAPSSSHA256(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) Strategy {
	return &rsaPSSSHA256Strategy{
		signutil:   encrypt.NewRepository(local.NewRepository()),
		privateKey: privateKey,
		publicKey:  publicKey,
	}
}

// NewEd25519 returns an Ed25519 signing strategy.
func NewEd25519(privateKey ed25519.PrivateKey, publicKey ed25519.PublicKey) Strategy {
	return &ed25519Strategy{
		signutil:   encrypt.NewRepository(local.NewRepository()),
		privateKey: privateKey,
		publicKey:  publicKey,
	}
}

// NewCustomStrategy returns a signing strategy backed by caller-provided
// signing and verification functions.
func NewCustomStrategy(algorithm string, sign SignFunc, verify VerifyFunc) (Strategy, error) {
	algorithm = strings.TrimSpace(algorithm)
	if algorithm == "" {
		return nil, ErrMissingAlgorithm
	}
	if sign == nil {
		return nil, ErrMissingSignFunc
	}
	if verify == nil {
		return nil, ErrMissingVerifyFunc
	}
	return &customStrategy{
		algorithm: algorithm,
		sign:      sign,
		verify:    verify,
	}, nil
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

// WithCustomStrategy configures the service to sign and verify tokens with
// caller-provided functions.
func WithCustomStrategy(algorithm string, sign SignFunc, verify VerifyFunc) Option {
	return func(service *Service) error {
		strategy, err := NewCustomStrategy(algorithm, sign, verify)
		if err != nil {
			return err
		}
		service.strategy = strategy
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

// WithContextTimeout configures a maximum duration for JWT signing,
// signature verification, and validation work performed by the service.
func WithContextTimeout(timeout time.Duration) Option {
	return func(service *Service) error {
		if timeout <= 0 {
			service.contextTimeout = 0
			service.contextTimeoutOn = false
			return nil
		}
		service.contextTimeout = timeout
		service.contextTimeoutOn = true
		return nil
	}
}

// Create builds and signs a JWT using the configured signing strategy.
// The token header is generated automatically, so callers only provide claims.
func (service *Service) Create(claims any) (string, error) {
	return service.CreateWithContext(context.Background(), claims)
}

// CreateWithContext builds and signs a JWT using ctx plus any service timeout.
func (service *Service) CreateWithContext(ctx context.Context, claims any) (string, error) {
	if service == nil || service.strategy == nil {
		return "", ErrNilStrategy
	}
	ctx, cancel := service.contextFor(ctx)
	defer cancel()

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

	signature, err := signStrategy(ctx, service.strategy, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("jwt: sign token: %w", err)
	}
	return signingInput + "." + encodeSegment(signature), nil
}

// ValidateSignature verifies the JWT structure, algorithm, and signature
// without decoding its claims into a destination value.
func (service *Service) ValidateSignature(token string) error {
	return service.ValidateSignatureWithContext(context.Background(), token)
}

// ValidateSignatureWithContext verifies the JWT signature using ctx plus any
// service timeout.
func (service *Service) ValidateSignatureWithContext(ctx context.Context, token string) error {
	ctx, cancel := service.contextFor(ctx)
	defer cancel()
	_, err := service.parseAndValidate(ctx, token)
	return err
}

// Read validates the token and unmarshals its claims into destination using a
// background context and the service-level validators.
func (service *Service) Read(token string, destination any) error {
	return service.ReadWithContext(context.Background(), token, destination)
}

// ReadWithContext validates the token and unmarshals its claims into
// destination using ctx plus any service timeout.
func (service *Service) ReadWithContext(ctx context.Context, token string, destination any) error {
	_, err := service.Decode(ctx, token, destination)
	return err
}

// Decode validates the token signature, unmarshals its claims into destination,
// and runs both service-level and per-call validators.
func (service *Service) Decode(ctx context.Context, token string, destination any, validators ...Validator) (*Token, error) {
	if destination == nil {
		return nil, ErrNilDestination
	}
	ctx, cancel := service.contextFor(ctx)
	defer cancel()

	parsedToken, err := service.parseAndValidate(ctx, token)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(parsedToken.Claims, destination); err != nil {
		return nil, fmt.Errorf("jwt: decode claims: %w", err)
	}

	allValidators := append(append([]Validator{}, service.validators...), validators...)
	for _, validator := range allValidators {
		if validator == nil {
			return nil, ErrMissingValidation
		}
		if err := validator(ctx, *parsedToken); err != nil {
			return nil, err
		}
	}
	return parsedToken, nil
}

func (service *Service) parseAndValidate(ctx context.Context, token string) (*Token, error) {
	if service == nil || service.strategy == nil {
		return nil, ErrNilStrategy
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	headerBytes, err := decodeSegment(parts[0])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode header: %w", err)
	}

	var header Header
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("jwt: parse header: %w", err)
	}

	if header.Algorithm != service.strategy.Algorithm() {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrUnexpectedAlg, service.strategy.Algorithm(), header.Algorithm)
	}

	claimsBytes, err := decodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode claims: %w", err)
	}

	signatureBytes, err := decodeSegment(parts[2])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode signature: %w", err)
	}

	signingInput := []byte(parts[0] + "." + parts[1])
	if err := verifyStrategy(ctx, service.strategy, signingInput, signatureBytes); err != nil {
		return nil, err
	}
	return &Token{
		Raw:       token,
		Header:    header,
		Claims:    claimsBytes,
		Signature: parts[2],
	}, nil
}

func (service *Service) contextFor(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if service == nil || !service.contextTimeoutOn {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, service.contextTimeout)
}

func signStrategy(ctx context.Context, strategy Strategy, signingInput []byte) ([]byte, error) {
	if contextAware, ok := strategy.(contextStrategy); ok {
		return contextAware.SignContext(ctx, signingInput)
	}
	return runStrategyWithContext(ctx, func() ([]byte, error) {
		return strategy.Sign(signingInput)
	})
}

func verifyStrategy(ctx context.Context, strategy Strategy, signingInput []byte, signature []byte) error {
	if contextAware, ok := strategy.(contextStrategy); ok {
		return contextAware.VerifyContext(ctx, signingInput, signature)
	}
	_, err := runStrategyWithContext(ctx, func() (struct{}, error) {
		return struct{}{}, strategy.Verify(signingInput, signature)
	})
	return err
}

type strategyResult[T any] struct {
	// value is the result returned by the strategy operation.
	value T
	// err is the error returned by the strategy operation.
	err error
}

func runStrategyWithContext[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	done := make(chan strategyResult[T], 1)
	go func() {
		value, err := fn()
		done <- strategyResult[T]{value: value, err: err}
	}()

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case result := <-done:
		return result.value, result.err
	}
}

func (strategy *hmacSHA256Strategy) Algorithm() string {
	return "HS256"
}

func (strategy *hmacSHA256Strategy) Sign(signingInput []byte) ([]byte, error) {
	return strategy.SignContext(context.Background(), signingInput)
}

func (strategy *hmacSHA256Strategy) SignContext(ctx context.Context, signingInput []byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
	return strategy.VerifyContext(context.Background(), signingInput, signature)
}

func (strategy *hmacSHA256Strategy) VerifyContext(ctx context.Context, signingInput []byte, signature []byte) error {
	expectedSignature, err := strategy.SignContext(ctx, signingInput)
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
	return strategy.SignContext(context.Background(), signingInput)
}

func (strategy *rsaSHA256Strategy) SignContext(ctx context.Context, signingInput []byte) ([]byte, error) {
	if strategy.privateKey == nil {
		return nil, ErrMissingPrivateKey
	}
	signatureB64, err := strategy.signutil.Sign_RSA_PKCS1v15_SHA256(ctx, mustMarshalRSAPrivateKey(strategy.privateKey), string(signingInput))
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(signatureB64)
}

func (strategy *rsaSHA256Strategy) Verify(signingInput []byte, signature []byte) error {
	return strategy.VerifyContext(context.Background(), signingInput, signature)
}

func (strategy *rsaSHA256Strategy) VerifyContext(ctx context.Context, signingInput []byte, signature []byte) error {
	if strategy.publicKey == nil {
		return ErrMissingPublicKey
	}
	if err := strategy.signutil.Verify_RSA_PKCS1v15_SHA256(ctx, string(signingInput), mustMarshalRSAPublicKey(strategy.publicKey), base64.StdEncoding.EncodeToString(signature)); err != nil {
		return signatureError(err)
	}
	return nil
}

func (strategy *rsaPSSSHA256Strategy) Algorithm() string {
	return "PS256"
}

func (strategy *rsaPSSSHA256Strategy) Sign(signingInput []byte) ([]byte, error) {
	return strategy.SignContext(context.Background(), signingInput)
}

func (strategy *rsaPSSSHA256Strategy) SignContext(ctx context.Context, signingInput []byte) ([]byte, error) {
	if strategy.privateKey == nil {
		return nil, ErrMissingPrivateKey
	}

	signatureB64, err := strategy.signutil.SignRSAPSS(ctx, mustMarshalRSAPrivateKey(strategy.privateKey), string(signingInput))
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(signatureB64)
}

func (strategy *rsaPSSSHA256Strategy) Verify(signingInput []byte, signature []byte) error {
	return strategy.VerifyContext(context.Background(), signingInput, signature)
}

func (strategy *rsaPSSSHA256Strategy) VerifyContext(ctx context.Context, signingInput []byte, signature []byte) error {
	if strategy.publicKey == nil {
		return ErrMissingPublicKey
	}

	if err := strategy.signutil.VerifyRSAPSS(ctx, mustMarshalRSAPublicKey(strategy.publicKey), string(signingInput), base64.StdEncoding.EncodeToString(signature)); err != nil {
		return signatureError(err)
	}
	return nil
}

func (strategy *ed25519Strategy) Algorithm() string {
	return "EdDSA"
}

func (strategy *ed25519Strategy) Sign(signingInput []byte) ([]byte, error) {
	return strategy.SignContext(context.Background(), signingInput)
}

func (strategy *ed25519Strategy) SignContext(ctx context.Context, signingInput []byte) ([]byte, error) {
	if len(strategy.privateKey) == 0 {
		return nil, ErrMissingEdDSAPrivateKey
	}

	signatureB64, err := strategy.signutil.SignEd25519(ctx, mustMarshalEd25519PrivateKey(strategy.privateKey), string(signingInput))
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(signatureB64)
}

func (strategy *ed25519Strategy) Verify(signingInput []byte, signature []byte) error {
	return strategy.VerifyContext(context.Background(), signingInput, signature)
}

func (strategy *ed25519Strategy) VerifyContext(ctx context.Context, signingInput []byte, signature []byte) error {
	if len(strategy.publicKey) == 0 {
		return ErrMissingEdDSAPublicKey
	}

	if err := strategy.signutil.VerifyEd25519(ctx, mustMarshalEd25519PublicKey(strategy.publicKey), string(signingInput), base64.StdEncoding.EncodeToString(signature)); err != nil {
		return signatureError(err)
	}
	return nil
}

func (strategy *customStrategy) Algorithm() string {
	return strategy.algorithm
}

func (strategy *customStrategy) Sign(signingInput []byte) ([]byte, error) {
	return strategy.SignContext(context.Background(), signingInput)
}

func (strategy *customStrategy) SignContext(ctx context.Context, signingInput []byte) ([]byte, error) {
	if strategy.sign == nil {
		return nil, ErrMissingSignFunc
	}
	return strategy.sign(ctx, signingInput)
}

func (strategy *customStrategy) Verify(signingInput []byte, signature []byte) error {
	return strategy.VerifyContext(context.Background(), signingInput, signature)
}

func (strategy *customStrategy) VerifyContext(ctx context.Context, signingInput []byte, signature []byte) error {
	if strategy.verify == nil {
		return ErrMissingVerifyFunc
	}
	return strategy.verify(ctx, signingInput, signature)
}

func signatureError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return ErrInvalidSignature
}

func encodeSegment(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func decodeSegment(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}

func parseRSAPrivateKeyFromConfig(value string) (*rsa.PrivateKey, error) {
	if shouldReadPEMFile(value) {
		return utilities.ParseRSAPrivateKeyFromPEMFile(value)
	}
	return utilities.ParseRSAPrivateKeyFromBase64(value)
}

func parseRSAPublicKeyFromConfig(value string) (*rsa.PublicKey, error) {
	if shouldReadPEMFile(value) {
		return utilities.ParseRSAPublicKeyFromPEMFile(value)
	}
	return utilities.ParseRSAPublicKeyFromBase64(value)
}

func parseEd25519PrivateKeyFromConfig(value string) (ed25519.PrivateKey, error) {
	if shouldReadPEMFile(value) {
		return utilities.ParseEd25519PrivateKeyFromPEMFile(value)
	}
	return utilities.ParseEd25519PrivateKeyFromBase64(value)
}

func parseEd25519PublicKeyFromConfig(value string) (ed25519.PublicKey, error) {
	if shouldReadPEMFile(value) {
		return utilities.ParseEd25519PublicKeyFromPEMFile(value)
	}
	return utilities.ParseEd25519PublicKeyFromBase64(value)
}

func shouldReadPEMFile(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lowerValue := strings.ToLower(value)
	if strings.HasSuffix(lowerValue, ".pem") {
		return true
	}
	if _, err := os.Stat(value); err == nil {
		return true
	}
	return false
}

func stringOrDefault(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func stringPtrOrDefault(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return stringOrDefault(*value, fallback)
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
