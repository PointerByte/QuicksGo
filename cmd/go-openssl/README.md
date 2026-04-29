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

### CLI to Go options

Every `go-openssl generate` flag maps to a field in `goopenssl.Options` when
you call the generator from Go:

| CLI flag | Go field |
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
| `--ca` | `IsCA` |

The Go examples below assume this import:

```go
import goopenssl "github.com/PointerByte/QuicksGo/cmd/go-openssl/code"
```

## Examples by Use Case

Each CLI example has an equivalent Go snippet using `GenerateCertificates`.

### Create a CA certificate separately

Use `--ca` only for a certificate authority. Naming the certificate `CA.pem`
makes sense for this case because it is used as a trust anchor, not as an
individual service certificate.

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

Equivalent Go code:

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

### Kubernetes service behind an Ingress

Use this when the public domain is terminated by an Ingress TLS certificate,
but the application container also needs its own certificate for backend TLS,
mTLS, sidecars, service meshes, or direct pod-to-service connections.

The Ingress certificate for `api.example.com` and the service certificate below
are different pieces of material. The service certificate should include the
Kubernetes service DNS names and can also include the public domain when the
service validates that hostname directly.

RSA certificate with a 4096-bit key:

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

Equivalent Go code:

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

ECC/ECDSA certificate with a P-256 key:

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

Equivalent Go code:

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

Use `--ecc-curve p384` or `ECCCurve: "p384"` if your environment requires a
larger curve.

### Internal service-to-service communication

Use this when one Kubernetes workload talks to another directly, for example
`orders` calling `payments` inside the cluster. Prefer internal Kubernetes DNS
names in the SAN list.

RSA is the compatibility-first option:

```bash
go-openssl generate \
  --algorithm rsa \
  --rsa-bits 4096 \
  --dir ./certs/orders-to-payments-rsa \
  --common-name payments.default.svc \
  --dns payments.default.svc \
  --dns payments.default.svc.cluster.local \
  --organization "Example Internal Services" \
  --days 365
```

Equivalent Go code:

```go
_, err := goopenssl.GenerateCertificates(goopenssl.Options{
	Algorithm:    "rsa",
	RSAKeySize:   4096,
	OutputDir:    "./certs/orders-to-payments-rsa",
	CommonName:   "payments.default.svc",
	DNSNames:     []string{"payments.default.svc", "payments.default.svc.cluster.local"},
	Organization: "Example Internal Services",
	ValidForDays: 365,
})
```

ECC/ECDSA keeps keys smaller and is a good fit when all clients support it:

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

Equivalent Go code:

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

### Ed25519 keys for JWT signatures

Use Ed25519 when your JWT layer signs tokens with EdDSA. The command still
writes `cert.pem`, `key.pem`, and `public.pem`; for JWT signing, the private
and public key files are usually the values you need.

```bash
go-openssl generate \
  --algorithm ed25519 \
  --dir ./certs/jwt-ed25519 \
  --common-name jwt-signing.default.svc \
  --dns jwt-signing.default.svc \
  --organization "Example Security" \
  --days 365 \
  --cert-file cert.pem \
  --key-file key.pem \
  --public-key-file public.pem
```

Equivalent Go code:

```go
_, err := goopenssl.GenerateCertificates(goopenssl.Options{
	Algorithm:         "ed25519",
	OutputDir:         "./certs/jwt-ed25519",
	CommonName:        "jwt-signing.default.svc",
	DNSNames:          []string{"jwt-signing.default.svc"},
	Organization:      "Example Security",
	ValidForDays:      365,
	CertFileName:      "cert.pem",
	KeyFileName:       "key.pem",
	PublicKeyFileName: "public.pem",
})
```

If your configuration expects Base64 DER values, convert the PEM files with
`openssl pkey`:

```bash
openssl pkey -in ./certs/jwt-ed25519/key.pem \
  -outform DER \
  | base64 -w 0

openssl pkey -pubin -in ./certs/jwt-ed25519/public.pem \
  -outform DER \
  | base64 -w 0
```

Those values can be stored in configuration keys such as
`jwt.eddsa.private_key` and `jwt.eddsa.public_key`.

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
