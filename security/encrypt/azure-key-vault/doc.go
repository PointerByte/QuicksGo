// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package azurekeyvault provides the same repository-style cryptographic API as
// the local package, specialized for Azure Key Vault integration points.
//
// Until Azure SDK integration is added to this repository, the package keeps
// local-only primitives delegated to the local implementation and returns
// explicit errors for provider-managed asymmetric operations that require a Key
// Vault-backed private key.
//
// When a provider key identifier is needed, the package reads it from viper
// using "encrypt.vault.azure-key-vault.key-id".
package azurekeyvault
