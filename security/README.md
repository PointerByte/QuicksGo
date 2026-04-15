# QuicksGo Security

`security` provides JWT services and Gin middlewares for token-based authentication. It is designed to work with `viper` configuration and uses `github.com/PointerByte/QuicksGo/encrypt` as its standalone cryptographic dependency.

## Installation

```bash
go get github.com/PointerByte/QuicksGo/security
```

If you also need direct access to cryptographic primitives, add:

```bash
go get github.com/PointerByte/QuicksGo/encrypt
```

## What this module includes

- JWT creation, validation, and claim decoding
- Cookie-based JWT auth
- Gin middlewares for JWT and cookie auth
- Security headers middleware
- Example app showing HMAC and RSA flows

## Package layout

- `auth/jwt`: JWT services and signing strategy configuration
- `auth/cookies`: JWT-from-cookie service
- `middlewares`: Gin middleware helpers

## Relationship with `encrypt`

`encrypt` is now a separate module. `security` depends on it internally for cryptographic operations, but the public import path is no longer `github.com/PointerByte/QuicksGo/security/encrypt`.

Use these module paths instead:

- `github.com/PointerByte/QuicksGo/security`
- `github.com/PointerByte/QuicksGo/encrypt`

## Configuration with Viper

JWT and cookie-auth services resolve configuration through `viper`.

This module does not automatically load `application.yaml`, `application.yml`, or `application.json`. Your application must load configuration into `viper` before creating a configured service or using `RequireJWT` / `RequireJWTCookie`.

Example loading YAML:

```go
import "github.com/spf13/viper"

func loadConfig() error {
	viper.SetConfigName("application")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	return viper.ReadInConfig()
}
```

Example loading JSON:

```go
import "github.com/spf13/viper"

func loadConfig() error {
	viper.SetConfigName("application")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	return viper.ReadInConfig()
}
```

Main keys:

- `jwt.enable`
- `jwt.algorithm`
- `jwt.cookie.name`
- `jwt.hmac.secret`
- `jwt.rsa.private_key`
- `jwt.rsa.public_key`
- `jwt.eddsa.private_key`
- `jwt.eddsa.public_key`

Example config files in this module:

- [application.yaml](./application.yaml)
- [application.json](./application.json)

## JWT usage

### Build a configured service

```go
import (
	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
	"github.com/spf13/viper"
)

viper.Set("jwt.algorithm", "HS256")
viper.Set("jwt.hmac.secret", "my-secret")

service, err := jwtservice.NewConfiguredService(jwtservice.ConfigServiceInput{})
if err != nil {
	panic(err)
}
```

### Create a token

```go
token, err := service.Create(map[string]any{
	"user_id": "42",
	"role":    "admin",
})
if err != nil {
	panic(err)
}
```

### Read typed claims

```go
var claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

if err := service.Read(token, &claims); err != nil {
	panic(err)
}
```

### Supported algorithms

- `HS256`
- `RS256`
- `PS256`
- `EdDSA`

## JWT middleware for Gin

`RequireJWT` builds the JWT service internally using `viper`.

```go
import (
	"context"

	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
	"github.com/PointerByte/QuicksGo/security/middlewares"
)

router.Use(middlewares.RequireJWT(
	middlewares.WithJWTClaimsFactory(func() any { return &MyClaims{} }),
	middlewares.WithJWTValidator(func(ctx context.Context, token jwtservice.Token) error {
		return nil
	}),
))
```

### Read claims from Gin context

```go
type MyClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

claimsValue, _ := c.Get(middlewares.JWTClaimsContextKey.String())
claims := claimsValue.(*MyClaims)
```

## Cookie auth

The `auth/cookies` package reuses the JWT service and reads the token from an HTTP cookie.

### Build a configured cookie service

```go
import (
	cookiesauth "github.com/PointerByte/QuicksGo/security/auth/cookies"
	"github.com/spf13/viper"
)

viper.Set("jwt.algorithm", "HS256")
viper.Set("jwt.hmac.secret", "my-secret")
viper.Set("jwt.cookie.name", "session_token")

service, err := cookiesauth.NewConfiguredService(cookiesauth.ConfigServiceInput{})
if err != nil {
	panic(err)
}
```

### Read claims from a request cookie

```go
var claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

if err := service.Read(r, &claims); err != nil {
	panic(err)
}
```

## Cookie middleware for Gin

```go
router.Use(middlewares.RequireJWTCookie(
	middlewares.WithJWTCookieClaimsFactory(func() any { return &MyClaims{} }),
))
```

By default it reads the cookie configured in `jwt.cookie.name`, or `access_token` when that key is missing.

## Direct `encrypt` usage alongside `security`

If your application uses `security` and also needs explicit crypto operations, import `encrypt` directly:

```go
import (
	"context"

	"github.com/PointerByte/QuicksGo/encrypt"
	"github.com/PointerByte/QuicksGo/encrypt/local"
)

ctx := context.Background()
repository := encrypt.NewRepository(local.NewRepository())

_, _ = ctx, repository
```

See the `encrypt` module README for backend-specific details.

## Runnable example

This module includes a runnable example in [main.go](./main.go).

Run it from the `security` directory:

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

## Tests

From the `security` module directory:

```bash
go test ./...
```

With coverage:

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
```

## Useful commands

Update dependencies:

```bash
go get -u ./...
```

Clear build, test, and module cache:

```bash
go clean -cache -testcache -modcache
```
