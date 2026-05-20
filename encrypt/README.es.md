# GoForge Encrypt

`encrypt` es el modulo de criptografia independiente de GoForge. Expone una
API estilo repositorio para cifrado simetrico, hashing, utilidades RSA/ECC y
firmas digitales, con implementaciones intercambiables locales y respaldadas
por proveedores cloud.

## Instalacion

```bash
go get github.com/PointerByte/GoForge/encrypt
```

Actualizar las dependencias usadas por el modulo actual:

```bash
go get -u ./...
```

## Paquetes

- `github.com/PointerByte/GoForge/encrypt`: interfaces compartidas y wrapper de repositorio compuesto
- `github.com/PointerByte/GoForge/encrypt/local`: criptografia en proceso
- `github.com/PointerByte/GoForge/encrypt/aws-kms`: operaciones con AWS KMS y fallbacks locales
- `github.com/PointerByte/GoForge/encrypt/azure-key-vault`: operaciones con Azure Key Vault y fallbacks locales
- `github.com/PointerByte/GoForge/encrypt/gcp-kms`: operaciones con Google Cloud KMS y fallbacks locales

## Capacidades

- cifrado y descifrado AES-GCM
- generacion HMAC-SHA256
- hashing SHA-256 y BLAKE3
- generacion de llaves RSA y cifrado/descifrado RSA-OAEP
- generacion de llaves ECC y cifrado/descifrado derivado con ECDH
- firma y verificacion Ed25519
- firma y verificacion RSA-PSS y RSA PKCS#1 v1.5 SHA-256
- APIs con `context.Context` para cancelacion y deadlines

## Modelo De Repositorio

El paquete raiz expone interfaces enfocadas:

- `SymmetricRepository`
- `AsymmetricRepository`
- `HashRepository`
- `SignatureRepository`
- `IRepository`, que combina todas las anteriores

Usa `encrypt.NewRepository(...)` cuando quieras un solo valor que exponga todas
las capacidades de un backend:

```go
repository := encrypt.NewRepository(local.NewRepository())
```

Los paquetes backend tambien exponen sus propios constructores
`NewRepository()`:

```go
localRepository := local.NewRepository()
awsRepository := awskms.NewRepository()
azureRepository := azurekeyvault.NewRepository()
gcpRepository := gcpkms.NewRepository()

_, _, _, _ = localRepository, awsRepository, azureRepository, gcpRepository
```

## KeyData

Los metodos de generacion de llaves devuelven `*models.KeyData`:

- `KeyID`: llave privada local, llave simetrica o identificador de proveedor
- `PublicKey`: llave publica local cuando es exportable
- `KeyRef`: referencia especifica del proveedor, como ARN, URL o version
- `Provider`: nombre del backend, por ejemplo `local`, `aws-kms`, `azure-key-vault` o `gcp-kms`

Para llaves simetricas locales, usa `KeyID` como llave AES. Para llaves
asimetricas locales, usa `KeyID` como llave privada y `PublicKey` como llave
publica.

## Inicio Rapido

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
	cipherText, err := repository.EncryptAES(ctx, keyData.KeyID, "hola", &additional)
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

## Hashing Y HMAC

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

cipherText, err := repository.RSA_OAEP_Encode(ctx, keys.PublicKey, "hola")
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

cipherText, err := repository.ECC_Encode(ctx, keys.PublicKey, "hola")
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

## Backends Cloud

Los paquetes cloud implementan el mismo contrato de repositorio y enrutan
operaciones al proveedor cuando la llave recibida parece una referencia del
proveedor. Las llaves locales explicitas usan fallback local donde esta
soportado.

```go
import (
	awskms "github.com/PointerByte/GoForge/encrypt/aws-kms"
	azurekeyvault "github.com/PointerByte/GoForge/encrypt/azure-key-vault"
	gcpkms "github.com/PointerByte/GoForge/encrypt/gcp-kms"
)
```

Claves de configuracion usadas como fallback:

- AWS KMS: `encrypt.vault.aws-kms.arn`
- Azure Key Vault: `encrypt.vault.azure-key-vault.key-id`
- Google Cloud KMS: `encrypt.vault.gcp-kms.key-id`

Azure y GCP tambien conservan compatibilidad con las claves antiguas
`encrypt.azure-key-vault.key-id` y `encrypt.gcp-kms.key-id`.

## Relacion Con `security`

`security` depende internamente de este modulo para firmas JWT y helpers
criptograficos, pero `encrypt` es independiente. Usalo directamente cuando tu
aplicacion necesite primitivas criptograficas fuera del middleware JWT.

## Pruebas

Desde el directorio del modulo `encrypt`:

```bash
go test ./...
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
```
