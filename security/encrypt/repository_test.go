// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package encrypt

import (
	"fmt"
	"testing"
)

func TestNewRepositorySelectsMode(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		kind string
	}{
		{name: "default local", mode: "", kind: "*local.repository"},
		{name: "local", mode: Local, kind: "*local.repository"},
		{name: "aws", mode: AwsKMS, kind: "*awskms.repository"},
		{name: "azure", mode: AzureKeyVault, kind: "*azurekeyvault.repository"},
		{name: "gcp", mode: GpcKMS, kind: "*gcpkms.repository"},
		{name: "unknown fallback", mode: Mode("nope"), kind: "*local.repository"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := typeName(NewRepository(test.mode)); got != test.kind {
				t.Fatalf("NewRepository(%q) type = %s, want %s", test.mode, got, test.kind)
			}
		})
	}
}

func typeName(value any) string {
	return fmt.Sprintf("%T", value)
}
