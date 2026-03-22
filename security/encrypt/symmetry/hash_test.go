// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package symmetry

import "testing"

func TestGenerateAndValidateHMAC(t *testing.T) {
	message := "hello world"
	secret := "super-secret"

	hash := GenerateHMAC(message, secret)
	if hash == "" {
		t.Fatal("expected non-empty hmac")
	}

	if !ValidateHMAC(message, secret, hash) {
		t.Fatal("expected hmac to validate")
	}

	if ValidateHMAC(message, secret, GenerateHMAC("other", secret)) {
		t.Fatal("expected hmac validation to fail for different message")
	}
}

func TestSha256Hex(t *testing.T) {
	got := Sha256Hex("hello")
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBlake3(t *testing.T) {
	got := Blake3("hello")
	want := "6o8WPbOGgpJeRJHF5Y1Ls1Bu+MFOt4qG6QjFYkpnIA8="

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
