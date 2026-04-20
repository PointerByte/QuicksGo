# QuicksGo Encrypt

`encrypt` is the standalone cryptography module for QuicksGo. It exposes a repository-based API for symmetric encryption, hashing, RSA helpers, and digital signatures while letting you choose the backend implementation that best fits your environment.

This module was split out from `security`, so it can now be used independently:

```bash
go get github.com/PointerByte/QuicksGo/encrypt
```

## What it includes

- AES-GCM encryption and decryption
- HMAC generation and validation
- SHA-256 and BLAKE3 hashing
- RSA key generation and RSA-OAEP helpers
- Ed25519 signing and verification
- Pluggable backends for local and cloud-backed implementations

## Available packages

- `github.com/PointerByte/QuicksGo/encrypt`
- `github.com/PointerByte/QuicksGo/encrypt/local`
- `github.com/PointerByte/QuicksGo/encrypt/aws-kms`
- `github.com/PointerByte/QuicksGo/encrypt/azure-key-vault`
- `github.com/PointerByte/QuicksGo/encrypt/gcp-kms`

## Repository model

The root package exposes the shared interfaces and a small constructor:

```go
repository := encrypt.NewRepository(local.NewRepository())
```

`encrypt.NewRepository` receives a backend implementation that satisfies `encrypt.IRepository` and returns a composed repository exposing:

- `SymmetricRepository`
- `AsymmetricRepository`
- `SignatureRepository`
- `HashRepository`

## Quick start

### Initialize a repository

```go
package main

import (
	"context"

	"github.com/PointerByte/QuicksGo/encrypt"
	"github.com/PointerByte/QuicksGo/encrypt/local"
)

func main() {
	ctx := context.Background()
	repository := encrypt.NewRepository(local.NewRepository())

	_, _ = ctx, repository
}
```

### AES-GCM

```go
keyData, err := repository.GenerateSymetrycKeys(ctx, 32)
if err != nil {
	panic(err)
}

additional := "aad"
cipherText, err := repository.EncryptAES(ctx, keyData.Key, "hello", &additional)
if err != nil {
	panic(err)
}

plainText, err := repository.DecryptAES(ctx, keyData.Key, cipherText, &additional)
if err != nil {
	panic(err)
}

_, _ = cipherText, plainText
```

### HMAC and hashes

```go
signature := repository.GenerateHMAC(ctx, "secret", "message")
valid := repository.ValidateHMAC(ctx, "secret", "message", signature)
sha := repository.Sha256Hex(ctx, "message")
blake := repository.Blake3(ctx, "message")

_, _, _ = valid, sha, blake
```

### RSA-OAEP

```go
keys, err := repository.GenerateRSAKeys(ctx, 2048)
if err != nil {
	panic(err)
}

cipherText, err := repository.RSA_OAEP_Encode(ctx, keys.PublicKey, "hello")
if err != nil {
	panic(err)
}

plainText, err := repository.RSA_OAEP_Decode(ctx, keys.KeyID, cipherText)
if err != nil {
	panic(err)
}

_, _ = cipherText, plainText
```

### Ed25519

```go
keys, err := repository.GenerateEd255Keys(ctx, 2048)
if err != nil {
	panic(err)
}

signature, err := repository.SignEd25519(ctx, keys.KeyID, "hello")
if err != nil {
	panic(err)
}

if err := repository.VerifyEd25519(ctx, keys.PublicKey, "hello", signature); err != nil {
	panic(err)
}
```

## Backends

### `local`

Use `local.NewRepository()` for development, tests, and cases where key material can live in-process.

### `aws-kms`

Use `aws-kms.NewRepository()` when you want AWS KMS-backed operations where the provider supports them.

### `azure-key-vault`

Use `azure-key-vault.NewRepository()` for Azure Key Vault-oriented flows, with local fallbacks for unsupported primitives.

### `gcp-kms`

Use `gcp-kms.NewRepository()` for Google Cloud KMS-oriented flows, with local fallbacks for unsupported primitives.

## Relationship with `security`

`security` depends on this module for signing and cryptographic helpers, but `encrypt` is no longer part of the `security` module. If you only need cryptographic primitives, depend on `github.com/PointerByte/QuicksGo/encrypt` directly.

## Tests

From the `encrypt` module directory:

```bash
go test ./...
```

With coverage:

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
```
