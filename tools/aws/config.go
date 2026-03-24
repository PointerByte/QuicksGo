package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

var LoadAWSConfig = func(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return cfg, err
	}
	// Attach OpenTelemetry instrumentation
	otelaws.AppendMiddlewares(&cfg.APIOptions)
	return cfg, nil
}
