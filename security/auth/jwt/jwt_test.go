// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package jwt

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

type testClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Active bool   `json:"active"`
}

func TestSetJWTAsymmetricKeys(t *testing.T) {
	t.Run("rsa uses defaults when keys are unset", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(viper.Reset)

		if err := SetJWTAsymmetricKeys("rsa-private", "rsa-public", "rsa"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if got := viper.GetString(DefaultRSAPrivateKeyKey); got != "rsa-private" {
			t.Fatalf("expected private key %q, got %q", "rsa-private", got)
		}
		if got := viper.GetString(DefaultRSAPublicKeyKey); got != "rsa-public" {
			t.Fatalf("expected public key %q, got %q", "rsa-public", got)
		}
	})

	t.Run("rsa overwrites existing values", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(viper.Reset)
		viper.Set(DefaultRSAPrivateKeyKey, "old-private")
		viper.Set(DefaultRSAPublicKeyKey, "old-public")

		if err := SetJWTAsymmetricKeys("new-private", "new-public", "RSA"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if got := viper.GetString(DefaultRSAPrivateKeyKey); got != "new-private" {
			t.Fatalf("expected private key %q, got %q", "new-private", got)
		}
		if got := viper.GetString(DefaultRSAPublicKeyKey); got != "new-public" {
			t.Fatalf("expected public key %q, got %q", "new-public", got)
		}
	})

	t.Run("eddsa uses defaults when keys are unset", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(viper.Reset)

		if err := SetJWTAsymmetricKeys("eddsa-private", "eddsa-public", " eddsa "); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if got := viper.GetString(DefaultEdDSAPrivateKeyKey); got != "eddsa-private" {
			t.Fatalf("expected private key %q, got %q", "eddsa-private", got)
		}
		if got := viper.GetString(DefaultEdDSAPublicKeyKey); got != "eddsa-public" {
			t.Fatalf("expected public key %q, got %q", "eddsa-public", got)
		}
	})

	t.Run("eddsa overwrites existing values", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(viper.Reset)
		viper.Set(DefaultEdDSAPrivateKeyKey, "old-private")
		viper.Set(DefaultEdDSAPublicKeyKey, "old-public")

		if err := SetJWTAsymmetricKeys("new-private", "new-public", "EdDSA"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if got := viper.GetString(DefaultEdDSAPrivateKeyKey); got != "new-private" {
			t.Fatalf("expected private key %q, got %q", "new-private", got)
		}
		if got := viper.GetString(DefaultEdDSAPublicKeyKey); got != "new-public" {
			t.Fatalf("expected public key %q, got %q", "new-public", got)
		}
	})

	t.Run("unsupported algorithm returns error", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(viper.Reset)

		err := SetJWTAsymmetricKeys("private", "public", "ecdsa")
		if !errors.Is(err, ErrUnsupportedAlg) {
			t.Fatalf("expected ErrUnsupportedAlg, got %v", err)
		}
	})
}

func TestCreateValidateAndReadJWT(t *testing.T) {
	service, err := New(WithHMACSHA256("super-secret"))
	if err != nil {
		t.Fatalf("expected service without error, got %v", err)
	}

	wantClaims := testClaims{
		UserID: "42",
		Role:   "admin",
		Active: true,
	}

	token, err := service.Create(wantClaims)
	if err != nil {
		t.Fatalf("expected token without error, got %v", err)
	}

	if err := service.ValidateSignature(token); err != nil {
		t.Fatalf("expected valid signature, got %v", err)
	}

	var gotClaims testClaims
	if err := service.Read(token, &gotClaims); err != nil {
		t.Fatalf("expected read without error, got %v", err)
	}

	if gotClaims != wantClaims {
		t.Fatalf("expected claims %+v, got %+v", wantClaims, gotClaims)
	}
}

