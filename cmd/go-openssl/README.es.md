# go-openssl

`go-openssl` es la CLI de QuicksGo para generar certificados y llaves PEM para
RSA, ECC/ECDSA o Ed25519. Puede crear certificados autofirmados, certificados
CA, certificados firmados por una CA existente y envoltorios PEM cifrados que
luego pueden leerse desde la CLI o desde la API Go.

## Instalacion

```bash
go install github.com/PointerByte/QuicksGo/cmd/go-openssl@latest
```

## Comandos

Generar archivos PEM:

```bash
go-openssl generate [flags]
```

Leer un archivo PEM plano o cifrado:

```bash
go-openssl read --file ./certs/cert.pem
```

## Defaults De Generacion

Cuando omites flags, la generacion usa:

- algoritmo: `rsa`
- directorio de salida: `.`
- common name: `localhost`
- DNS SAN: `localhost`
- organizacion: `PointerByte`
- vigencia: `365` dias
- tamano RSA: `2048` bits
- curva ECC: `p256`
- archivos: `cert.pem`, `key.pem`, `public.pem`

Las llaves privadas se escriben con modo `0600`; certificados y llaves publicas
se escriben con modo `0644`.

## Flags De Generate

| Flag | Short | Descripcion |
| --- | --- | --- |
| `--algorithm` | `-a` | `rsa`, `ecc` o `ed25519` |
| `--dir` | `-d` | directorio de salida |
| `--common-name` | `-n` | common name del certificado |
| `--dns` | | Subject Alternative Name DNS; puede repetirse o ir separado por comas |
| `--organization` | | organizacion del subject del certificado |
| `--days` | | vigencia del certificado en dias |
| `--rsa-bits` | | tamano de llave RSA en bits; minimo `2048` |
| `--ecc-curve` | | `p256`, `p384` o `p521` |
| `--salt` | | entropia adicional opcional mezclada en la generacion |
| `--cert-file` | | nombre del archivo de certificado |
| `--key-file` | | nombre del archivo de llave privada |
| `--public-key-file` | | nombre del archivo de llave publica |
| `--signed-by` | | ruta del certificado CA PEM que firma el nuevo certificado |
| `--ca-key` | | ruta de la llave privada CA usada con `--signed-by` |
| `--ca` | | marca el certificado generado como CA |
| `--encrypt-secret` | | cifra los PEM generados; debe tener al menos 32 bytes |
| `--signed-by-secret` | | secreto para leer un certificado `--signed-by` cifrado |
| `--ca-key-secret` | | secreto para leer una llave privada `--ca-key` cifrada |

`--signed-by` y `--ca-key` deben pasarse juntos. Si algun archivo de CA esta
cifrado, pasa el secreto correspondiente con `--signed-by-secret` o
`--ca-key-secret`.

## Flags De Read

| Flag | Short | Descripcion |
| --- | --- | --- |
| `--file` | `-f` | archivo PEM plano o cifrado a leer |
| `--secret` | `-s` | secreto usado para descifrar archivos PEM cifrados |
| `--out` | `-o` | destino opcional para el PEM descifrado |

Si omites `--out`, el comando escribe el contenido PEM en stdout.

## Ejemplos Basicos

Generar un certificado RSA autofirmado:

```bash
go-openssl generate --algorithm rsa --dir ./certs
```

Generar un certificado ECC:

```bash
go-openssl generate \
  --algorithm ecc \
  --ecc-curve p384 \
  --dir ./certs/ecc \
  --common-name api.default.svc \
  --dns api.default.svc \
  --dns api.default.svc.cluster.local
```

Generar certificado y par de llaves Ed25519:

```bash
go-openssl generate \
  --algorithm ed25519 \
  --dir ./certs/jwt \
  --common-name jwt-signing.default.svc \
  --key-file key.pem \
  --public-key-file public.pem
```

## Ejemplo CA Y mTLS

Crear una CA:

```bash
go-openssl generate \
  --algorithm rsa \
  --rsa-bits 4096 \
  --ca \
  --dir ./certs/ca \
  --common-name internal-ca.example.local \
  --organization "Example Internal CA" \
  --days 3650 \
  --cert-file ca.pem \
  --key-file ca-key.pem \
  --public-key-file ca-public.pem
```

Crear un certificado de servidor firmado por esa CA:

```bash
go-openssl generate \
  --algorithm ecc \
  --ecc-curve p256 \
  --dir ./certs/server \
  --common-name my-api.default.svc \
  --dns my-api.default.svc \
  --dns my-api.default.svc.cluster.local \
  --organization "Example Platform" \
  --days 365 \
  --signed-by ./certs/ca/ca.pem \
  --ca-key ./certs/ca/ca-key.pem
```

Crear un certificado cliente para mTLS:

