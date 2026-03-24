// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -source=config.go -destination=./mocksConfig.go -package=server_gRPC

package server_gRPC

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/PointerByte/QuicksGo/config/utilities"
	"github.com/PointerByte/QuicksGo/config/utilities/traces"
	"github.com/PointerByte/QuicksGo/logger/builder"
	loggerMiddlewares "github.com/PointerByte/QuicksGo/logger/middlewares"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var listenTCP = net.Listen
var loadX509KeyPairFn = tls.LoadX509KeyPair
var readFileFn = os.ReadFile
var newCertPoolFn = x509.NewCertPool
var quit chan os.Signal
var logServerInfoFn = func(message string) {
	builder.New(context.Background()).Info(message)
}
var logServerErrorFn = func(err error) {
	builder.New(context.Background()).Error(err)
}
var loadEnv = utilities.LoadEnv
var runAsyncFn = func(fn func()) {
	go fn()
}
var waitForShutdownSignalFn = waitForShutdownSignal

func init() {
	quit = make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
}

// RegisterServiceFunc allows registering any generated gRPC service from the proto package.
// Example:
//
//	srv.Register(func(r grpc.ServiceRegistrar) {
//	  proto.RegisterGreeterServer(r, handler)
//	})
type RegisterServiceFunc func(grpc.ServiceRegistrar)

// IConfig defines the basic operations required to configure and run a unary gRPC server.
//
// The implementation is transport-oriented and does not depend on a specific proto service.
// Generated registrations from the proto package can be injected through Register.
type IConfig interface {
	// SetAddress defines the TCP address that will be used when Serve needs to create its own listener.
	SetAddress(address string)

	// SetListener defines an external listener. If present, Serve uses it instead of creating a new one.
	SetListener(listener net.Listener)

	// Register injects one generated registration function against the underlying grpc.Server.
	Register(register RegisterServiceFunc) error

	// Serve starts the gRPC server using the configured listener or address.
	Serve() error

	// GracefulStop stops the server gracefully.
	GracefulStop()

	// Stop stops the server immediately.
	Stop()

	// GetServer returns the underlying grpc.Server.
	GetServer() *grpc.Server

	// GetListener returns the currently configured listener.
	GetListener() net.Listener
}

type Config struct {
	mocks     IConfig
	server    *grpc.Server
	serverErr error
	listener  net.Listener
	address   string
	mux       sync.RWMutex
}

var tlsConfig *tls.Config

// SetTLSConfig sets the TLS configuration that NewIUnitary should attach to
// internally created gRPC servers.
//
// When a custom grpc.Server is injected into NewIUnitary, this configuration is
// ignored because the server has already been created by the caller.
func SetTLSConfig(config *tls.Config) {
	tlsConfig = config
}

// NewIConfig creates a new unary gRPC server wrapper.
//
// The server parameter lets callers inject an already constructed
// *grpc.Server when they need explicit control over server options before
// handing execution to this package.
//
// If server is nil, the function creates a default grpc.Server with the
// package interceptors for traces and logging.
//
// If server is not nil, that instance is used as-is and its existing
// configuration is preserved.
//
// If mocks is provided, all operations delegate to it, which is useful for
// tests generated with mockgen.
//
// Common usage:
//
//	srv := unitary.NewIConfig(nil, nil)
//	srv.SetAddress(":50051")
//	err := srv.Register(func(r grpc.ServiceRegistrar) {
//		proto.RegisterGreeterServer(r, handler)
//	})
//	if err != nil {
//		return err
//	}
//	return srv.Serve()
//
// Example with a custom gRPC server:
//
//	custom := grpc.NewServer()
//	srv := unitary.NewIConfig(nil, custom)
func NewIConfig(mocks IConfig, server *grpc.Server) IConfig {
	return &Config{
		mocks:  mocks,
		server: server,
	}
}

func (su *Config) SetAddress(address string) {
	su.mux.Lock()
	defer su.mux.Unlock()
	su.address = address
}

func (su *Config) SetListener(listener net.Listener) {
	su.mux.Lock()
	defer su.mux.Unlock()
	su.listener = listener
}

func (su *Config) Register(register RegisterServiceFunc) error {
	if su.mocks != nil {
		return su.mocks.Register(register)
	}
	if err := su.ensureServer(); err != nil {
		return err
	}
	if register == nil {
		return fmt.Errorf("register function is required")
	}
	register(su.server)
	return nil
}

func (su *Config) Serve() error {
	if su.mocks != nil {
		return su.mocks.Serve()
	}
	if err := loadEnv("."); err != nil {
		return err
	}
	if err := su.ensureServer(); err != nil {
		return err
	}

	su.mux.Lock()
	if su.listener == nil {
		if su.address == "" {
			su.address = strings.TrimSpace(viper.GetString("server.grpc.port"))
		}
		if su.address == "" {
			su.mux.Unlock()
			return fmt.Errorf("address or listener is required")
		}

		listener, err := listenTCP("tcp", su.address)
		if err != nil {
			su.mux.Unlock()
			return fmt.Errorf("problem creating tcp listener: %w", err)
		}
		su.listener = listener
	}
	listener := su.listener
	server := su.server
	su.mux.Unlock()

	address := listener.Addr().String()
	logServerInfoFn(fmt.Sprintf("gRPC server started on %s", address))
	runAsyncFn(func() {
		waitForShutdownSignalFn(server)
	})

	err := server.Serve(listener)
	if err != nil {
		logServerErrorFn(fmt.Errorf("gRPC server stopped with error: %w", err))
		return err
	}

	logServerInfoFn("gRPC server stopped successfully")
	return nil
}

