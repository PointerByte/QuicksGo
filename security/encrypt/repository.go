// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package encrypt

import (
	awskms "github.com/PointerByte/QuicksGo/security/encrypt/aws-kms"
	azurekeyvault "github.com/PointerByte/QuicksGo/security/encrypt/azure-key-vault"
	gcpkms "github.com/PointerByte/QuicksGo/security/encrypt/gcp-kms"
	"github.com/PointerByte/QuicksGo/security/encrypt/local"
	"github.com/spf13/viper"
)

type Mode string

const (
	Local         Mode = "local"
	AwsKMS        Mode = "aws-kms"
	AzureKeyVault Mode = "azure-key-vault"
	GpcKMS        Mode = "gcp-kms"
)

// NewRepository returns a combined repository with the main cryptographic
// capabilities exposed by this package.
//
// The selected backend is controlled by viper key "encrypt.vault.mode".
// Supported values are "local", "aws-kms", "azure-key-vault", and "gcp-kms".
// When the value is empty or does not match a known mode, the function falls
// back to the local implementation.
func NewRepository() Repository {
	strMode := viper.GetString("encrypt.vault.mode")
	switch Mode(strMode) {
	case AwsKMS:
		return awskms.NewRepository()
	case AzureKeyVault:
		return azurekeyvault.NewRepository()
	case GpcKMS:
		return gcpkms.NewRepository()
	default:
		return local.NewRepository()
	}
}