func TestDecodeRunsServiceAndInlineValidators(t *testing.T) {
	const tokenUserID = "db-user"

	service, err := New(
		WithHMACSHA256("another-secret"),
		WithValidator(func(ctx context.Context, token Token) error {
			if token.Header.Type != "JWT" {
				return errors.New("unexpected token type")
			}
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("expected service without error, got %v", err)
	}

	token, err := service.Create(testClaims{
		UserID: tokenUserID,
		Role:   "reader",
		Active: true,
	})
	if err != nil {
		t.Fatalf("expected token without error, got %v", err)
	}

	var gotClaims testClaims
	visitedDB := false

	parsedToken, err := service.Decode(context.Background(), token, &gotClaims, func(ctx context.Context, token Token) error {
		visitedDB = true

		var claims testClaims
		if err := unmarshalClaims(token.Claims, &claims); err != nil {
			return err
		}

		if claims.UserID != tokenUserID {
			return errors.New("user not found in db")
		}

		return nil
	})
	if err != nil {
		t.Fatalf("expected decode without error, got %v", err)
	}

	if !visitedDB {
		t.Fatal("expected inline validator to run")
	}

	if parsedToken.Header.Algorithm != "HS256" {
		t.Fatalf("expected algorithm HS256, got %s", parsedToken.Header.Algorithm)
	}

	if gotClaims.UserID != tokenUserID {
		t.Fatalf("expected user id %q, got %q", tokenUserID, gotClaims.UserID)
	}
}

func TestValidateSignatureFailsWithTamperedToken(t *testing.T) {
	service, err := New(WithHMACSHA256("super-secret"))
	if err != nil {
		t.Fatalf("expected service without error, got %v", err)
	}

	token, err := service.Create(testClaims{
		UserID: "42",
		Role:   "admin",
		Active: true,
	})
	if err != nil {
		t.Fatalf("expected token without error, got %v", err)
	}

	parts := strings.Split(token, ".")
	tampered := parts[0] + "." + encodeSegment([]byte(`{"user_id":"999","role":"admin","active":true}`)) + "." + parts[2]

	if err := service.ValidateSignature(tampered); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected invalid signature error, got %v", err)
	}
}

func TestDecodeFailsWhenInlineValidatorRejectsToken(t *testing.T) {
	service, err := New(WithHMACSHA256("super-secret"))
	if err != nil {
		t.Fatalf("expected service without error, got %v", err)
	}

	token, err := service.Create(testClaims{
		UserID: "inactive-user",
		Role:   "reader",
		Active: false,
	})
	if err != nil {
		t.Fatalf("expected token without error, got %v", err)
	}

	var gotClaims testClaims
	err = service.Read(token, &gotClaims)
	if err != nil {
		t.Fatalf("expected read without error, got %v", err)
	}

	_, err = service.Decode(context.Background(), token, &gotClaims, func(ctx context.Context, token Token) error {
		if !gotClaims.Active {
			return errors.New("user inactive in db")
		}
		return nil
	})
	if err == nil || err.Error() != "user inactive in db" {
		t.Fatalf("expected db validation error, got %v", err)
	}
}

func TestCreateValidateAndReadJWTWithRS256(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("expected rsa key without error, got %v", err)
	}

	service, err := New(WithRSASHA256(privateKey, &privateKey.PublicKey))
	if err != nil {
		t.Fatalf("expected service without error, got %v", err)
	}

	wantClaims := testClaims{
		UserID: "rsa-user",
		Role:   "operator",
		Active: true,
	}

	token, err := service.Create(wantClaims)
	if err != nil {
		t.Fatalf("expected token without error, got %v", err)
	}

	if err := service.ValidateSignature(token); err != nil {
		t.Fatalf("expected valid rsa signature, got %v", err)
	}

	var gotClaims testClaims
	parsedToken, err := service.Decode(context.Background(), token, &gotClaims)
	if err != nil {
		t.Fatalf("expected decode without error, got %v", err)
	}

	if parsedToken.Header.Algorithm != "RS256" {
		t.Fatalf("expected algorithm RS256, got %s", parsedToken.Header.Algorithm)
	}

	if gotClaims != wantClaims {
		t.Fatalf("expected claims %+v, got %+v", wantClaims, gotClaims)
	}
}