func waitForShutdownSignal(server *grpc.Server) {
	<-quit
	logServerInfoFn("Signal received, turning off gRPC server...")
	server.GracefulStop()
}

func (su *Config) GracefulStop() {
	if su.mocks != nil {
		su.mocks.GracefulStop()
		return
	}
	su.server.GracefulStop()
}

func (su *Config) Stop() {
	if su.mocks != nil {
		su.mocks.Stop()
		return
	}
	su.server.Stop()
}

func (su *Config) GetServer() *grpc.Server {
	if su.mocks != nil {
		return su.mocks.GetServer()
	}
	_ = su.ensureServer()
	return su.server
}

func (su *Config) GetListener() net.Listener {
	if su.mocks != nil {
		return su.mocks.GetListener()
	}

	su.mux.RLock()
	defer su.mux.RUnlock()
	return su.listener
}

func (su *Config) ensureServer() error {
	su.mux.Lock()
	defer su.mux.Unlock()
	return su.ensureServerLocked()
}

func (su *Config) ensureServerLocked() error {
	if su.server != nil || su.serverErr != nil {
		return su.serverErr
	}

	options, err := defaultServerOptions(su)
	if err != nil {
		su.serverErr = err
		return err
	}
	su.server = grpc.NewServer(options...)
	return nil
}

func defaultServerOptions(cfg *Config) ([]grpc.ServerOption, error) {
	options := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			traces.MiddlewareOtelGRPCUnary(),
			loggerMiddlewares.InitLoggerUnaryServerInterceptor(),
			loggerMiddlewares.LoggerWithConfigUnaryServerInterceptor(),
			loggerMiddlewares.CaptureBodyUnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			traces.MiddlewareOtelGRPCStream(),
			loggerMiddlewares.InitLoggerStreamServerInterceptor(),
			loggerMiddlewares.LoggerWithConfigStreamServerInterceptor(),
			loggerMiddlewares.CaptureBodyStreamServerInterceptor(),
		),
		grpc.UnknownServiceHandler(unknownServiceHandler(cfg)),
	}

	config, err := resolveTLSConfig()
	if err != nil {
		return nil, err
	}
	if config != nil {
		options = append(options, grpc.Creds(credentials.NewTLS(config)))
	}
	return options, nil
}

func resolveTLSConfig() (*tls.Config, error) {
	if tlsConfig != nil {
		return tlsConfig, nil
	}

	tlsEnabled := viper.GetBool("server.grpc.tls.enable")
	mtlsEnabled := viper.GetBool("server.grpc.mtls.enable")
	if !tlsEnabled && !mtlsEnabled {
		return nil, nil
	}

	certFile := strings.TrimSpace(viper.GetString("server.grpc.tls.certFile"))
	keyFile := strings.TrimSpace(viper.GetString("server.grpc.tls.keyFile"))
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("server.grpc.tls.certFile and server.grpc.tls.keyFile are required")
	}

	certificate, err := loadX509KeyPairFn(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("problem loading server tls certificate: %w", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   parseTLSVersion(viper.GetString("server.grpc.tls.version")),
	}

	if mtlsEnabled {
		clientCAFile := strings.TrimSpace(viper.GetString("server.grpc.mtls.clientCAFile"))
		if clientCAFile == "" {
			return nil, fmt.Errorf("server.grpc.mtls.clientCAFile is required")
		}

		caPEM, err := readFileFn(clientCAFile)
		if err != nil {
			return nil, fmt.Errorf("problem reading client ca file: %w", err)
		}

		pool := newCertPoolFn()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("problem parsing client ca file")
		}
		config.ClientCAs = pool
		config.ClientAuth = parseClientAuth(viper.GetString("server.grpc.mtls.clientAuth"))
	}

	return config, nil
}

func parseTLSVersion(raw string) uint16 {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "tlsv10":
		return tls.VersionTLS10
	case "tlsv11":
		return tls.VersionTLS11
	case "tlsv13":
		return tls.VersionTLS13
	default:
		return tls.VersionTLS12
	}
}

func parseClientAuth(raw string) tls.ClientAuthType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "requestclientcert", "request_client_cert":
		return tls.RequestClientCert
	case "requireanyclientcert", "require_any_client_cert":
		return tls.RequireAnyClientCert
	case "verifyclientcertifgiven", "verify_client_cert_if_given":
		return tls.VerifyClientCertIfGiven
	case "noclientcert", "no_client_cert":
		return tls.NoClientCert
	default:
		return tls.RequireAndVerifyClientCert
	}
}
