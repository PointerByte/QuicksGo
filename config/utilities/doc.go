// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package utilities provides shared configuration-loading helpers for the
// config module.
//
// Its main responsibility is to load application settings from
// application.yml or application.json, merge .env and .env.local files, and
// apply environment-variable overrides derived from the Viper key paths.
//
// Main entry point:
//   - LoadEnv to load configuration files and apply override rules
package utilities