```bash
go-openssl generate \
  --algorithm ecc \
  --ecc-curve p256 \
  --dir ./certs/client \
  --common-name my-api-client.default.svc \
  --dns my-api-client.default.svc \
  --organization "Example Platform" \
  --days 365 \
  --signed-by ./certs/ca/ca.pem \
  --ca-key ./certs/ca/ca-key.pem
```

## Archivos PEM Cifrados

Usa `--encrypt-secret` para cifrar `cert.pem`, `key.pem` y `public.pem` como
envoltorios `QUICKSGO ENCRYPTED PEM` con AES-256-GCM. El secreto debe tener al
menos 32 bytes.

```bash
go-openssl generate \
  --algorithm rsa \
  --rsa-bits 4096 \
  --dir ./certs/encrypted \
  --common-name api.default.svc \
  --encrypt-secret "12345678901234567890123456789012"
```

Leer un PEM cifrado hacia stdout:

```bash
go-openssl read \
  --file ./certs/encrypted/cert.pem \
  --secret "12345678901234567890123456789012"
```

Escribir el PEM descifrado en un archivo nuevo:

```bash
go-openssl read \
  --file ./certs/encrypted/key.pem \
  --secret "12345678901234567890123456789012" \
  --out ./certs/encrypted/key.decrypted.pem
```

Usar una CA cifrada para firmar otro certificado:

```bash
go-openssl generate \
  --algorithm ecc \
  --ecc-curve p384 \
  --dir ./certs/service \
  --common-name service.default.svc \
  --dns service.default.svc \
  --signed-by ./certs/ca/ca.pem \
  --ca-key ./certs/ca/ca-key.pem \
  --signed-by-secret "12345678901234567890123456789012" \
  --ca-key-secret "12345678901234567890123456789012"
```

## Ejemplos Kubernetes

Certificado backend para un servicio detras de un Ingress:

```bash
go-openssl generate \
  --algorithm rsa \
  --rsa-bits 4096 \
  --dir ./certs/my-api \
  --common-name my-api.default.svc \
  --dns my-api.default.svc \
  --dns my-api.default.svc.cluster.local \
  --dns api.example.com \
  --organization "Example Platform" \
  --days 365
```

Certificado interno servicio-a-servicio:

```bash
go-openssl generate \
  --algorithm ecc \
  --ecc-curve p256 \
  --dir ./certs/orders-to-payments \
  --common-name payments.default.svc \
  --dns payments.default.svc \
  --dns payments.default.svc.cluster.local \
  --organization "Example Internal Services" \
  --days 365
```

## API Go

El generador tambien puede usarse directamente desde Go:

```go
package main

import (
	"log"

	goopenssl "github.com/PointerByte/QuicksGo/cmd/go-openssl/code"
)

func main() {
	result, err := goopenssl.GenerateCertificates(goopenssl.Options{
		Algorithm:    "ecc",
		ECCCurve:     "p256",
		OutputDir:    "./certs",
		CommonName:   "localhost",
		DNSNames:     []string{"localhost"},
		IPAddresses:  []string{"127.0.0.1"},
		Organization: "Example",
		ValidForDays: 365,
	})
	if err != nil {
		log.Fatal(err)
	}

	cert, err := goopenssl.ReadCertificateFile(result.CertificatePath, "")
	if err != nil {
		log.Fatal(err)
	}

	_ = cert
}
```

`go-openssl generate` corresponde a campos de `goopenssl.Options`:

| Flag CLI | Campo Go |
| --- | --- |
| `--algorithm` | `Algorithm` |
| `--dir` | `OutputDir` |
| `--common-name` | `CommonName` |
| `--dns` | `DNSNames` |
| solo API Go | `IPAddresses` |
| `--organization` | `Organization` |
| `--days` | `ValidForDays` |
| `--rsa-bits` | `RSAKeySize` |
| `--ecc-curve` | `ECCCurve` |
| `--salt` | `Salt` |
| `--cert-file` | `CertFileName` |
| `--key-file` | `KeyFileName` |
| `--public-key-file` | `PublicKeyFileName` |
| `--signed-by` | `SignedBy` |
| `--ca-key` | `CAKeyFile` |
| `--ca` | `IsCA` |
| `--encrypt-secret` | `EncryptSecret` |
| `--signed-by-secret` | `SignedBySecret` |
| `--ca-key-secret` | `CAKeySecret` |

Helpers de lectura:

- `ReadPEMFile(path, secret)`
- `ReadCertificateFile(path, secret)`
- `ReadPrivateKeyFile(path, secret)`
- `ReadPublicKeyFile(path, secret)`

Los PEM planos pueden leerse con secreto vacio. Los PEM cifrados requieren el
mismo secreto usado durante la generacion.

## Desarrollo

Desde el directorio del modulo `cmd/go-openssl`:

```bash
go test ./...
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
```
