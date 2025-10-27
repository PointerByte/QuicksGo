package telemetry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc/credentials"
)

const (
	gRPC         = "grpc"
	httpProtocol = "http/protobuf"
	httpJson     = "http/json"
)

type ShutdownOtel func(context.Context) error

var HandlerShutdownOtel ShutdownOtel = func(context.Context) error { return nil }

func initOtel(ctx context.Context) (shutdown ShutdownOtel, err error) {
	traceExporterName := strings.ToLower(viper.GetString("OTEL_TRACES_EXPORTER"))
	metricExporterName := strings.ToLower(viper.GetString("OTEL_METRICS_EXPORTER"))
	logsExporterName := strings.ToLower(viper.GetString("OTEL_LOGS_EXPORTER"))

	endpoint := viper.GetString("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" && (traceExporterName == "" || traceExporterName == "otlp" ||
		metricExporterName == "" || metricExporterName == "otlp" ||
		logsExporterName == "" || logsExporterName == "otlp") {
		return HandlerShutdownOtel, nil
	}

	serviceName := viper.GetString("service.name")
	serviceVersion := viper.GetString("service.version")

	resource, err := sdkresource.New(ctx,
		sdkresource.WithFromEnv(), // lee OTEL_RESOURCE_ATTRIBUTES
		sdkresource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return HandlerShutdownOtel, err
	}

	protocol := viper.GetString("OTEL_EXPORTER_OTLP_ENDPOINT")
	m := initMonitoring(ctx, endpoint, protocol, traceExporterName, metricExporterName, logsExporterName, resource)
	tp, err := m.Traces()
	if err != nil {
		return HandlerShutdownOtel, err
	}
	mp, err := m.Metrics()
	if err != nil {
		return HandlerShutdownOtel, err
	}
	lp, err := m.Logs()
	if err != nil {
		return HandlerShutdownOtel, err
	}

	// ---- SHUTDOWN FUNCTION ----
	shutdown = func(ctx context.Context) error {
		if tp != nil {
			_err := tp.Shutdown(ctx)
			if _err != nil {
				return _err
			}
		}
		if mp != nil {
			_err := mp.Shutdown(ctx)
			if _err != nil {
				return _err
			}
		}
		if lp != nil {
			_err := lp.Shutdown(ctx)
			if _err != nil {
				return _err
			}
		}
		return nil
	}

	return shutdown, nil
}

type monitoring interface {
	Traces() (*sdktrace.TracerProvider, error)
	Metrics() (*metric.MeterProvider, error)
	Logs() (*sdklog.LoggerProvider, error)
}

func initMonitoring(ctx context.Context,
	endpoint string,
	protocol string,
	traceExporterName string,
	metricExporterName string,
	logsExporterName string,
	resource *sdkresource.Resource) monitoring {
	m := &monitoringImp{
		ctx:                ctx,
		protocol:           protocol,
		endpoint:           endpoint,
		traceExporterName:  traceExporterName,
		metricExporterName: metricExporterName,
		logsExporterName:   logsExporterName,
		resource:           resource,
	}
	m.tlsConfig()
	return m
}

type monitoringImp struct {
	ctx                context.Context
	endpoint           string
	protocol           string
	traceExporterName  string
	metricExporterName string
	logsExporterName   string
	resource           *sdkresource.Resource
	grpcTraceTLS       otlptracegrpc.Option
	grpcMetricsTLS     otlpmetricgrpc.Option
	grpcLoggerTLS      otlploggrpc.Option

	httpTraceTLS   otlptracehttp.Option
	httpMetricsTLS otlpmetrichttp.Option
	httpLoggerTLS  otlploghttp.Option
}

func (m *monitoringImp) tlsConfig() error {
	vp := viper.GetViper()
	if vp.GetBool("otlp.tls.insegure") {
		if m.traceExporterName == "otlp" {
			m.grpcTraceTLS = otlptracegrpc.WithInsecure()
		}
		if m.metricExporterName == "otlp" {
			m.grpcMetricsTLS = otlpmetricgrpc.WithInsecure()
		}
		if m.logsExporterName == "otlp" {
			m.grpcLoggerTLS = otlploggrpc.WithInsecure()
		}
		return nil
	}

	caPath := vp.GetString("otlp.tls.caPath")

	// 1) Pool of CAs
	var rootCAs *x509.CertPool
	if caPath != "" {
		pem, err := os.ReadFile(caPath)
		if err != nil {
			return fmt.Errorf("read CA %q: %w", caPath, err)
		}
		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(pem) {
			return fmt.Errorf("couldn't parse the CA PEM in %q", caPath)
		}
	}

	// 2) mTLS (opcional)
	var certs []tls.Certificate
	if vp.GetBool("otlp.tls.mTLS.enable") {
		clientCertPath := vp.GetString("otlp.tls.mTLS.clientCertPath")
		clientKeyPath := vp.GetString("otlp.tls.mTLS.clientKeyPath")
		if clientCertPath != "" || clientKeyPath != "" {
			if clientCertPath == "" || clientKeyPath == "" {
				return errors.New("for mTLS you need client cert and key")
			}
			cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
			if err != nil {
				return fmt.Errorf("loading for client (%q,%q): %w", clientCertPath, clientKeyPath, err)
			}
			certs = []tls.Certificate{cert}
		}
	}

	tlsCfg := &tls.Config{
		RootCAs:      rootCAs,
		Certificates: certs,
	}
	if m.traceExporterName == "otlp" {
		switch m.protocol {
		case gRPC:
			m.grpcTraceTLS = otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsCfg))
		case httpProtocol, httpJson:
			m.httpTraceTLS = otlptracehttp.WithTLSClientConfig(tlsCfg)
		default:
			return fmt.Errorf("unsupported OTLP protocol for traces: %s", m.protocol)
		}

	}
	if m.metricExporterName == "otlp" {
		switch m.protocol {
		case gRPC:
			m.grpcMetricsTLS = otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(tlsCfg))
		case httpProtocol, httpJson:
			m.httpMetricsTLS = otlpmetrichttp.WithTLSClientConfig(tlsCfg)
		default:
			return fmt.Errorf("unsupported OTLP protocol for metrics: %s", m.protocol)
		}
	}
	if m.logsExporterName == "otlp" {
		switch m.protocol {
		case gRPC:
			m.grpcLoggerTLS = otlploggrpc.WithTLSCredentials(credentials.NewTLS(tlsCfg))
		case httpProtocol, httpJson:
			m.httpLoggerTLS = otlploghttp.WithTLSClientConfig(tlsCfg)
		default:
			return fmt.Errorf("unsupported OTLP protocol for logs: %s", m.protocol)
		}
	}
	return nil
}

