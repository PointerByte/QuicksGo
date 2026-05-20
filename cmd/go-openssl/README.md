# go-openssl

`go-openssl` is the GoForge CLI for generating PEM certificates and keys for
RSA, ECC/ECDSA, or Ed25519. It can create self-signed certificates, CA
certificates, certificates signed by an existing CA, and encrypted PEM
envelopes that can later be read back by the CLI or the Go API.

## Install

```bash
go install github.com/PointerByte/GoForge/cmd/go-openssl@latest
```

Update the dependencies used by the current module:

```bash
go get -u ./...
```

## Commands

Generate PEM files:

```bash
go-openssl generate [flags]
```

Read a plain or encrypted PEM file:

```bash
go-openssl read --file ./certs/cert.pem
```

## Generate Defaults

When flags are omitted, generation uses:

- algorithm: `rsa`
- output directory: `.`
- common name: `localhost`
- DNS SAN: `localhost`
- organization: `PointerByte`
- validity: `365` days
- RSA size: `2048` bits
- ECC curve: `p256`
- files: `cert.pem`, `key.pem`, `public.pem`

Private keys are written with mode `0600`; certificate and public key files are
written with mode `0644`.

## Generate Flags

| Flag | Short | Description |
| --- | --- | --- |
| `--algorithm` | `-a` | `rsa`, `ecc`, or `ed25519` |
| `--dir` | `-d` | output directory |
| `--common-name` | `-n` | certificate common name |
| `--dns` | | DNS Subject Alternative Name; may be repeated or comma-separated |
| `--organization` | | certificate subject organization |
| `--days` | | certificate validity in days |
| `--rsa-bits` | | RSA key size in bits; minimum `2048` |
| `--ecc-curve` | | `p256`, `p384`, or `p521` |
| `--salt` | | optional extra entropy mixed into generation |
| `--cert-file` | | certificate file name |
| `--key-file` | | private key file name |
| `--public-key-file` | | public key file name |
| `--signed-by` | | CA certificate PEM path used to sign the new certificate |
| `--ca-key` | | CA private key PEM path used with `--signed-by` |
| `--ca` | | mark the generated certificate as a CA |
| `--encrypt-secret` | | encrypt generated PEM files; must be at least 32 bytes |
| `--signed-by-secret` | | secret used to read an encrypted `--signed-by` certificate |
| `--ca-key-secret` | | secret used to read an encrypted `--ca-key` private key |

`--signed-by` and `--ca-key` must be provided together. If either CA file is
encrypted, pass the matching secret with `--signed-by-secret` or
`--ca-key-secret`.

## Read Flags

| Flag | Short | Description |
| --- | --- | --- |
| `--file` | `-f` | plain or encrypted PEM file to read |
| `--secret` | `-s` | secret used to decrypt encrypted PEM files |
| `--out` | `-o` | optional destination for decrypted PEM output |

If `--out` is omitted, the command writes the PEM content to stdout.

## Basic Examples

Generate a self-signed RSA certificate:

```bash
go-openssl generate --algorithm rsa --dir ./certs
```

Generate an ECC certificate:

```bash
go-openssl generate \
  --algorithm ecc \
  --ecc-curve p384 \
  --dir ./certs/ecc \
  --common-name api.default.svc \
  --dns api.default.svc \
  --dns api.default.svc.cluster.local
```

Generate an Ed25519 certificate and key pair:

```bash
go-openssl generate \
  --algorithm ed25519 \
  --dir ./certs/jwt \
  --common-name jwt-signing.default.svc \
  --key-file key.pem \
  --public-key-file public.pem
```

## CA And mTLS Example

Create a CA:

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

Create a server certificate signed by that CA:

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

Create a client certificate for mTLS:

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

## Encrypted PEM Files

Use `--encrypt-secret` to encrypt `cert.pem`, `key.pem`, and `public.pem` as
`GoForge ENCRYPTED PEM` envelopes using AES-256-GCM. The secret must be at
least 32 bytes.

```bash
go-openssl generate \
  --algorithm rsa \
  --rsa-bits 4096 \
  --dir ./certs/encrypted \
  --common-name api.default.svc \
  --encrypt-secret "12345678901234567890123456789012"
```

Read an encrypted PEM to stdout:

```bash
go-openssl read \
  --file ./certs/encrypted/cert.pem \
  --secret "12345678901234567890123456789012"
```

Write the decrypted PEM to a new file:

```bash
go-openssl read \
  --file ./certs/encrypted/key.pem \
  --secret "12345678901234567890123456789012" \
  --out ./certs/encrypted/key.decrypted.pem
```

Use an encrypted CA to sign another certificate:

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

## Kubernetes Examples

Backend certificate for a service behind an Ingress:

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

Internal service-to-service certificate:

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

## Go API

The generator can also be used directly from Go:

```go
package main

import (
	"log"

	goopenssl "github.com/PointerByte/GoForge/cmd/go-openssl/code"
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

`go-openssl generate` maps to `goopenssl.Options` fields:

| CLI flag | Go field |
| --- | --- |
| `--algorithm` | `Algorithm` |
| `--dir` | `OutputDir` |
| `--common-name` | `CommonName` |
| `--dns` | `DNSNames` |
| Go API only | `IPAddresses` |
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

Reader helpers:

- `ReadPEMFile(path, secret)`
- `ReadCertificateFile(path, secret)`
- `ReadPrivateKeyFile(path, secret)`
- `ReadPublicKeyFile(path, secret)`

Plain PEM files can be read with an empty secret. Encrypted PEM files require
the same secret used during generation.

## Development

From the `cmd/go-openssl` module directory:

```bash
go test ./...
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
```