func TestCreateValidateAndReadJWTWithPS256(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("expected rsa key without error, got %v", err)
	}

	service, err := New(WithRSAPSSSHA256(privateKey, &privateKey.PublicKey))
	if err != nil {
		t.Fatalf("expected service without error, got %v", err)
	}

	wantClaims := testClaims{UserID: "pss-user", Role: "operator", Active: true}

	token, err := service.Create(wantClaims)
	if err != nil {
		t.Fatalf("expected token without error, got %v", err)
	}

	if err := service.ValidateSignature(token); err != nil {
		t.Fatalf("expected valid pss signature, got %v", err)
	}

	var gotClaims testClaims
	parsedToken, err := service.Decode(context.Background(), token, &gotClaims)
	if err != nil {
		t.Fatalf("expected decode without error, got %v", err)
	}

	if parsedToken.Header.Algorithm != "PS256" {
		t.Fatalf("expected algorithm PS256, got %s", parsedToken.Header.Algorithm)
	}

	if gotClaims != wantClaims {
		t.Fatalf("expected claims %+v, got %+v", wantClaims, gotClaims)
	}
}

func TestCreateValidateAndReadJWTWithEdDSA(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("expected ed25519 key without error, got %v", err)
	}

	service, err := New(WithEd25519(privateKey, publicKey))
	if err != nil {
		t.Fatalf("expected service without error, got %v", err)
	}

	wantClaims := testClaims{UserID: "eddsa-user", Role: "operator", Active: true}

	token, err := service.Create(wantClaims)
	if err != nil {
		t.Fatalf("expected token without error, got %v", err)
	}

	if err := service.ValidateSignature(token); err != nil {
		t.Fatalf("expected valid eddsa signature, got %v", err)
	}

	var gotClaims testClaims
	parsedToken, err := service.Decode(context.Background(), token, &gotClaims)
	if err != nil {
		t.Fatalf("expected decode without error, got %v", err)
	}

	if parsedToken.Header.Algorithm != "EdDSA" {
		t.Fatalf("expected algorithm EdDSA, got %s", parsedToken.Header.Algorithm)
	}

	if gotClaims != wantClaims {
		t.Fatalf("expected claims %+v, got %+v", wantClaims, gotClaims)
	}
}

func TestNewAndOptionsErrors(t *testing.T) {
	if _, err := New(); !errors.Is(err, ErrNilStrategy) {
		t.Fatalf("expected ErrNilStrategy, got %v", err)
	}

	if _, err := New(WithHMACSHA256("")); !errors.Is(err, ErrMissingSecret) {
		t.Fatalf("expected ErrMissingSecret, got %v", err)
	}

	if _, err := New(WithStrategy(nil)); !errors.Is(err, ErrNilStrategy) {
		t.Fatalf("expected ErrNilStrategy from WithStrategy, got %v", err)
	}

	if _, err := New(WithValidator(nil), WithHMACSHA256("secret")); !errors.Is(err, ErrNilValidator) {
		t.Fatalf("expected ErrNilValidator, got %v", err)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("expected rsa key without error, got %v", err)
	}

	if _, err := New(WithRSASHA256(nil, &privateKey.PublicKey)); !errors.Is(err, ErrMissingPrivateKey) {
		t.Fatalf("expected ErrMissingPrivateKey, got %v", err)
	}

	if _, err := New(WithRSASHA256(privateKey, nil)); !errors.Is(err, ErrMissingPublicKey) {
		t.Fatalf("expected ErrMissingPublicKey, got %v", err)
	}

	if _, err := New(WithRSAPSSSHA256(nil, &privateKey.PublicKey)); !errors.Is(err, ErrMissingPrivateKey) {
		t.Fatalf("expected ErrMissingPrivateKey, got %v", err)
	}

	if _, err := New(WithRSAPSSSHA256(privateKey, nil)); !errors.Is(err, ErrMissingPublicKey) {
		t.Fatalf("expected ErrMissingPublicKey, got %v", err)
	}

	if _, err := New(WithEd25519(nil, nil)); !errors.Is(err, ErrMissingEdDSAPrivateKey) {
		t.Fatalf("expected ErrMissingEdDSAPrivateKey, got %v", err)
	}
}

