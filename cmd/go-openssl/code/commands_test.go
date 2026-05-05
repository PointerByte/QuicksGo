// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package code

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAppAndExecute(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Fatal("expected app")
	}
	if app.streams.In == nil || app.streams.Out == nil || app.streams.Err == nil {
		t.Fatal("expected streams to be initialized")
	}
	if app.generator == nil {
		t.Fatal("expected generator to be initialized")
	}

	root := app.rootCommand()
	if root.Use != "go-openssl" {
		t.Fatalf("expected root use go-openssl, got %q", root.Use)
	}
}

func TestExecuteRunsRootCommandHelp(t *testing.T) {
	app := &App{
		streams: IOStreams{
			In:  strings.NewReader(""),
			Out: &bytes.Buffer{},
			Err: &bytes.Buffer{},
		},
		generator: NewGenerator(),
	}

	root := app.rootCommand()
	root.SetArgs([]string{"help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("expected help execution without error, got %v", err)
	}
}

func TestAppExecute(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() {
		os.Args = originalArgs
	})

	app := &App{
		streams: IOStreams{
			In:  strings.NewReader(""),
			Out: &bytes.Buffer{},
			Err: &bytes.Buffer{},
		},
		generator: NewGenerator(),
	}

	os.Args = []string{"go-openssl", "help"}
	if err := app.Execute(); err != nil {
		t.Fatalf("expected app.Execute without error, got %v", err)
	}
}

func TestGenerateCommandCobra(t *testing.T) {
	app := &App{
		streams:   IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}},
		generator: NewGenerator(),
	}

	generateCmd := newGenerateCommand(app).Cobra()
	if generateCmd.Use != "generate" {
		t.Fatalf("expected use generate, got %q", generateCmd.Use)
	}
	if generateCmd.Flag("algorithm") == nil || generateCmd.Flag("dir") == nil || generateCmd.Flag("salt") == nil ||
		generateCmd.Flag("signed-by") == nil || generateCmd.Flag("ca-key") == nil {
		t.Fatal("expected certificate flags to be registered")
	}
}

func TestGenerateCommandRunE(t *testing.T) {
	output := &bytes.Buffer{}
	app := &App{
		streams: IOStreams{
			In:  strings.NewReader(""),
			Out: output,
			Err: &bytes.Buffer{},
		},
		generator: NewGenerator(),
	}

	cmd := app.rootCommand()
	dir := filepath.Join(t.TempDir(), "certs")
	cmd.SetArgs([]string{
		"generate",
		"--algorithm", algorithmECC,
		"--ecc-curve", curveP256,
		"--common-name", "localhost",
		"--dir", dir,
		"--salt", "pepper",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected command execution without error, got %v", err)
	}
	if !strings.Contains(output.String(), "Certificate generated in") {
		t.Fatalf("expected success output, got %q", output.String())
	}
}
