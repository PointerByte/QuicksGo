# go-openssl

`go-openssl` es la CLI de QuicksGo para generar certificados y llaves `.pem` para RSA, ECC o Ed25519.

## Instalación

```bash
go install github.com/PointerByte/QuicksGo/cmd/go-openssl@latest
```

## Comandos

Generar un certificado RSA autofirmado:

```bash
go-openssl generate --algorithm rsa --dir ./certs
```

Generar un certificado ECC con un `salt` opcional:

```bash
go-openssl generate --algorithm ecc --ecc-curve p384 --dir ./certs --salt my-extra-entropy
```

Generar un certificado Ed25519:

```bash
go-openssl generate --algorithm ed25519 --dir ./certs
```

El generador permite controlar:

- el algoritmo: `rsa`, `ecc` o `ed25519`
- el directorio de salida con `--dir`
- el Common Name del certificado con `--common-name`
- los Subject Alternative Names DNS con `--dns` repetidos
- la organización con `--organization`
- la vigencia con `--days`
- el tamaño de llave RSA con `--rsa-bits`
- la curva ECC con `--ecc-curve`
- los nombres de archivo con `--cert-file`, `--key-file` y `--public-key-file`
- firmar con una CA existente usando `--signed-by` y `--ca-key`
- si el certificado debe ser CA con `--ca`
- entropía adicional opcional con `--salt`

El comando escribe por defecto dentro del directorio seleccionado:

- `cert.pem`
- `key.pem`
- `public.pem`

Cuando usas `--signed-by`, tambien debes pasar `--ca-key`. El certificado
generado queda firmado por esa CA, mientras que `key.pem` y `public.pem`
pertenecen al nuevo certificado.

### Equivalencia entre CLI y Go

Cada flag de `go-openssl generate` corresponde a un campo en
`goopenssl.Options` cuando llamas al generador desde Go:

| Flag CLI | Campo Go |
| --- | --- |
| `--algorithm` | `Algorithm` |
| `--dir` | `OutputDir` |
| `--common-name` | `CommonName` |
| `--dns` | `DNSNames` |
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

Los ejemplos Go de abajo asumen este import:

```go
import goopenssl "github.com/PointerByte/QuicksGo/cmd/go-openssl/code"
```

## Ejemplos Por Caso De Uso

Cada ejemplo CLI tiene su equivalente en Go usando `GenerateCertificates`.

### Crear un certificado CA por separado

Usa `--ca` solo para una autoridad certificadora. Nombrar el certificado
`CA.pem` tiene sentido en este caso porque se usa como ancla de confianza, no
como certificado individual de un servicio.

```bash
go-openssl generate \
  --algorithm rsa \
  --rsa-bits 4096 \
  --ca \
  --dir ./certs/internal-ca \
  --common-name internal-ca.example.local \
  --dns internal-ca.example.local \
  --organization "Example Internal CA" \
  --days 3650 \
  --cert-file CA.pem \
  --key-file CA-key.pem \
  --public-key-file CA-public.pem
```

Codigo Go equivalente:

```go
_, err := goopenssl.GenerateCertificates(goopenssl.Options{
	Algorithm:         "rsa",
	RSAKeySize:        4096,
	IsCA:              true,
	OutputDir:         "./certs/internal-ca",
	CommonName:        "internal-ca.example.local",
	DNSNames:          []string{"internal-ca.example.local"},
	Organization:      "Example Internal CA",
	ValidForDays:      3650,
	CertFileName:      "CA.pem",
	KeyFileName:       "CA-key.pem",
	PublicKeyFileName: "CA-public.pem",
})
```

### Crear un certificado de servicio firmado por una CA

Genera primero la CA, luego pasa su certificado y llave privada con
`--signed-by` y `--ca-key` al generar el certificado de servicio.