func TestNewHMACServiceSupportsDirectSecretEnvAndOptionalValidator(t *testing.T) {
	t.Run("direct secret without validator", func(t *testing.T) {
		service, err := NewHMACService(HMACServiceInput{
			Secret: "service-secret",
		})
		if err != nil {
			t.Fatalf("expected service without error, got %v", err)
		}

		token, err := service.Create(testClaims{UserID: "42", Role: "reader", Active: true})
		if err != nil {
			t.Fatalf("expected token without error, got %v", err)
		}

		var claims testClaims
		if err := service.Read(token, &claims); err != nil {
			t.Fatalf("expected read without error, got %v", err)
		}
	})

	t.Run("secret from env with validator", func(t *testing.T) {
		viper.Set("JWT_TEST_HMAC_SECRET", "env-secret")
		defer viper.Reset()

		validatorCalled := false
		service, err := NewHMACService(HMACServiceInput{
			SecretEnv: "JWT_TEST_HMAC_SECRET",
			Validator: func(ctx context.Context, token Token) error {
				validatorCalled = true
				return nil
			},
		})
		if err != nil {
			t.Fatalf("expected service without error, got %v", err)
		}

		token, err := service.Create(testClaims{UserID: "24", Role: "admin", Active: true})
		if err != nil {
			t.Fatalf("expected token without error, got %v", err)
		}

		var claims testClaims
		if _, err := service.Decode(context.Background(), token, &claims); err != nil {
			t.Fatalf("expected decode without error, got %v", err)
		}

		if !validatorCalled {
			t.Fatal("expected validator to run")
		}
	})
}

func TestNewRSAServiceSupportsDirectKeysEnvAndOptionalValidator(t *testing.T) {
	t.Run("direct keys without validator", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("expected rsa key without error, got %v", err)
		}

		service, err := NewRSAService(RSAServiceInput{
			PrivateKey: privateKey,
			PublicKey:  &privateKey.PublicKey,
		})
		if err != nil {
			t.Fatalf("expected service without error, got %v", err)
		}

		token, err := service.Create(testClaims{UserID: "99", Role: "operator", Active: true})
		if err != nil {
			t.Fatalf("expected token without error, got %v", err)
		}

		if err := service.ValidateSignature(token); err != nil {
			t.Fatalf("expected valid signature, got %v", err)
		}
	})

	t.Run("keys from env with validator", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
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

		viper.Set("JWT_TEST_RSA_PRIVATE", base64.StdEncoding.EncodeToString(privateDER))
		viper.Set("JWT_TEST_RSA_PUBLIC", base64.StdEncoding.EncodeToString(publicDER))
		defer viper.Reset()

		validatorCalled := false
		service, err := NewRSAService(RSAServiceInput{
			PrivateKeyEnv: "JWT_TEST_RSA_PRIVATE",
			PublicKeyEnv:  "JWT_TEST_RSA_PUBLIC",
			Validator: func(ctx context.Context, token Token) error {
				validatorCalled = true
				return nil
			},
		})
		if err != nil {
			t.Fatalf("expected service without error, got %v", err)
		}

		token, err := service.Create(testClaims{UserID: "77", Role: "admin", Active: true})
		if err != nil {
			t.Fatalf("expected token without error, got %v", err)
		}

		var claims testClaims
		if _, err := service.Decode(context.Background(), token, &claims); err != nil {
			t.Fatalf("expected decode without error, got %v", err)
		}

		if !validatorCalled {
			t.Fatal("expected validator to run")
		}
	})
}

