# GoForge Security

`security` provides JWT services and Gin middleware for token-based
authentication. It uses `viper` for configured services and depends on
`github.com/PointerByte/GoForge/encrypt` for cryptographic helpers.

## Installation

```bash
go get github.com/PointerByte/GoForge/security
```

If your application also needs direct cryptographic operations, add:

```bash
go get github.com/PointerByte/GoForge/encrypt
```

Update the dependencies used by the current module:

```bash
go get -u ./...
```

## Packages

- `auth/jwt`: JWT creation, signature validation, claim decoding, and signing strategies
- `auth/cookies`: JWT validation from HTTP cookies
- `middlewares`: Gin middleware for bearer tokens, cookie tokens, and security headers

## Capabilities

- create JWTs from arbitrary claims
- validate compact JWT signatures and algorithms
- decode claims into `map[string]any` or typed structs
- add service-level and per-call validators
- use request contexts and service-level timeouts
- protect Gin routes through bearer or cookie JWT middleware
- apply common HTTP security headers
- plug in custom signing strategies

## Configuration

This module does not load `application.yaml`, `application.yml`, or
`application.json` automatically. Load configuration into `viper` before using
`NewConfiguredService`, `RequireJWT`, or `RequireJWTCookie`.

```yaml
jwt:
  enable: true
  cookie:
    name: access_token
  eddsa:
    private_key: ./certs/jwt/ed25519-key.pem
    public_key: ./certs/jwt/ed25519-public.pem
```

Main keys:

- `jwt.enable`: when explicitly set to `false`, Gin JWT middleware lets requests pass through
- `jwt.algorithm`: `HS256`, `RS256`, `PS256`, or `EdDSA`; optional when only one strategy is configured
- `jwt.cookie.name`: cookie name used by cookie-based auth; defaults to `access_token`
- `jwt.hmac.secret`: shared secret for `HS256`
- `jwt.rsa.private_key`: RSA private key value or PEM file path
- `jwt.rsa.public_key`: RSA public key value or PEM file path
- `jwt.eddsa.private_key`: Ed25519 private key value or PEM file path
- `jwt.eddsa.public_key`: Ed25519 public key value or PEM file path

Configure one strategy per service. If more than one strategy exists under
`jwt`, set `jwt.algorithm` to resolve the ambiguity.

The `application.yaml` and `application.json` files in this module are complete
examples: they include `hmac`, `rsa`, and `eddsa` to document the available
options. Because multiple strategies are configured, they also include
`jwt.algorithm`.

Configured service inputs can receive direct values through `HMACSecret`,
`RSAPrivateKey`, `RSAPublicKey`, `EdDSAPrivateKey`, and `EdDSAPublicKey`. The
`*Key` fields keep their legacy behavior: they point to viper keys, but RSA and
EdDSA key fields also accept direct PEM paths or Base64-encoded DER values when
no matching viper key exists.

Example files:

- [application.yaml](./application.yaml)
- [application.json](./application.json)

## JWT Service

### Configured From Viper

With one configured strategy, `NewConfiguredService` can infer the algorithm:

```go
viper.Set("jwt.eddsa.private_key", "./certs/jwt/ed25519-key.pem")
viper.Set("jwt.eddsa.public_key", "./certs/jwt/ed25519-public.pem")

service, err := jwtservice.NewConfiguredService(jwtservice.ConfigServiceInput{})
if err != nil {
	panic(err)
}
```

If you configure multiple strategies, set `jwt.algorithm` or pass it in
`ConfigServiceInput`:

```go
package main

import (
	jwtservice "github.com/PointerByte/GoForge/security/auth/jwt"
	"github.com/spf13/viper"
)

func main() {
	viper.Set("jwt.algorithm", "HS256")
	viper.Set("jwt.hmac.secret", "my-secret")

	hmacSecretKey := jwtservice.DefaultHMACSecretKey

	service, err := jwtservice.NewConfiguredService(jwtservice.ConfigServiceInput{
		Algorithm:     "HS256",
		HMACSecretKey: &hmacSecretKey,
	})
	if err != nil {
		panic(err)
	}

	token, err := service.Create(map[string]any{"user_id": "42"})
	if err != nil {
		panic(err)
	}

	var claims map[string]any
	if err := service.Read(token, &claims); err != nil {
		panic(err)
	}
}
```

### Direct Secret

```go
service, err := jwtservice.New(
	jwtservice.WithHMACSHA256("my-secret"),
)
if err != nil {
	panic(err)
}
```

### Context And Validators

```go
service, err := jwtservice.New(
	jwtservice.WithHMACSHA256("my-secret"),
	jwtservice.WithContextTimeout(2*time.Second),
	jwtservice.WithValidator(func(ctx context.Context, token jwtservice.Token) error {
		return nil
	}),
)
if err != nil {
	panic(err)
}

ctx := context.Background()

token, err := service.CreateWithContext(ctx, map[string]any{"user_id": "42"})
if err != nil {
	panic(err)
}

var claims struct {
	UserID string `json:"user_id"`
}

parsedToken, err := service.Decode(ctx, token, &claims)
if err != nil {
	panic(err)
}

_ = parsedToken
```

Use `ValidateSignatureWithContext(ctx, token)` when you only need to verify the
JWT structure, algorithm, and signature without decoding claims.

## Supported Algorithms

- `HS256`: HMAC-SHA256
- `RS256`: RSA SHA-256
- `PS256`: RSA-PSS SHA-256
- `EdDSA`: Ed25519

