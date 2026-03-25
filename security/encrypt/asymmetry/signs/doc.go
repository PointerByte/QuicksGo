// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package signs provides asymmetric signing and signature-verification helpers
// for the security module.
//
// It includes helpers for:
//   - RSA-PSS signatures encoded as Base64
//   - RSA PKCS#1 v1.5 signatures with SHA-256
//   - Ed25519 signatures encoded as Base64
//
// These helpers are used by higher-level packages such as JWT services but can
// also be consumed directly when applications need explicit signing workflows.
package signs
