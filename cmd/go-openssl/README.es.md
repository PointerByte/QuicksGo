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
- si el certificado debe ser CA con `--ca`
- entropía adicional opcional con `--salt`

El comando escribe por defecto dentro del directorio seleccionado:

- `cert.pem`
- `key.pem`
- `public.pem`

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
