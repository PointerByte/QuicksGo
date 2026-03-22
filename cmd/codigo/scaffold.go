package codigo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	configYAML = "yaml"
	configJSON = "json"
)

var (
	modulePathPattern  = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

type scaffoldOptions struct {
	modulePath   string
	appName      string
	configFormat string
	outputDir    string
}

type scaffolder struct {
	streams     IOStreams
	runner      goRunner
	absFn       func(string) (string, error)
	statFn      func(string) (fs.FileInfo, error)
	mkdirAllFn  func(string, os.FileMode) error
	writeFileFn func(string, []byte, os.FileMode) error
}

func newScaffolder(streams IOStreams, runner goRunner) *scaffolder {
	return &scaffolder{
		streams:     streams,
		runner:      runner,
		absFn:       filepath.Abs,
		statFn:      os.Stat,
		mkdirAllFn:  os.MkdirAll,
		writeFileFn: os.WriteFile,
	}
}

func resolveScaffoldOptions(streams IOStreams, options *scaffoldOptions) (scaffoldOptions, error) {
	reader := bufio.NewReader(streams.In)

	modulePath, err := promptRequired(reader, streams.Out, "Package/module name", options.modulePath)
	if err != nil {
		return scaffoldOptions{}, err
	}
	if !isValidModulePath(modulePath) {
		return scaffoldOptions{}, fmt.Errorf("package/module name contains invalid characters")
	}

	appName, err := promptRequired(reader, streams.Out, "app.name", options.appName)
	if err != nil {
		return scaffoldOptions{}, err
	}
	if !isValidServiceName(appName) {
		return scaffoldOptions{}, fmt.Errorf("app.name contains invalid characters")
	}

	configFormat, err := promptConfigFormat(reader, streams.Out, options.configFormat)
	if err != nil {
		return scaffoldOptions{}, err
	}

	outputDir := strings.TrimSpace(options.outputDir)
	if outputDir == "" {
		outputDir = appName
	}

	return scaffoldOptions{
		modulePath:   modulePath,
		appName:      appName,
		configFormat: configFormat,
		outputDir:    outputDir,
	}, nil
}

func promptRequired(reader *bufio.Reader, output io.Writer, label string, fallback string) (string, error) {
	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback, nil
	}

	if _, err := fmt.Fprintf(output, "%s: ", label); err != nil {
		return "", err
	}

	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}

func promptConfigFormat(reader *bufio.Reader, output io.Writer, fallback string) (string, error) {
	fallback = strings.ToLower(strings.TrimSpace(fallback))
	if fallback != "" {
		if !isValidConfigFormat(fallback) {
			return "", fmt.Errorf("invalid config format %q", fallback)
		}
		return fallback, nil
	}

	if _, err := fmt.Fprint(output, "Config format [yaml/json] (default: yaml): "); err != nil {
		return "", err
	}

	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return configYAML, nil
	}
	if !isValidConfigFormat(value) {
		return "", fmt.Errorf("invalid config format %q", value)
	}
	return value, nil
}

func isValidConfigFormat(value string) bool {
	return value == configYAML || value == configJSON
}

func isValidModulePath(value string) bool {
	return modulePathPattern.MatchString(value) && !strings.Contains(value, " ")
}

func isValidServiceName(value string) bool {
	return serviceNamePattern.MatchString(value) && !strings.Contains(value, " ")
}

func (scaffolder *scaffolder) createService(serviceType string, options scaffoldOptions) error {
	outputDir, err := scaffolder.absFn(options.outputDir)
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}

	if _, err := scaffolder.statFn(outputDir); err == nil {
		return fmt.Errorf("output directory already exists: %s", outputDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output directory: %w", err)
	}

	if err := scaffolder.mkdirAllFn(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	files, err := buildProjectFiles(serviceType, options)
	if err != nil {
		return err
	}

	for name, content := range files {
		target := filepath.Join(outputDir, name)
		if err := scaffolder.writeFileFn(target, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	if err := scaffolder.runner(outputDir, "mod", "init", options.modulePath); err != nil {
		return fmt.Errorf("initialize go module: %w", err)
	}
	if err := scaffolder.runner(outputDir, "mod", "tidy"); err != nil {
		return fmt.Errorf("install dependencies: %w", err)
	}

	if _, err := fmt.Fprintf(scaffolder.streams.Out, "Service created in %s\n", outputDir); err != nil {
		return err
	}
	return nil
}
