// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package encrypt provides high-level cryptographic repositories for the
// security module.
//
// The package groups helpers for:
//   - symmetric encryption with AES-GCM
//   - hashing and HMAC generation
//   - RSA key generation and RSA-OAEP encryption
//   - Ed25519 and RSA-based digital signatures
//
// Applications can depend on the focused repository interfaces when they need
// only one capability, or use NewRepository to obtain a combined entry point
// for the main encryption services. Every operation receives a context.Context
// so callers can control request scope, deadlines, and cancellation across
// local and provider-backed implementations.
//
// NewRepository selects its backend from viper key "encrypt.vault.mode".
// Supported values are:
//   - "local" for in-process cryptography
//   - "aws-kms" for AWS KMS-backed repositories
//   - "azure-key-vault" for Azure Key Vault-backed repositories
//   - "gcp-kms" for Google Cloud KMS-backed repositories
//
// When the configuration value is empty or unsupported, NewRepository falls
// back to the local repository implementation.
package encrypt
