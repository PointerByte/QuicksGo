// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package cookies

import (
	"context"
	"errors"
	"net/http"
	"strings"

	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
	"github.com/spf13/viper"
)

var (
	ErrNilJWTService    = errors.New("cookies: jwt service is required")
	ErrMissingCookieKey = errors.New("cookies: cookie name is required")
	ErrNilRequest       = errors.New("cookies: request is required")
	ErrMissingCookie    = errors.New("cookies: auth cookie is required")
)

const (
	DefaultCookieName    = "access_token"
	DefaultCookieNameKey = "jwt.cookie.name"
)

// Option configures a cookie auth Service.
type Option func(*Service) error

// Service validates JWT auth tokens extracted from HTTP cookies.
type Service struct {
	jwtService *jwtservice.Service
	cookieName string
}

// ConfigServiceInput configures NewConfiguredService.
// When CookieName is empty, it is loaded from viper using CookieNameKey, and
// then falls back to DefaultCookieName.
type ConfigServiceInput struct {
	CookieName    string
	CookieNameKey string
	JWT           jwtservice.ConfigServiceInput
}

// New builds a cookie auth service from options.
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
	if service.jwtService == nil {
		return nil, ErrNilJWTService
	}
	if strings.TrimSpace(service.cookieName) == "" {
		return nil, ErrMissingCookieKey
	}
	return service, nil
}

// NewConfiguredService builds a cookie auth service from viper-backed JWT and
// cookie configuration.
func NewConfiguredService(input ConfigServiceInput) (*Service, error) {
	jwtService, err := jwtservice.NewConfiguredService(input.JWT)
	if err != nil {
		return nil, err
	}

	cookieName := strings.TrimSpace(input.CookieName)
	if cookieName == "" {
		cookieName = strings.TrimSpace(viper.GetString(stringOrDefault(input.CookieNameKey, DefaultCookieNameKey)))
	}
	if cookieName == "" {
		cookieName = DefaultCookieName
	}

	return New(
		WithJWTService(jwtService),
		WithCookieName(cookieName),
	)
}

// WithJWTService injects the JWT service used to validate cookie values.
func WithJWTService(service *jwtservice.Service) Option {
	return func(config *Service) error {
		if service == nil {
			return ErrNilJWTService
		}
		config.jwtService = service
		return nil
	}
}

// WithCookieName configures the cookie name used to extract the auth token.
func WithCookieName(cookieName string) Option {
	return func(config *Service) error {
		cookieName = strings.TrimSpace(cookieName)
		if cookieName == "" {
			return ErrMissingCookieKey
		}
		config.cookieName = cookieName
		return nil
	}
}

// CookieName returns the configured cookie name.
func (service *Service) CookieName() string {
	if service == nil {
		return ""
	}
	return service.cookieName
}

// TokenFromRequest extracts the raw JWT token from the configured request
// cookie.
func (service *Service) TokenFromRequest(request *http.Request) (string, error) {
	if service == nil || service.jwtService == nil {
		return "", ErrNilJWTService
	}
	if request == nil {
		return "", ErrNilRequest
	}

	cookie, err := request.Cookie(service.cookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", ErrMissingCookie
		}
		return "", err
	}

	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return "", ErrMissingCookie
	}
	return token, nil
}

// ValidateRequest validates the JWT signature from the configured request
// cookie without decoding claims.
func (service *Service) ValidateRequest(request *http.Request) error {
	token, err := service.TokenFromRequest(request)
	if err != nil {
		return err
	}
	return service.jwtService.ValidateSignature(token)
}

// Read validates the token stored in the configured cookie and decodes its
// claims into destination.
func (service *Service) Read(request *http.Request, destination any) error {
	_, err := service.Decode(context.Background(), request, destination)
	return err
}

// Decode extracts the token from the configured cookie, validates it, decodes
// its claims into destination, and runs optional validators.
func (service *Service) Decode(ctx context.Context, request *http.Request, destination any, validators ...jwtservice.Validator) (jwtservice.Token, error) {
	token, err := service.TokenFromRequest(request)
	if err != nil {
		return jwtservice.Token{}, err
	}
	return service.jwtService.Decode(ctx, token, destination, validators...)
}

func stringOrDefault(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
