// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package viperdata

import (
	"testing"

	"github.com/spf13/viper"
)

func TestGetViperData(t *testing.T) {
	ResetViperDataSingleton()

	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set(string(AppAtribute), "test-app")
	viper.Set(string(LoggerModeTestAtribute), true)

	got := GetViperData(string(AppAtribute))
	if got != "test-app" {
		t.Errorf("app.name = %v, want %v", got, "test-app")
	}

	got = GetViperData(string(LoggerModeTestAtribute))
	if got != true {
		t.Errorf("logger.modeTest = %v, want %v", got, "true")
	}
}