```bash
go-openssl generate \
  --algorithm ecc \
  --ecc-curve p521 \
  --ca \
  --dir ./certs/server \
  --common-name dragon-cmk-ca.chest-max.svc.cluster.local \
  --organization "Example Platform" \
  --days 365 \
  --cert-file ca.pem \
  --key-file ca-key.pem \
  --public-key-file ca-public.pem

go-openssl generate \
  --algorithm ecc \
  --ecc-curve p521 \
  --dir ./certs/server \
  --common-name dragon-cmk.chest-max.svc.cluster.local \
  --dns dragon-cmk.chest-max.svc.cluster.local \
  --organization "Example Platform" \
  --days 365 \
  --cert-file cert.pem \
  --key-file key.pem \
  --public-key-file public.pem \
  --signed-by ./certs/server/ca.pem \
  --ca-key ./certs/server/ca-key.pem
```

Codigo Go equivalente:

```go
caResult, err := goopenssl.GenerateCertificates(goopenssl.Options{
	Algorithm:         "ecc",
	ECCCurve:          "p521",
	IsCA:              true,
	OutputDir:         "./certs/server",
	CommonName:        "dragon-cmk-ca.chest-max.svc.cluster.local",
	Organization:      "Example Platform",
	ValidForDays:      365,
	CertFileName:      "ca.pem",
	KeyFileName:       "ca-key.pem",
	PublicKeyFileName: "ca-public.pem",
})
if err != nil {
	return err
}

_, err = goopenssl.GenerateCertificates(goopenssl.Options{
	Algorithm:         "ecc",
	ECCCurve:          "p521",
	OutputDir:         "./certs/server",
	CommonName:        "dragon-cmk.chest-max.svc.cluster.local",
	DNSNames:          []string{"dragon-cmk.chest-max.svc.cluster.local"},
	Organization:      "Example Platform",
	ValidForDays:      365,
	CertFileName:      "cert.pem",
	KeyFileName:       "key.pem",
	PublicKeyFileName: "public.pem",
	SignedBy:          caResult.CertificatePath,
	CAKeyFile:         caResult.PrivateKeyPath,
})
```

### Servicio Kubernetes detras de un Ingress

Usa este caso cuando el dominio publico termina TLS en un Ingress, pero el
contenedor de la aplicacion tambien necesita su propio certificado para TLS de
backend, mTLS, sidecars, service meshes o conexiones directas pod-a-servicio.

El certificado del Ingress para `api.example.com` y el certificado interno del
servicio son materiales distintos. El certificado del servicio debe incluir los
DNS del Service de Kubernetes y tambien puede incluir el dominio publico cuando
el servicio valida ese hostname directamente.

Certificado RSA con llave de 4096 bits:

```bash
go-openssl generate \
  --algorithm rsa \
  --rsa-bits 4096 \
  --dir ./certs/my-api-public-rsa \
  --common-name my-api.default.svc \
  --dns my-api.default.svc \
  --dns my-api.default.svc.cluster.local \
  --dns api.example.com \
  --organization "Example Platform" \
  --days 365
```

Codigo Go equivalente:

```go
_, err := goopenssl.GenerateCertificates(goopenssl.Options{
	Algorithm:    "rsa",
	RSAKeySize:   4096,
	OutputDir:    "./certs/my-api-public-rsa",
	CommonName:   "my-api.default.svc",
	DNSNames:     []string{"my-api.default.svc", "my-api.default.svc.cluster.local", "api.example.com"},
	Organization: "Example Platform",
	ValidForDays: 365,
})
```

Certificado ECC/ECDSA con llave P-256:

```bash
go-openssl generate \
  --algorithm ecc \
  --ecc-curve p256 \
  --dir ./certs/my-api-public-ecc \
  --common-name my-api.default.svc \
  --dns my-api.default.svc \
  --dns my-api.default.svc.cluster.local \
  --dns api.example.com \
  --organization "Example Platform" \
  --days 365
```

Codigo Go equivalente:

```go
_, err := goopenssl.GenerateCertificates(goopenssl.Options{
	Algorithm:    "ecc",
	ECCCurve:     "p256",
	OutputDir:    "./certs/my-api-public-ecc",
	CommonName:   "my-api.default.svc",
	DNSNames:     []string{"my-api.default.svc", "my-api.default.svc.cluster.local", "api.example.com"},
	Organization: "Example Platform",
	ValidForDays: 365,
})
```

