// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package gcpkms provides the same repository-style cryptographic API as the
// local package, specialized for Google Cloud KMS integration points.
//
// Until Google Cloud KMS SDK integration is added to this repository, the
// package keeps local-only primitives delegated to the local implementation and
// returns explicit errors for provider-managed asymmetric operations that
// require a Cloud KMS-backed private key.
//
// When a provider key identifier is needed, the package reads it from viper
// using "encrypt.vault.gcp-kms.key-id".
package gcpkms