func TestNewServicesFromEnvErrors(t *testing.T) {
	t.Run("missing hmac secret", func(t *testing.T) {
		defer viper.Reset()
		if _, err := NewHMACService(HMACServiceInput{SecretEnv: "JWT_TEST_MISSING_SECRET"}); !errors.Is(err, ErrMissingSecret) {
			t.Fatalf("expected ErrMissingSecret, got %v", err)
		}
	})

	t.Run("invalid rsa private key env", func(t *testing.T) {
		viper.Set("JWT_TEST_BAD_RSA_PRIVATE", "%%%")
		viper.Set("JWT_TEST_BAD_RSA_PUBLIC", "")
		defer viper.Reset()

		_, err := NewRSAService(RSAServiceInput{
			PrivateKeyEnv: "JWT_TEST_BAD_RSA_PRIVATE",
			PublicKeyEnv:  "JWT_TEST_BAD_RSA_PUBLIC",
		})
		if err == nil || !strings.Contains(err.Error(), "JWT_TEST_BAD_RSA_PRIVATE") {
			t.Fatalf("expected private key env parse error, got %v", err)
		}
	})
}

func TestNewConfiguredServiceUsesViperAlgorithm(t *testing.T) {
	t.Run("hs256", func(t *testing.T) {
		viper.Set(DefaultAlgorithmKey, "HS256")
		viper.Set(DefaultHMACSecretKey, "configured-secret")
		defer viper.Reset()

		service, err := NewConfiguredService(ConfigServiceInput{
			Validator: func(ctx context.Context, token Token) error { return nil },
		})
		if err != nil {
			t.Fatalf("expected service without error, got %v", err)
		}

		token, err := service.Create(testClaims{UserID: "1", Role: "admin", Active: true})
		if err != nil {
			t.Fatalf("expected token without error, got %v", err)
		}

		if err := service.ValidateSignature(token); err != nil {
			t.Fatalf("expected valid signature, got %v", err)
		}
	})

	t.Run("rs256", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
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

		viper.Set(DefaultAlgorithmKey, "RS256")
		viper.Set(DefaultRSAPrivateKeyKey, base64.StdEncoding.EncodeToString(privateDER))
		viper.Set(DefaultRSAPublicKeyKey, base64.StdEncoding.EncodeToString(publicDER))
		defer viper.Reset()

		service, err := NewConfiguredService(ConfigServiceInput{})
		if err != nil {
			t.Fatalf("expected service without error, got %v", err)
		}

		token, err := service.Create(testClaims{UserID: "2", Role: "admin", Active: true})
		if err != nil {
			t.Fatalf("expected token without error, got %v", err)
		}

		if err := service.ValidateSignature(token); err != nil {
			t.Fatalf("expected valid signature, got %v", err)
		}
	})

	t.Run("ps256", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
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

		viper.Set(DefaultAlgorithmKey, "PS256")
		viper.Set(DefaultRSAPrivateKeyKey, base64.StdEncoding.EncodeToString(privateDER))
		viper.Set(DefaultRSAPublicKeyKey, base64.StdEncoding.EncodeToString(publicDER))
		defer viper.Reset()

		service, err := NewConfiguredService(ConfigServiceInput{})
		if err != nil {
			t.Fatalf("expected service without error, got %v", err)
		}

		token, err := service.Create(testClaims{UserID: "3", Role: "admin", Active: true})
		if err != nil {
			t.Fatalf("expected token without error, got %v", err)
		}

		if err := service.ValidateSignature(token); err != nil {
			t.Fatalf("expected valid signature, got %v", err)
		}
	})

	t.Run("eddsa", func(t *testing.T) {
		publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("expected ed25519 key without error, got %v", err)
		}

		privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			t.Fatalf("expected private key marshal without error, got %v", err)
		}

		publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
		if err != nil {
			t.Fatalf("expected public key marshal without error, got %v", err)
		}

		viper.Set(DefaultAlgorithmKey, "EdDSA")
		viper.Set(DefaultEdDSAPrivateKeyKey, base64.StdEncoding.EncodeToString(privateDER))
		viper.Set(DefaultEdDSAPublicKeyKey, base64.StdEncoding.EncodeToString(publicDER))
		defer viper.Reset()

		service, err := NewConfiguredService(ConfigServiceInput{})
		if err != nil {
			t.Fatalf("expected service without error, got %v", err)
		}

		token, err := service.Create(testClaims{UserID: "4", Role: "admin", Active: true})
		if err != nil {
			t.Fatalf("expected token without error, got %v", err)
		}

		if err := service.ValidateSignature(token); err != nil {
			t.Fatalf("expected valid signature, got %v", err)
		}
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		viper.Set(DefaultAlgorithmKey, "ES256")
		defer viper.Reset()

		_, err := NewConfiguredService(ConfigServiceInput{})
		if !errors.Is(err, ErrUnsupportedAlg) {
			t.Fatalf("expected ErrUnsupportedAlg, got %v", err)
		}
	})
}

