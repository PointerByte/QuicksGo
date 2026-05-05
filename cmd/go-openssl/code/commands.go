// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package code

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	algorithmRSA     = "rsa"
	algorithmECC     = "ecc"
	algorithmEd25519 = "ed25519"
	curveP256        = "p256"
	curveP384        = "p384"
	curveP521        = "p521"
)

type generateCommand struct {
	app     *App
	options *Options
}

// newGenerateCommand creates the certificate generation command.
func newGenerateCommand(app *App) Command {
	return &generateCommand{
		app:     app,
		options: defaultOptions(),
	}
}

// Cobra creates the executable Cobra command that resolves options and generates PEM files.
func (command *generateCommand) Cobra() *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a certificate and key files",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedOptions, err := normalizeOptions(*command.options)
			if err != nil {
				return err
			}

			result, err := command.app.generator.Generate(resolvedOptions)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(
				command.app.streams.Out,
				"Certificate generated in %s using %s\nCert: %s\nKey: %s\nPublic key: %s\n",
				result.OutputDir,
				strings.ToUpper(result.Algorithm),
				result.CertificatePath,
				result.PrivateKeyPath,
				result.PublicKeyPath,
			)
			return err
		},
	}

	cobraCmd.Flags().StringVarP(&command.options.Algorithm, "algorithm", "a", command.options.Algorithm, "Certificate algorithm: rsa, ecc, or ed25519")
	cobraCmd.Flags().StringVarP(&command.options.OutputDir, "dir", "d", command.options.OutputDir, "Output directory for the generated PEM files")
	cobraCmd.Flags().StringVarP(&command.options.CommonName, "common-name", "n", command.options.CommonName, "Common Name for the self-signed certificate")
	cobraCmd.Flags().StringSliceVar(&command.options.DNSNames, "dns", command.options.DNSNames, "DNS Subject Alternative Names")
	cobraCmd.Flags().StringVar(&command.options.Organization, "organization", command.options.Organization, "Organization value for the certificate subject")
	cobraCmd.Flags().IntVar(&command.options.ValidForDays, "days", command.options.ValidForDays, "Certificate validity in days")
	cobraCmd.Flags().IntVar(&command.options.RSAKeySize, "rsa-bits", command.options.RSAKeySize, "RSA key size in bits")
	cobraCmd.Flags().StringVar(&command.options.ECCCurve, "ecc-curve", command.options.ECCCurve, "ECC curve: p256, p384, or p521")
	cobraCmd.Flags().StringVar(&command.options.Salt, "salt", command.options.Salt, "Optional extra entropy salt used during key and certificate generation")
	cobraCmd.Flags().StringVar(&command.options.CertFileName, "cert-file", command.options.CertFileName, "Certificate file name")
	cobraCmd.Flags().StringVar(&command.options.KeyFileName, "key-file", command.options.KeyFileName, "Private key file name")
	cobraCmd.Flags().StringVar(&command.options.PublicKeyFileName, "public-key-file", command.options.PublicKeyFileName, "Public key file name")
	cobraCmd.Flags().StringVar(&command.options.SignedBy, "signed-by", command.options.SignedBy, "CA certificate PEM file used to sign the generated certificate")
	cobraCmd.Flags().StringVar(&command.options.CAKeyFile, "ca-key", command.options.CAKeyFile, "CA private key PEM file used to sign the generated certificate")
	cobraCmd.Flags().BoolVar(&command.options.IsCA, "ca", command.options.IsCA, "Mark the generated certificate as a certificate authority")
	return cobraCmd
}
