// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package rsa provides RSA encryption helpers for the security module.
//
// It focuses on RSA-OAEP encryption and decryption plus Base64 helpers for
// loading PKCS#8 private keys and X.509 public keys.
//
// Main entry points:
//   - ParseRSAPublicKeyFromBase64
//   - ParseRSAPrivateKeyFromBase64
//   - Encode to encrypt plaintext with RSA-OAEP
//   - Decode to decrypt Base64 ciphertext with RSA-OAEP
package rsa