// Traces implements monitoring.
func (m *monitoringImp) Traces() (*sdktrace.TracerProvider, error) {
	var tp *sdktrace.TracerProvider
	var err error

	// ---- TRACES ----
	if m.traceExporterName != "none" {
		var traceExp sdktrace.SpanExporter
		switch m.traceExporterName {
		case "otlp":
			switch m.protocol {
			case gRPC:
				traceExp, err = otlptracegrpc.New(m.ctx,
					otlptracegrpc.WithEndpoint(m.endpoint),
					m.grpcTraceTLS,
					otlptracegrpc.WithCompressor("gzip"),
				)
			case httpProtocol, httpJson:
				traceExp, err = otlptracehttp.New(m.ctx,
					otlptracehttp.WithEndpoint(m.endpoint),
					m.httpTraceTLS,
					otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
				)
			default:
				return nil, fmt.Errorf("unsupported OTLP protocol for traces: %s", m.protocol)
			}
		case "console":
			traceExp, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		default:
			return nil, errors.New("otel_traces_exporter invalid")
		}

		if err != nil {
			return nil, err
		}

		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExp),
			sdktrace.WithResource(m.resource),
		)
		otel.SetTracerProvider(tp)
	}
	return tp, nil
}

// Metrics implements monitoring.
func (m *monitoringImp) Metrics() (*metric.MeterProvider, error) {
	var mp *metric.MeterProvider
	var err error

	// ---- METRICS ----
	if m.metricExporterName != "none" {
		var metricExp metric.Exporter
		switch m.metricExporterName {
		case "otlp":
			switch m.protocol {
			case gRPC:
				metricExp, err = otlpmetricgrpc.New(m.ctx,
					otlpmetricgrpc.WithEndpoint(m.endpoint),
					m.grpcMetricsTLS,
					otlpmetricgrpc.WithCompressor("gzip"),
				)
			case httpProtocol, httpJson:
				metricExp, err = otlpmetrichttp.New(m.ctx,
					otlpmetrichttp.WithEndpoint(m.endpoint),
					m.httpMetricsTLS,
					otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression),
				)
			default:
				return nil, fmt.Errorf("unsupported OTLP protocol for traces: %s", m.protocol)
			}
		case "console":
			metricExp, err = stdoutmetric.New(stdoutmetric.WithPrettyPrint())
		default:
			return nil, errors.New("otel_metrics_exporter invalid")
		}

		if err != nil {
			return nil, err
		}

		mp = metric.NewMeterProvider(
			metric.WithReader(metric.NewPeriodicReader(metricExp, metric.WithInterval(5*time.Second))),
			metric.WithResource(m.resource),
		)
		otel.SetMeterProvider(mp)
	}
	return mp, nil
}

// Logs implements monitoring.
func (m *monitoringImp) Logs() (*sdklog.LoggerProvider, error) {
	var lp *sdklog.LoggerProvider
	var err error

	if m.logsExporterName != "none" {
		var logExp sdklog.Exporter
		switch m.logsExporterName {
		case "otlp":
			switch m.protocol {
			case gRPC:
				logExp, err = otlploggrpc.New(m.ctx,
					otlploggrpc.WithEndpoint(m.endpoint),
					m.grpcLoggerTLS,
					otlploggrpc.WithCompressor("gzip"),
				)
			case httpProtocol, httpJson:
				logExp, err = otlploghttp.New(m.ctx,
					otlploghttp.WithEndpoint(m.endpoint),
					m.httpLoggerTLS,
					otlploghttp.WithCompression(otlploghttp.GzipCompression),
				)
			default:
				return nil, fmt.Errorf("unsupported OTLP protocol for traces: %s", m.protocol)
			}
		case "console":
			logExp, err = stdoutlog.New(stdoutlog.WithPrettyPrint())
		default:
			return nil, errors.New("otel_logs_exporter invalid")
		}
		if err != nil {
			return nil, err
		}
		if logExp != nil {
			lp = sdklog.NewLoggerProvider(
				sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
				sdklog.WithResource(m.resource),
			)
			SetLoggerProvider(lp)
		}
	}
	return lp, nil
}

func GetMiddleware() gin.HandlerFunc {
	return otelgin.Middleware(
		viper.GetString("service.name"),
		otelgin.WithTracerProvider(otel.GetTracerProvider()),
		otelgin.WithMeterProvider(otel.GetMeterProvider()),
		otelgin.WithPropagators(otel.GetTextMapPropagator()),
	)
}