RSA and Ed25519 configured keys may be PEM file paths or supported encoded key
values.

## Bearer Middleware For Gin

`RequireJWT` reads a bearer token from the `Authorization` header, validates it,
and stores the parsed token and claims in Gin context.

```go
package main

import (
	"context"

	jwtservice "github.com/PointerByte/GoForge/security/auth/jwt"
	"github.com/PointerByte/GoForge/security/middlewares"
	"github.com/gin-gonic/gin"
)

type MyClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func main() {
	router := gin.New()
	hmacSecretKey := jwtservice.DefaultHMACSecretKey

	router.Use(middlewares.RequireJWT(
		middlewares.WithJWTServiceConfig(jwtservice.ConfigServiceInput{
			Algorithm:     "HS256",
			HMACSecretKey: &hmacSecretKey,
		}),
		middlewares.WithJWTClaimsFactory(func() any { return &MyClaims{} }),
		middlewares.WithJWTValidator(func(ctx context.Context, token jwtservice.Token) error {
			return nil
		}),
	))
}
```

Read values from Gin context:

```go
claimsValue, ok := c.Get(middlewares.JWTClaimsContextKey.String())
if !ok {
	return
}

claims := claimsValue.(*MyClaims)
_ = claims
```

The parsed token is stored under `middlewares.JWTTokenContextKey.String()`.
Without a claims factory, decoded claims are stored as `map[string]any`.
Customize context keys with `WithJWTContextKeys`.

## Bearer Interceptors For gRPC

`RequireJWTUnaryServerInterceptor` and `RequireJWTStreamServerInterceptor` read
a bearer token from gRPC `authorization` metadata, validate the JWT, and store
the parsed token and claims in `context.Context`.

```go
server := grpc.NewServer(
	grpc.ChainUnaryInterceptor(
		middlewares.RequireJWTUnaryServerInterceptor(
			middlewares.WithGRPCJWTClaimsFactory(func() any { return &MyClaims{} }),
		),
	),
	grpc.ChainStreamInterceptor(
		middlewares.RequireJWTStreamServerInterceptor(
			middlewares.WithGRPCJWTClaimsFactory(func() any { return &MyClaims{} }),
		),
	),
)
```

Read values from gRPC context:

```go
claimsValue, ok := middlewares.JWTClaimsFromContext(ctx)
if !ok {
	return nil, status.Error(codes.Unauthenticated, "claims not available")
}

claims := claimsValue.(*MyClaims)
_ = claims
```

Clients must send metadata:

```text
authorization: Bearer <token>
```

## Cookie Auth

The `auth/cookies` package validates JWTs stored in an HTTP cookie.

```go
import (
	cookiesauth "github.com/PointerByte/GoForge/security/auth/cookies"
	jwtservice "github.com/PointerByte/GoForge/security/auth/jwt"
)

hmacSecretKey := jwtservice.DefaultHMACSecretKey

service, err := cookiesauth.NewConfiguredService(cookiesauth.ConfigServiceInput{
	CookieNameKey: cookiesauth.DefaultCookieNameKey,
	JWT: jwtservice.ConfigServiceInput{
		Algorithm:     "HS256",
		HMACSecretKey: &hmacSecretKey,
	},
})
if err != nil {
	panic(err)
}

var claims map[string]any
if err := service.Read(request, &claims); err != nil {
	panic(err)
}
```

Gin cookie middleware:

```go
router.Use(middlewares.RequireJWTCookie(
	middlewares.WithJWTCookieServiceConfig(cookiesauth.ConfigServiceInput{
		CookieNameKey: cookiesauth.DefaultCookieNameKey,
		JWT: jwtservice.ConfigServiceInput{
			Algorithm:     "HS256",
			HMACSecretKey: &hmacSecretKey,
		},
	}),
	middlewares.WithJWTCookieClaimsFactory(func() any { return &MyClaims{} }),
))
```

It reads `jwt.cookie.name` from viper and falls back to `access_token`.

## Custom Strategies

Use `WithCustomStrategy` directly or override middleware service creation with
`WithJWTServiceFactory`.

```go
service, err := jwtservice.New(
	jwtservice.WithCustomStrategy("CUSTOM", signFunc, verifyFunc),
)
if err != nil {
	panic(err)
}

_ = service
```

## Security Headers

`middlewares.SecurityHeaders()` adds common response headers such as
`X-Frame-Options`, `Content-Security-Policy`, `Strict-Transport-Security`,
`Referrer-Policy`, `X-Content-Type-Options`, and `Permissions-Policy`.

```go
router.Use(middlewares.SecurityHeaders())
```

## Relationship With `encrypt`

`encrypt` is a separate module. `security` uses it internally, but the public
crypto import path is:

```go
github.com/PointerByte/GoForge/encrypt
```

Use `encrypt` directly when your application needs AES, hashing, RSA/ECC, KMS,
or signature helpers outside JWT auth.

## Runnable Example

This module includes an example app in [main.go](./main.go).

```bash
go run .
```

Example routes:

- `GET /health`
- `POST /hmac/login`
- `GET /hmac/api/me`
- `GET /hmac/api/admin`
- `POST /rsa/login`
- `GET /rsa/api/me`
- `GET /rsa/api/admin`
- `POST /custom/login`
- `GET /custom/api/me`
- `GET /custom/api/admin`

## Tests

From the `security` module directory:

```bash
go test ./...
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
```
