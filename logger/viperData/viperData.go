// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package viperdata

import (
	"sync"

	"github.com/spf13/viper"
)

var (
	viperData map[string]any
	once      sync.Once
)

const layout = "2006-01-02T15:04:05.000"

func ResetViperDataSingleton() {
	viperData = nil
	once = sync.Once{}
}

var mux sync.Mutex

// GetViperData retrieves the value associated with the given key from the viperData map.
// It initializes the viperData map on the first call using sync.Once to ensure thread safety.
// The function also sets a default value for LoggerFormatDateAtribute if it is not already set in Viper.
func GetViperData(key string) any {
	if viper.GetString(string(LoggerFormatDateAtribute)) == "" {
		viper.Set(string(LoggerFormatDateAtribute), layout)
	}

	once.Do(func() {
		viperData = map[string]any{
			string(AppVersionAtribute):                         viper.GetString(string(AppVersionAtribute)),
			string(AppAtribute):                                viper.GetString(string(AppAtribute)),
			string(GinLoggerWithConfigEnabledAtribute):         viper.GetBool(string(GinLoggerWithConfigEnabledAtribute)),
			string(GinLoggerWithConfigSkipPathsAtribute):       viper.GetStringSlice(string(GinLoggerWithConfigSkipPathsAtribute)),
			string(GinLoggerWithConfigSkipQueryStringAtribute): viper.GetBool(string(GinLoggerWithConfigSkipQueryStringAtribute)),
			string(LoggerModeTestAtribute):                     viper.GetBool(string(LoggerModeTestAtribute)),
			string(LoggerLevelAtribute):                        viper.GetString(string(LoggerLevelAtribute)),
			string(LoggerIgnoredHeadersAtribute):               viper.GetStringSlice(string(LoggerIgnoredHeadersAtribute)),
			string(LoggerFormatterAtribute):                    viper.GetString(string(LoggerFormatterAtribute)),
			string(LoggerFormatDateAtribute):                   viper.GetString(string(LoggerFormatDateAtribute)),
			string(LoggerRotateEnableAtribute):                 viper.GetBool(string(LoggerRotateEnableAtribute)),
			string(LoggerRotateMaxSizeAtribute):                viper.GetInt(string(LoggerRotateMaxSizeAtribute)),
			string(LoggerRotateMaxBackupsAtribute):             viper.GetInt(string(LoggerRotateMaxBackupsAtribute)),
			string(LoggerRotateMaxAgeAtribute):                 viper.GetInt(string(LoggerRotateMaxAgeAtribute)),
			string(LoggerCompressMaxAgeAtribute):               viper.GetBool(string(LoggerCompressMaxAgeAtribute)),
		}
	})

	mux.Lock()
	defer mux.Unlock()
	return viperData[key]
}
