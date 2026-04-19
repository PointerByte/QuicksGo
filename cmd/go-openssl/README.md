# go-openssl

`go-openssl` is the QuicksGo CLI used to generate `.pem` certificates and keys for RSA, ECC, or Ed25519.

## Install

```bash
go install github.com/PointerByte/QuicksGo/cmd/go-openssl@latest
```

## Commands

Generate a self-signed RSA certificate:

```bash
go-openssl generate --algorithm rsa --dir ./certs
```

Generate an ECC certificate with an optional salt:

```bash
go-openssl generate --algorithm ecc --ecc-curve p384 --dir ./certs --salt my-extra-entropy
```

Generate an Ed25519 certificate:

```bash
go-openssl generate --algorithm ed25519 --dir ./certs
```

The generator can control:

- the algorithm: `rsa`, `ecc`, or `ed25519`
- the output directory through `--dir`
- the certificate common name through `--common-name`
- DNS Subject Alternative Names through repeated `--dns`
- the organization value through `--organization`
- validity time through `--days`
- RSA key size through `--rsa-bits`
- ECC curve through `--ecc-curve`
- the output file names through `--cert-file`, `--key-file`, and `--public-key-file`
- whether the certificate is a CA through `--ca`
- optional extra entropy through `--salt`

The command writes:

- `cert.pem`
- `key.pem`
- `public.pem`

inside the selected directory by default.

## Go Usage

The dependency can also be called directly from Go code:

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
