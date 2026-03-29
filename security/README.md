# QuicksGo Security

Security library for Go with utilities for:

- JWT with `viper`
- Gin middlewares
- Symmetric encryption
- RSA
- Digital signatures

## Installation

```bash
go get github.com/PointerByte/QuicksGo/security
```

## Packages

- `auth/jwt`: JWT creation, validation, and claim decoding
- `auth/cookies`: JWT validation and claim decoding from HTTP cookies
- `middlewares`: Gin middlewares (`RequireJWT`, `RequireJWTCookie`, security headers)
- `encrypt`: context-aware repository API for AES-GCM, HMAC, hashes, RSA, and digital signatures
- `encrypt/local`: in-process implementation with exportable key material
- `encrypt/aws-kms`: AWS KMS-backed implementation where the provider supports the operation
- `encrypt/azure-key-vault`: Azure Key Vault-oriented implementation with local fallbacks for local-only primitives
- `encrypt/gcp-kms`: Google Cloud KMS-oriented implementation with local fallbacks for local-only primitives

## Viper Configuration

The JWT and cookie-auth packages resolve configuration through `viper`.

`security` does not load `application.yaml`, `application.yml`, or `application.json` by itself. The host application must load one of those files into `viper` before creating the JWT service or using `RequireJWT` / `RequireJWTCookie`.

In this repository, for example, the server package loads configuration from the application root with this priority:

- `application.yml`
- `application.json`

That means `viper` values such as `jwt.enable` or `jwt.algorithm` come from the file your application loaded first, and can then be overridden by environment variables if your bootstrap does that.

Example loading `application.yaml` or `application.yml`:

```go
import "github.com/spf13/viper"

func loadConfig() error {
	viper.SetConfigName("application")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	return viper.ReadInConfig()
}
```

Example loading `application.json`:

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

Included examples:

- [config.example.yaml](/e:/Proyects/Practices/QuicksGoV2t/security/config.example.yaml)
- [config.example.json](/e:/Proyects/Practices/QuicksGoV2t/security/config.example.json)

## JWT Usage

### Create a service from configuration

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
```

### Validate and decode claims

```go
var claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

err := service.Read(token, &claims)
```

### Supported JWT algorithms

- `HS256`
- `RS256`
- `PS256`
- `EdDSA`

## JWT Middleware for Gin

`RequireJWT` builds the JWT service internally using `viper`.

```go
router.Use(middlewares.RequireJWT(
	middlewares.WithJWTClaimsFactory(func() any { return &MyClaims{} }),
	middlewares.WithJWTValidator(func(ctx context.Context, token jwtservice.Token) error {
		return nil
	}),
))
```

### Typed claims

```go
type MyClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}
```

Then in a handler:

```go
claimsValue, _ := c.Get(middlewares.JWTClaimsContextKey.String())
claims := claimsValue.(*MyClaims)
```

## Cookie Auth

The `auth/cookies` package reuses the JWT service and reads the token from an HTTP cookie.

### Create a cookie auth service from configuration

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

### Validate claims from a request cookie

```go
var claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

err := service.Read(r, &claims)
```

## Cookie Middleware for Gin

`RequireJWTCookie` validates the JWT from a request cookie and stores the parsed token and claims in the Gin context.

```go
router.Use(middlewares.RequireJWTCookie(
	middlewares.WithJWTCookieClaimsFactory(func() any { return &MyClaims{} }),
))
```

By default it reads the cookie configured in `jwt.cookie.name`, or `access_token` when that key is not set.

## Encrypt Usage

The encryption module now exposes repository interfaces through `encrypt`.

### Create a repository

```go
import (
	"context"

	"github.com/PointerByte/QuicksGo/security/encrypt"
)

ctx := context.Background()
repository := encrypt.NewRepository()
```

### AES-GCM

```go
keyData, err := repository.GeneratesSymetrycKey(ctx, 32)
if err != nil {
	panic(err)
}

additional := "aad"
encrypted, err := repository.EncryptAES(ctx, keyData.Key, "hello", &additional)
if err != nil {
	panic(err)
}

plainText, err := repository.DecryptAES(ctx, keyData.Key, encrypted, additional)
```

### HMAC

```go
hash := repository.GenerateHMAC(ctx, "message", "secret")
ok := repository.ValidateHMAC(ctx, "message", "secret", hash)
```

### RSA-OAEP with SHA-256

```go
keyData, err := repository.GeneratesRSAKey(ctx, 2048)
if err != nil {
	panic(err)
}

cipherText, err := repository.RSA_OAEP_Encode(ctx, keyData.PublicKey, "hello")
if err != nil {
	panic(err)
}

plainText, err := repository.RSA_OAEP_Decode(ctx, keyData.PrivateKey, cipherText)
```

### Ed25519

```go
keyData, err := repository.GeneratesEd255Key(ctx, 2048)
if err != nil {
	panic(err)
}

signature, err := repository.SignEd25519(ctx, keyData.PrivateKey, "hello")
if err != nil {
	panic(err)
}

err = repository.VerifyEd25519(ctx, keyData.PublicKey, "hello", signature)
```

## Runnable Example

The project includes a Gin example in [main.go](/e:/Proyects/Practices/QuicksGoV2t/security/main.go).

Run it with:

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

```bash
go test ./...
```

## Useful Commands

### Update dependencies

Updates module dependencies to newer allowed versions.

```bash
go get -u ./...
```

### Clear build, test, and module cache

Removes the build cache, test cache, and downloaded module cache.

```bash
go clean -cache -testcache -modcache
```

### Run unit tests with coverage

Runs all tests in the project and generates the `coverage.out` file.

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
```

### Generate HTML coverage report

Converts `coverage.out` into an HTML coverage report.

```bash
go tool cover -html="coverage.out" -o "coverage.html"
```

### Show coverage from `coverage.out`

Prints per-function coverage information in the terminal.

```bash
go tool cover -func="coverage.out"
```
