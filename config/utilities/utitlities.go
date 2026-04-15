package utilities

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

var readInConfig = viper.ReadInConfig

func LoadEnv(prefixPath string) error {
	// Prefer application.yml and fall back to application.json.
	configFile := filepath.Join(prefixPath, "application.json")
	configType := "json"
	if _, err := os.Stat(filepath.Join(prefixPath, "application.yml")); err == nil {
		configFile = filepath.Join(prefixPath, "application.yml")
		configType = "yml"
	}

	viper.SetConfigFile(configFile)
	viper.SetConfigType(configType)
	if err := readInConfig(); err != nil {
		return err
	}

	// Load .env
	_ = godotenv.Overload(".env")
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	_ = viper.MergeInConfig()

	// Load .env.local
	_ = godotenv.Overload(".env.local")
	viper.SetConfigFile(".env.local")
	viper.SetConfigType("env")
	_ = viper.MergeInConfig()

	// Enable reading from environment variables.
	viper.AutomaticEnv()

	// Apply mapped environment variable overrides.
	exportMappedEnv()
	return nil
}

func exportMappedEnv() {
	applyEnvOverrides(viper.AllSettings(), nil)
}

func applyEnvOverrides(node any, path []string) {
	switch value := node.(type) {
	case map[string]any:
		for key, nested := range value {
			applyEnvOverrides(nested, append(path, key))
		}
	case map[any]any:
		for rawKey, nested := range value {
			key, ok := rawKey.(string)
			if !ok {
				continue
			}
			applyEnvOverrides(nested, append(path, key))
		}
	default:
		if len(path) == 0 {
			return
		}
		envName := envNameFromPath(path)
		rawValue, ok := os.LookupEnv(envName)
		if !ok {
			return
		}
		viper.Set(strings.Join(path, "."), parseEnvValue(rawValue, value))
	}
}

func envNameFromPath(path []string) string {
	parts := make([]string, 0, len(path))
	for _, part := range path {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts = append(parts, strings.ToUpper(part))
	}
	return strings.Join(parts, "_")
}

func parseEnvValue(raw string, template any) any {
	raw = strings.TrimSpace(raw)
	switch template := template.(type) {
	case bool:
		if parsed, err := strconv.ParseBool(raw); err == nil {
			return parsed
		}
	case int:
		if parsed, err := strconv.Atoi(raw); err == nil {
			return parsed
		}
	case int8, int16, int32, int64:
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return parsed
		}
	case uint, uint8, uint16, uint32, uint64:
		if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return parsed
		}
	case float32, float64:
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			return parsed
		}
	case []string:
		return parseStringSlice(raw)
	case []any:
		return parseSliceValue(raw, template)
	}
	return raw
}

func parseStringSlice(raw string) []string {
	if strings.HasPrefix(raw, "[") {
		var items []string
		if err := json.Unmarshal([]byte(raw), &items); err == nil {
			return items
		}
	}
	return splitCommaSeparated(raw)
}

func parseSliceValue(raw string, template []any) any {
	if strings.HasPrefix(raw, "[") {
		if allStringValues(template) {
			var items []string
			if err := json.Unmarshal([]byte(raw), &items); err == nil {
				return items
			}
		}

		var items []any
		if err := json.Unmarshal([]byte(raw), &items); err == nil {
			return items
		}
	}

	values := splitCommaSeparated(raw)
	if allStringValues(template) {
		return values
	}

	items := make([]any, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	return items
}

func allStringValues(values []any) bool {
	if len(values) == 0 {
		return true
	}

	for _, value := range values {
		if _, ok := value.(string); !ok {
			return false
		}
	}
	return true
}

func splitCommaSeparated(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