func TestCreateAndDecodeErrors(t *testing.T) {
	service, err := New(WithHMACSHA256("secret"))
	if err != nil {
		t.Fatalf("expected service without error, got %v", err)
	}

	if _, err := (*Service)(nil).Create(testClaims{}); !errors.Is(err, ErrNilStrategy) {
		t.Fatalf("expected ErrNilStrategy for nil service create, got %v", err)
	}

	badClaims := map[string]any{"bad": make(chan int)}
	if _, err := service.Create(badClaims); err == nil || !strings.Contains(err.Error(), "jwt: encode claims") {
		t.Fatalf("expected claims encode error, got %v", err)
	}

	if _, err := service.Decode(context.Background(), "token", nil); !errors.Is(err, ErrNilDestination) {
		t.Fatalf("expected ErrNilDestination, got %v", err)
	}

	token, err := service.Create(testClaims{UserID: "42", Role: "admin", Active: true})
	if err != nil {
		t.Fatalf("expected token without error, got %v", err)
	}

	var claims testClaims
	if _, err := service.Decode(context.Background(), token, &claims, nil); !errors.Is(err, ErrMissingValidation) {
		t.Fatalf("expected ErrMissingValidation, got %v", err)
	}

	if _, err := service.Decode(context.Background(), token, claims); err == nil || !strings.Contains(err.Error(), "jwt: decode claims") {
		t.Fatalf("expected decode claims error, got %v", err)
	}
}