Usa `--ecc-curve p384` o `ECCCurve: "p384"` si tu entorno requiere una curva
mas grande.

### Comunicacion interna entre servicios

Usa este caso cuando un workload de Kubernetes habla con otro directamente, por
ejemplo `orders` llamando a `payments` dentro del cluster. Prefiere DNS internos
de Kubernetes en la lista SAN.

RSA es la opcion de mayor compatibilidad:

```bash
go-openssl generate \
  --algorithm rsa \
  --rsa-bits 2048 \
  --dir ./certs/orders-to-payments-rsa \
  --common-name payments.default.svc \
  --dns payments.default.svc \
  --dns payments.default.svc.cluster.local \
  --organization "Example Internal Services" \
  --days 365
```

Codigo Go equivalente:

```go
_, err := goopenssl.GenerateCertificates(goopenssl.Options{
	Algorithm:    "rsa",
	RSAKeySize:   2048,
	OutputDir:    "./certs/orders-to-payments-rsa",
	CommonName:   "payments.default.svc",
	DNSNames:     []string{"payments.default.svc", "payments.default.svc.cluster.local"},
	Organization: "Example Internal Services",
	ValidForDays: 365,
})
```

ECC/ECDSA mantiene llaves mas pequenas y funciona bien cuando todos los
clientes lo soportan:

```bash
go-openssl generate \
  --algorithm ecc \
  --ecc-curve p256 \
  --dir ./certs/orders-to-payments-ecc \
  --common-name payments.default.svc \
  --dns payments.default.svc \
  --dns payments.default.svc.cluster.local \
  --organization "Example Internal Services" \
  --days 365
```

Codigo Go equivalente:

```go
_, err := goopenssl.GenerateCertificates(goopenssl.Options{
	Algorithm:    "ecc",
	ECCCurve:     "p256",
	OutputDir:    "./certs/orders-to-payments-ecc",
	CommonName:   "payments.default.svc",
	DNSNames:     []string{"payments.default.svc", "payments.default.svc.cluster.local"},
	Organization: "Example Internal Services",
	ValidForDays: 365,
})
```

### Llaves Ed25519 para firmas JWT

Usa Ed25519 cuando tu capa JWT firma tokens con EdDSA. El comando tambien
escribe `cert.pem`, `key.pem` y `public.pem`; para firmar JWT normalmente
necesitas la llave privada y la publica.

```bash
go-openssl generate \
  --algorithm ed25519 \
  --dir ./certs/jwt \
  --common-name jwt-signing.default.svc \
  --dns jwt-signing.default.svc \
  --organization "Example Security" \
  --days 365 \
  --key-file key.pem \
  --public-key-file public.pem
```

Codigo Go equivalente:

```go
_, err := goopenssl.GenerateCertificates(goopenssl.Options{
	Algorithm:         "ed25519",
	OutputDir:         "./certs/jwt",
	CommonName:        "jwt-signing.default.svc",
	DNSNames:          []string{"jwt-signing.default.svc"},
	Organization:      "Example Security",
	ValidForDays:      365,
	KeyFileName:       "key.pem",
	PublicKeyFileName: "public.pem",
})
```

Si tu configuracion espera valores DER en Base64, convierte los archivos PEM
con `openssl pkey`:

```bash
openssl pkey -in ./certs/jwt/key.pem \
  -outform DER \
  | base64 -w 0

openssl pkey -pubin -in ./certs/jwt/public.pem \
  -outform DER \
  | base64 -w 0
```

Esos valores pueden guardarse en claves de configuracion como
`jwt.eddsa.private_key` y `jwt.eddsa.public_key`.

## Uso Desde Go

La dependencia también puede llamarse directamente desde código Go:

```go
package main

import (
	"log"

	goopenssl "github.com/PointerByte/QuicksGo/cmd/go-openssl/code"
)

func main() {
	_, err := goopenssl.GenerateCertificates(goopenssl.Options{
		Algorithm:  "ecc",
		ECCCurve:   "p256",
		OutputDir:  "./certs",
		CommonName: "localhost",
		Salt:       "my-extra-entropy",
	})
	if err != nil {
		log.Fatal(err)
	}
}
```
