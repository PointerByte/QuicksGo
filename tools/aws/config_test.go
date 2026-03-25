package aws

import (
	"context"
	"errors"
	"testing"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

func TestLoadAWSConfigAttachesOpenTelemetryMiddlewares(t *testing.T) {
	prevLoad := loadDefaultAWSConfigFn
	prevAppend := appendOTelMiddlewaresFn
	t.Cleanup(func() {
		loadDefaultAWSConfigFn = prevLoad
		appendOTelMiddlewaresFn = prevAppend
	})

	appendCalled := false
	loadDefaultAWSConfigFn = func(context.Context, ...func(*awsconfig.LoadOptions) error) (sdkaws.Config, error) {
		return sdkaws.Config{}, nil
	}
	appendOTelMiddlewaresFn = func(apiOptions *[]func(*middleware.Stack) error, _ ...otelaws.Option) {
		appendCalled = true
	}

	if _, err := LoadAWSConfig(context.Background()); err != nil {
		t.Fatalf("LoadAWSConfig() error = %v", err)
	}
	if !appendCalled {
		t.Fatal("expected OpenTelemetry middlewares to be attached")
	}
}

func TestLoadAWSConfigReturnsLoadError(t *testing.T) {
	prevLoad := loadDefaultAWSConfigFn
	prevAppend := appendOTelMiddlewaresFn
	t.Cleanup(func() {
		loadDefaultAWSConfigFn = prevLoad
		appendOTelMiddlewaresFn = prevAppend
	})

	loadDefaultAWSConfigFn = func(context.Context, ...func(*awsconfig.LoadOptions) error) (sdkaws.Config, error) {
		return sdkaws.Config{}, errors.New("config boom")
	}
	appendOTelMiddlewaresFn = func(apiOptions *[]func(*middleware.Stack) error, _ ...otelaws.Option) {
		t.Fatal("did not expect middleware attachment on config error")
	}

	if _, err := LoadAWSConfig(context.Background()); err == nil {
		t.Fatal("expected LoadAWSConfig to return an error")
	}
}
