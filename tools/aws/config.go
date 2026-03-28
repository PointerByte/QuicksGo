// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

var (
	loadDefaultAWSConfigFn  = config.LoadDefaultConfig
	appendOTelMiddlewaresFn = otelaws.AppendMiddlewares
)

// LoadAWSConfig loads the default AWS SDK configuration for the current
// environment and attaches OpenTelemetry middlewares to instrument AWS API
// calls.
//
// It is intended to be reused by packages that need a shared, trace-enabled
// AWS client configuration, and it can also be replaced in tests to avoid
// loading real cloud credentials.
func LoadAWSConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := loadDefaultAWSConfigFn(ctx)
	if err != nil {
		return cfg, err
	}
	appendOTelMiddlewaresFn(&cfg.APIOptions)
	return cfg, nil
}
