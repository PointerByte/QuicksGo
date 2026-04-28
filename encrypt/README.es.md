# QuicksGo Encrypt

`encrypt` es el modulo de criptografia independiente de QuicksGo. Expone una API basada en repositorios para cifrado simetrico, hashing, utilidades RSA y firmas digitales, permitiendo elegir el backend que mejor se adapte al entorno.

Este modulo se separo de `security`, por lo que ahora puede usarse por si solo:

```bash
go get github.com/PointerByte/QuicksGo/encrypt
```

## Que incluye

- Cifrado y descifrado AES-GCM
- Generacion y validacion de HMAC
- Hashing con SHA-256 y BLAKE3
- Generacion de claves RSA y utilidades RSA-OAEP
- Firma y verificacion con Ed25519
- Backends intercambiables para implementaciones locales y basadas en nube

## Paquetes disponibles

- `github.com/PointerByte/QuicksGo/encrypt`
- `github.com/PointerByte/QuicksGo/encrypt/local`
- `github.com/PointerByte/QuicksGo/encrypt/aws-kms`
- `github.com/PointerByte/QuicksGo/encrypt/azure-key-vault`
- `github.com/PointerByte/QuicksGo/encrypt/gcp-kms`

## Modelo de repositorio

El paquete raiz expone las interfaces compartidas y un constructor pequeno:

```go
repository := encrypt.NewRepository(local.NewRepository())
```

`encrypt.NewRepository` recibe una implementacion que satisface `encrypt.IRepository` y devuelve un repositorio compuesto que expone:

- `SymmetricRepository`
- `AsymmetricRepository`
- `SignatureRepository`
- `HashRepository`

## Inicio rapido

### Inicializar un repositorio

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
cipherText, err := repository.EncryptAES(ctx, keyData.Key, "hola", &additional)
if err != nil {
	panic(err)
}

plainText, err := repository.DecryptAES(ctx, keyData.Key, cipherText, &additional)
if err != nil {
	panic(err)
}

_, _ = cipherText, plainText
```

### HMAC y hashes

```go
signature := repository.HMAC(ctx, "secret", "message")
sha := repository.Sha256Hex(ctx, "message")
blake := repository.Blake3(ctx, "message")

_, _, _ = signature, sha, blake
```

### RSA-OAEP

```go
keys, err := repository.GenerateRSAKeys(ctx, 2048)
if err != nil {
	panic(err)
}

cipherText, err := repository.RSA_OAEP_Encode(ctx, keys.PublicKey, "hola")
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
keys, err := repository.GenerateEd255Keys(ctx)
if err != nil {
	panic(err)
}

signature, err := repository.SignEd25519(ctx, keys.KeyID, "hola")
if err != nil {
	panic(err)
}

if err := repository.VerifyEd25519(ctx, keys.PublicKey, "hola", signature); err != nil {
	panic(err)
}
```

## Backends

### `local`

Usa `local.NewRepository()` para desarrollo, pruebas y escenarios donde el material criptografico puede vivir en proceso.

### `aws-kms`

Usa `aws-kms.NewRepository()` cuando quieras operaciones respaldadas por AWS KMS donde el proveedor las soporte.

### `azure-key-vault`

Usa `azure-key-vault.NewRepository()` para flujos orientados a Azure Key Vault, con fallbacks locales para primitivas no soportadas.

### `gcp-kms`

Usa `gcp-kms.NewRepository()` para flujos orientados a Google Cloud KMS, con fallbacks locales para primitivas no soportadas.

## Relacion con `security`

`security` depende de este modulo para firmas y utilidades criptograficas, pero `encrypt` ya no forma parte del modulo `security`. Si solo necesitas primitivas criptograficas, depende directamente de `github.com/PointerByte/QuicksGo/encrypt`.

## Pruebas

Desde el directorio del modulo `encrypt`:

```bash
go test ./...
```

Con cobertura:

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
```
