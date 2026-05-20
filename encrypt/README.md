# GoForge Encrypt

`encrypt` is the standalone cryptography module for GoForge. It exposes a
repository-style API for symmetric encryption, hashing, RSA/ECC helpers, and
digital signatures, with interchangeable local and cloud-backed
implementations.

## Installation

```bash
go get github.com/PointerByte/GoForge/encrypt
```

Update the dependencies used by the current module:

```bash
go get -u ./...
```

## Packages

- `github.com/PointerByte/GoForge/encrypt`: shared interfaces and composite repository wrapper
- `github.com/PointerByte/GoForge/encrypt/local`: in-process cryptography
- `github.com/PointerByte/GoForge/encrypt/aws-kms`: AWS KMS-backed operations with local fallback paths
- `github.com/PointerByte/GoForge/encrypt/azure-key-vault`: Azure Key Vault-backed operations with local fallback paths
- `github.com/PointerByte/GoForge/encrypt/gcp-kms`: Google Cloud KMS-backed operations with local fallback paths

## Capabilities

- AES-GCM encryption and decryption
- HMAC-SHA256 generation
- SHA-256 and BLAKE3 hashing
- RSA key generation and RSA-OAEP encryption/decryption
- ECC key generation and ECDH-derived encryption/decryption
- Ed25519 signing and verification
- RSA-PSS and RSA PKCS#1 v1.5 SHA-256 signing and verification
- context-aware APIs for cancellation and deadlines

## Repository Model

The root package exposes focused interfaces:

- `SymmetricRepository`
- `AsymmetricRepository`
- `HashRepository`
- `SignatureRepository`
- `IRepository`, which combines all of them

Use `encrypt.NewRepository(...)` when you want one value that exposes every
capability from a backend implementation:

```go
repository := encrypt.NewRepository(local.NewRepository())
```

Backend packages also expose their own `NewRepository()` constructors:

```go
localRepository := local.NewRepository()
awsRepository := awskms.NewRepository()
azureRepository := azurekeyvault.NewRepository()
gcpRepository := gcpkms.NewRepository()

_, _, _, _ = localRepository, awsRepository, azureRepository, gcpRepository
```

## Key Data

Key-generation methods return `*models.KeyData`:

- `KeyID`: local private key, symmetric key, or provider key identifier
- `PublicKey`: local public key when exportable
- `KeyRef`: provider-specific reference such as ARN, URL, or version name
- `Provider`: backend name, for example `local`, `aws-kms`, `azure-key-vault`, or `gcp-kms`

For local symmetric keys, use `KeyID` as the AES key. For local asymmetric
keys, use `KeyID` as the private key and `PublicKey` as the public key.

## Quick Start

```go
package main

import (
	"context"

	"github.com/PointerByte/GoForge/encrypt"
	"github.com/PointerByte/GoForge/encrypt/common"
	"github.com/PointerByte/GoForge/encrypt/local"
)

func main() {
	ctx := context.Background()
	repository := encrypt.NewRepository(local.NewRepository())

	keyData, err := repository.GenerateSymetrycKeys(ctx, common.Key256Bits)
	if err != nil {
		panic(err)
	}

	additional := "aad"
	cipherText, err := repository.EncryptAES(ctx, keyData.KeyID, "hello", &additional)
	if err != nil {
		panic(err)
	}

	plainText, err := repository.DecryptAES(ctx, keyData.KeyID, cipherText, &additional)
	if err != nil {
		panic(err)
	}

	_ = plainText
}
```

## Hashing And HMAC

```go
hmacValue := repository.HMAC(ctx, "secret", "message")
sha256Value := repository.Sha256Hex(ctx, "message")
blake3Value := repository.Blake3(ctx, "message")

_, _, _ = hmacValue, sha256Value, blake3Value
```

## RSA

```go
keys, err := repository.GenerateRSAKeys(ctx, common.Key2048Bits)
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

signature, err := repository.SignRSAPSS(ctx, keys.KeyID, "payload")
if err != nil {
	panic(err)
}

if err := repository.VerifyRSAPSS(ctx, keys.PublicKey, "payload", signature); err != nil {
	panic(err)
}

_ = plainText
```

## ECC

```go
keys, err := repository.GenerateECCKeys(ctx, common.CurveP256)
if err != nil {
	panic(err)
}

cipherText, err := repository.ECC_Encode(ctx, keys.PublicKey, "hello")
if err != nil {
	panic(err)
}

plainText, err := repository.ECC_Decode(ctx, keys.KeyID, cipherText)
if err != nil {
	panic(err)
}

_ = plainText
```

## Ed25519

```go
keys, err := repository.GenerateEd255Keys(ctx)
if err != nil {
	panic(err)
}

signature, err := repository.SignEd25519(ctx, keys.KeyID, "payload")
if err != nil {
	panic(err)
}

if err := repository.VerifyEd25519(ctx, keys.PublicKey, "payload", signature); err != nil {
	panic(err)
}
```

## Cloud Backends

Cloud packages implement the same repository contract and route operations to
the provider when the supplied key looks like a provider reference. Explicit
local keys are handled by the local fallback where supported.

```go
import (
	awskms "github.com/PointerByte/GoForge/encrypt/aws-kms"
	azurekeyvault "github.com/PointerByte/GoForge/encrypt/azure-key-vault"
	gcpkms "github.com/PointerByte/GoForge/encrypt/gcp-kms"
)
```

Configuration fallback keys:

- AWS KMS: `encrypt.vault.aws-kms.arn`
- Azure Key Vault: `encrypt.vault.azure-key-vault.key-id`
- Google Cloud KMS: `encrypt.vault.gcp-kms.key-id`

Azure and GCP also keep compatibility fallbacks for the older
`encrypt.azure-key-vault.key-id` and `encrypt.gcp-kms.key-id` keys.

## Relationship With `security`

`security` depends on this module internally for JWT signing and cryptographic
helpers, but `encrypt` is independent. Use it directly when your application
needs cryptographic primitives outside JWT middleware.

## Tests

From the `encrypt` module directory:

```bash
go test ./...
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
```
