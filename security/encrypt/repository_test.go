// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package encrypt

import (
	"fmt"
	"testing"

	"github.com/spf13/viper"
)

func TestNewRepositorySelectsMode(t *testing.T) {
	t.Cleanup(viper.Reset)

	tests := []struct {
		name string
		mode string
		kind string
	}{
		{name: "default local", mode: "", kind: "*local.repository"},
		{name: "local", mode: string(Local), kind: "*local.repository"},
		{name: "aws", mode: string(AwsKMS), kind: "*awskms.repository"},
		{name: "azure", mode: string(AzureKeyVault), kind: "*azurekeyvault.repository"},
		{name: "gcp", mode: string(GpcKMS), kind: "*gcpkms.repository"},
		{name: "unknown fallback", mode: "nope", kind: "*local.repository"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("encrypt.vault.mode", test.mode)

			if got := typeName(NewRepository()); got != test.kind {
				t.Fatalf("NewRepository() type = %s, want %s", got, test.kind)
			}
		})
	}
}

func typeName(value any) string {
	return fmt.Sprintf("%T", value)
}