func TestValidateSignatureAndParseErrors(t *testing.T) {
	service, err := New(WithHMACSHA256("secret"))
	if err != nil {
		t.Fatalf("expected service without error, got %v", err)
	}

	tests := []struct {
		name  string
		token string
		check func(error) bool
	}{
		{
			name:  "invalid parts",
			token: "only-two.parts",
			check: func(err error) bool { return errors.Is(err, ErrInvalidToken) },
		},
		{
			name:  "invalid header base64",
			token: "%%%." + encodeSegment([]byte(`{"user_id":"42"}`)) + "." + encodeSegment([]byte("sig")),
			check: func(err error) bool { return err != nil && strings.Contains(err.Error(), "jwt: decode header") },
		},
		{
			name:  "invalid header json",
			token: encodeSegment([]byte("{")) + "." + encodeSegment([]byte(`{"user_id":"42"}`)) + "." + encodeSegment([]byte("sig")),
			check: func(err error) bool { return err != nil && strings.Contains(err.Error(), "jwt: parse header") },
		},
		{
			name:  "unexpected alg",
			token: buildRawToken(t, Header{Type: "JWT", Algorithm: "RS256"}, map[string]any{"user_id": "42"}, []byte("sig")),
			check: func(err error) bool { return err != nil && errors.Is(err, ErrUnexpectedAlg) },
		},
		{
			name:  "invalid claims base64",
			token: buildRawTokenFromParts(t, Header{Type: "JWT", Algorithm: "HS256"}, "%%%", encodeSegment([]byte("sig"))),
			check: func(err error) bool { return err != nil && strings.Contains(err.Error(), "jwt: decode claims") },
		},
		{
			name:  "invalid signature base64",
			token: buildRawTokenFromParts(t, Header{Type: "JWT", Algorithm: "HS256"}, encodeSegment([]byte(`{"user_id":"42"}`)), "%%%"),
			check: func(err error) bool { return err != nil && strings.Contains(err.Error(), "jwt: decode signature") },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := service.ValidateSignature(test.token)
			if !test.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	if err := (*Service)(nil).ValidateSignature("any"); !errors.Is(err, ErrNilStrategy) {
		t.Fatalf("expected ErrNilStrategy for nil service validate, got %v", err)
	}
}

func TestStrategySpecificErrorsAndHelpers(t *testing.T) {
	hmacStrategy := NewHMACSHA256("").(*hmacSHA256Strategy)
	if _, err := hmacStrategy.Sign([]byte("input")); !errors.Is(err, ErrMissingSecret) {
		t.Fatalf("expected ErrMissingSecret, got %v", err)
	}

	rsaStrategy := &rsaSHA256Strategy{}
	if _, err := rsaStrategy.Sign([]byte("input")); !errors.Is(err, ErrMissingPrivateKey) {
		t.Fatalf("expected ErrMissingPrivateKey, got %v", err)
	}

	if err := rsaStrategy.Verify([]byte("input"), []byte("sig")); !errors.Is(err, ErrMissingPublicKey) {
		t.Fatalf("expected ErrMissingPublicKey, got %v", err)
	}

	rsaPSSStrategy := &rsaPSSSHA256Strategy{}
	if _, err := rsaPSSStrategy.Sign([]byte("input")); !errors.Is(err, ErrMissingPrivateKey) {
		t.Fatalf("expected ErrMissingPrivateKey, got %v", err)
	}

	if err := rsaPSSStrategy.Verify([]byte("input"), []byte("sig")); !errors.Is(err, ErrMissingPublicKey) {
		t.Fatalf("expected ErrMissingPublicKey, got %v", err)
	}

	eddsaStrategy := &ed25519Strategy{}
	if _, err := eddsaStrategy.Sign([]byte("input")); !errors.Is(err, ErrMissingEdDSAPrivateKey) {
		t.Fatalf("expected ErrMissingEdDSAPrivateKey, got %v", err)
	}

	if err := eddsaStrategy.Verify([]byte("input"), []byte("sig")); !errors.Is(err, ErrMissingEdDSAPublicKey) {
		t.Fatalf("expected ErrMissingEdDSAPublicKey, got %v", err)
	}

	encoded := encodeSegment([]byte("hello"))
	decoded, err := decodeSegment(encoded)
	if err != nil {
		t.Fatalf("expected decodeSegment without error, got %v", err)
	}
	if string(decoded) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", string(decoded))
	}

	if _, err := decodeSegment("%%%"); err == nil {
		t.Fatal("expected decodeSegment error")
	}
}

func buildRawToken(t *testing.T, header Header, claims any, signature []byte) string {
	t.Helper()

	headerBytes, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("expected header json without error, got %v", err)
	}

	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("expected claims json without error, got %v", err)
	}

	return fmt.Sprintf("%s.%s.%s", encodeSegment(headerBytes), encodeSegment(claimsBytes), encodeSegment(signature))
}

func buildRawTokenFromParts(t *testing.T, header Header, claimsPart string, signaturePart string) string {
	t.Helper()

	headerBytes, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("expected header json without error, got %v", err)
	}

	return fmt.Sprintf("%s.%s.%s", encodeSegment(headerBytes), claimsPart, signaturePart)
}

func unmarshalClaims(raw []byte, destination any) error {
	return json.Unmarshal(raw, destination)
}
