// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package middlewares

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/PointerByte/QuicksGo/logger/formatter"
	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type nilPointerError struct{}

func (*nilPointerError) Error() string {
	return "nil-pointer-error"
}

type fakeServerStream struct {
	grpc.ServerStream
	ctx       context.Context
	recvItems []any
	sendItems []any
	sendErr   error
}

func (s *fakeServerStream) Context() context.Context {
	return s.ctx
}

func (s *fakeServerStream) RecvMsg(m any) error {
	if len(s.recvItems) == 0 {
		return io.EOF
	}
	item := s.recvItems[0]
	s.recvItems = s.recvItems[1:]
	switch dst := m.(type) {
	case *string:
		*dst = item.(string)
	}
	return nil
}

func (s *fakeServerStream) SendMsg(m any) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sendItems = append(s.sendItems, m)
	return nil
}

func resetGRPCTestState(t *testing.T) {
	t.Helper()
	viper.Reset()
	viperdata.ResetViperDataSingleton()
	t.Cleanup(func() {
		viper.Reset()
		viperdata.ResetViperDataSingleton()
	})

	viper.Set(string(viperdata.AppAtribute), "test-service")
	viper.Set(string(viperdata.LoggerIgnoredHeadersAtribute), []string{})
	viper.Set(string(viperdata.LoggerModeTestAtribute), false)
}

func TestInitLoggerUnaryServerInterceptor(t *testing.T) {
	resetGRPCTestState(t)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "abc123"))
	ctx = peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}})

	var gotCtxLogger *builder.Context
	interceptor := InitLoggerUnaryServerInterceptor()
	_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/pkg.Greeter/SayHello"}, func(ctx context.Context, req any) (any, error) {
		gotCtxLogger = builder.New(ctx)
		return "response", nil
	})
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}

	detailsAny, ok := gotCtxLogger.Get(detailsKey)
	if !ok {
		t.Fatalf("expected %q in logger context", detailsKey)
	}
	details := detailsAny.(formatter.KibanaData)
	if details.Protocol != "gRPC" {
		t.Fatalf("details.Protocol = %q, want %q", details.Protocol, "gRPC")
	}
	if details.Method != "SayHello" {
		t.Fatalf("details.Method = %q, want %q", details.Method, "SayHello")
	}
	if details.Path != "/pkg.Greeter/SayHello" {
		t.Fatalf("details.Path = %q, want %q", details.Path, "/pkg.Greeter/SayHello")
	}
	if got := details.Headers.Get("x-request-id"); got != "abc123" {
		t.Fatalf("details.Headers[x-request-id] = %q, want %q", got, "abc123")
	}
}

func TestUnaryInterceptorsCaptureBodiesAndPopulateDetails(t *testing.T) {
	resetGRPCTestState(t)

	var gotCtxLogger *builder.Context
	resp, err := InitLoggerUnaryServerInterceptor()(context.Background(), map[string]any{"kind": "info"}, &grpc.UnaryServerInfo{
		FullMethod: "/pkg.Greeter/SayHello",
	}, func(ctx context.Context, req any) (any, error) {
		return LoggerWithConfigUnaryServerInterceptor()(ctx, req, &grpc.UnaryServerInfo{
			FullMethod: "/pkg.Greeter/SayHello",
		}, func(ctx context.Context, req any) (any, error) {
			return CaptureBodyUnaryServerInterceptor()(ctx, req, &grpc.UnaryServerInfo{
				FullMethod: "/pkg.Greeter/SayHello",
			}, func(ctx context.Context, req any) (any, error) {
				gotCtxLogger = builder.New(ctx)
				gotCtxLogger.Set(disableBodyKey, false)
				gotCtxLogger.Set(formatter.InfoLevel, "request processed")
				return map[string]any{"message": "ok"}, nil
			})
		})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}

	detailsAny, ok := gotCtxLogger.Get(detailsKey)
	if !ok {
		t.Fatalf("expected %q in logger context", detailsKey)
	}
	details := detailsAny.(formatter.KibanaData)
	requestBody, ok := details.Request.(map[string]any)
	if !ok || requestBody["kind"] != "info" {
		t.Fatalf("details.Request = %#v, want captured request map", details.Request)
	}
	responseBody, ok := details.Response.(map[string]any)
	if !ok || responseBody["message"] != "ok" {
		t.Fatalf("details.Response = %#v, want captured response map", details.Response)
	}
}

func TestLoggerWithConfigUnaryOmitsBodiesWhenDisabled(t *testing.T) {
	resetGRPCTestState(t)

	var gotCtxLogger *builder.Context
	_, err := InitLoggerUnaryServerInterceptor()(context.Background(), "request", &grpc.UnaryServerInfo{
		FullMethod: "/pkg.Greeter/SayHello",
	}, func(ctx context.Context, req any) (any, error) {
		return LoggerWithConfigUnaryServerInterceptor()(ctx, req, &grpc.UnaryServerInfo{
			FullMethod: "/pkg.Greeter/SayHello",
		}, func(ctx context.Context, req any) (any, error) {
			return CaptureBodyUnaryServerInterceptor()(ctx, req, &grpc.UnaryServerInfo{
				FullMethod: "/pkg.Greeter/SayHello",
			}, func(ctx context.Context, req any) (any, error) {
				gotCtxLogger = builder.New(ctx)
				gotCtxLogger.Set(disableBodyKey, true)
				return "response", errors.New("boom")
			})
		})
	})
	if err == nil {
		t.Fatal("expected error")
	}

	detailsAny, ok := gotCtxLogger.Get(detailsKey)
	if !ok {
		t.Fatalf("expected %q in logger context", detailsKey)
	}
	details := detailsAny.(formatter.KibanaData)
	if details.Request != nil {
		t.Fatalf("details.Request = %#v, want nil", details.Request)
	}
	if details.Response != nil {
		t.Fatalf("details.Response = %#v, want nil", details.Response)
	}
}

func TestCaptureBodyStreamServerInterceptor(t *testing.T) {
	resetGRPCTestState(t)

	stream := &fakeServerStream{
		ctx:       context.Background(),
		recvItems: []any{"first", "second"},
	}
	var gotCtxLogger *builder.Context

	err := CaptureBodyStreamServerInterceptor()(nil, stream, &grpc.StreamServerInfo{
		FullMethod:     "/pkg.Greeter/StreamAlerts",
		IsServerStream: true,
		IsClientStream: true,
	}, func(srv any, stream grpc.ServerStream) error {
		gotCtxLogger = builder.New(stream.Context())
		var first string
		if err := stream.RecvMsg(&first); err != nil {
			return err
		}
		var second string
		if err := stream.RecvMsg(&second); err != nil {
			return err
		}
		if err := stream.SendMsg("out-1"); err != nil {
			return err
		}
		return stream.SendMsg("out-2")
	})
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}

	requestBody, _ := gotCtxLogger.Get(requestBodyKey)
	responseBody, _ := gotCtxLogger.Get(responseBodyKey)

	requests, ok := requestBody.([]any)
	if !ok || len(requests) != 2 {
		t.Fatalf("requestBody = %#v, want 2 captured messages", requestBody)
	}
	responses, ok := responseBody.([]any)
	if !ok || len(responses) != 2 {
		t.Fatalf("responseBody = %#v, want 2 captured messages", responseBody)
	}
}

func TestGRPCMetadataCarrier(t *testing.T) {
	carrier := grpcMetadataCarrier(metadata.MD{})

	if got := carrier.Get("missing"); got != "" {
		t.Fatalf("Get(missing) = %q, want empty string", got)
	}

	carrier.Set("X-Trace-Id", "abc")
	if got := carrier.Get("x-trace-id"); got != "abc" {
		t.Fatalf("Get(x-trace-id) = %q, want %q", got, "abc")
	}

	keys := carrier.Keys()
	if len(keys) != 1 || keys[0] != "x-trace-id" {
		t.Fatalf("Keys() = %#v, want %#v", keys, []string{"x-trace-id"})
	}
}

func TestGRPCCaptureStreamHelpers(t *testing.T) {
	baseCtx := context.WithValue(context.Background(), "k", "v")
	stream := &grpcCaptureStream{
		ServerStream: &fakeServerStream{
			ctx:       baseCtx,
			recvItems: []any{"hello"},
		},
		ctx: baseCtx,
	}

	if got := stream.Context(); got != baseCtx {
		t.Fatal("Context() did not return wrapped context")
	}

	var msg string
	if err := stream.RecvMsg(&msg); err != nil {
		t.Fatalf("RecvMsg() error = %v, want nil", err)
	}
	if msg != "hello" {
		t.Fatalf("RecvMsg() message = %q, want %q", msg, "hello")
	}
	if len(stream.requests) != 1 {
		t.Fatalf("requests len = %d, want 1", len(stream.requests))
	}

	if err := stream.SendMsg("world"); err != nil {
		t.Fatalf("SendMsg() error = %v, want nil", err)
	}
	if len(stream.responses) != 1 {
		t.Fatalf("responses len = %d, want 1", len(stream.responses))
	}
}

func TestGRPCCaptureStreamSendErrorDoesNotCaptureResponse(t *testing.T) {
	wantErr := errors.New("send failed")
	stream := &grpcCaptureStream{
		ServerStream: &fakeServerStream{
			ctx:     context.Background(),
			sendErr: wantErr,
		},
		ctx: context.Background(),
	}

	err := stream.SendMsg("world")
	if !errors.Is(err, wantErr) {
		t.Fatalf("SendMsg() error = %v, want %v", err, wantErr)
	}
	if len(stream.responses) != 0 {
		t.Fatalf("responses len = %d, want 0", len(stream.responses))
	}
}

func TestInitLoggerStreamServerInterceptor(t *testing.T) {
	resetGRPCTestState(t)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "stream-123"))
	stream := &fakeServerStream{ctx: ctx}
	var gotCtxLogger *builder.Context

	err := InitLoggerStreamServerInterceptor()(nil, stream, &grpc.StreamServerInfo{
		FullMethod:     "/pkg.Greeter/StreamAlerts",
		IsServerStream: true,
	}, func(srv any, stream grpc.ServerStream) error {
		gotCtxLogger = builder.New(stream.Context())
		return nil
	})
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}

	detailsAny, ok := gotCtxLogger.Get(detailsKey)
	if !ok {
		t.Fatalf("expected %q in logger context", detailsKey)
	}
	details := detailsAny.(formatter.KibanaData)
	if details.Method != "StreamAlerts" {
		t.Fatalf("details.Method = %q, want %q", details.Method, "StreamAlerts")
	}
	if got := details.Headers.Get("X-Request-Id"); got != "stream-123" {
		t.Fatalf("details.Headers[X-Request-Id] = %q, want %q", got, "stream-123")
	}
}

func TestLoggerWithConfigStreamServerInterceptor(t *testing.T) {
	resetGRPCTestState(t)

	stream := &fakeServerStream{ctx: context.Background()}
	var gotCtxLogger *builder.Context

	err := InitLoggerStreamServerInterceptor()(nil, stream, &grpc.StreamServerInfo{
		FullMethod:     "/pkg.Greeter/StreamAlerts",
		IsServerStream: true,
	}, func(srv any, stream grpc.ServerStream) error {
		return LoggerWithConfigStreamServerInterceptor()(srv, stream, &grpc.StreamServerInfo{
			FullMethod:     "/pkg.Greeter/StreamAlerts",
			IsServerStream: true,
		}, func(srv any, stream grpc.ServerStream) error {
			return CaptureBodyStreamServerInterceptor()(srv, stream, &grpc.StreamServerInfo{
				FullMethod:     "/pkg.Greeter/StreamAlerts",
				IsServerStream: true,
			}, func(srv any, stream grpc.ServerStream) error {
				gotCtxLogger = builder.New(stream.Context())
				gotCtxLogger.Set(disableBodyKey, false)
				gotCtxLogger.Set(formatter.InfoLevel, "stream processed")
				if err := stream.SendMsg("out"); err != nil {
					return err
				}
				return nil
			})
		})
	})
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}

	detailsAny, ok := gotCtxLogger.Get(detailsKey)
	if !ok {
		t.Fatalf("expected %q in logger context", detailsKey)
	}
	details := detailsAny.(formatter.KibanaData)
	if details.Response != "out" {
		t.Fatalf("details.Response = %#v, want %#v", details.Response, "out")
	}
}

func TestExtractGRPCContextWithoutMetadata(t *testing.T) {
	got := extractGRPCContext(context.Background())
	if got == nil {
		t.Fatal("extractGRPCContext() returned nil context")
	}
}

func TestNewGRPCLoggerContextWithoutPeerOrMetadata(t *testing.T) {
	resetGRPCTestState(t)

	ctx, span := newGRPCLoggerContext(context.Background(), context.Background(), "/pkg.Greeter/SayHello")
	defer span.End()

	ctxLogger := builder.New(ctx)
	detailsAny, ok := ctxLogger.Get(detailsKey)
	if !ok {
		t.Fatalf("expected %q in logger context", detailsKey)
	}
	details := detailsAny.(formatter.KibanaData)
	if details.Client != "" {
		t.Fatalf("details.Client = %q, want empty", details.Client)
	}
	if details.Headers != nil {
		t.Fatalf("details.Headers = %#v, want nil", details.Headers)
	}
}

func TestMetadataToHTTPHeader(t *testing.T) {
	if got := metadataToHTTPHeader(nil); got != nil {
		t.Fatalf("metadataToHTTPHeader(nil) = %#v, want nil", got)
	}

	got := metadataToHTTPHeader(metadata.Pairs("x-request-id", "abc", "content-type", "application/json"))
	want := http.Header{
		"X-Request-Id": {"abc"},
		"Content-Type": {"application/json"},
	}
	if got.Get("X-Request-Id") != want.Get("X-Request-Id") {
		t.Fatalf("X-Request-Id = %q, want %q", got.Get("X-Request-Id"), want.Get("X-Request-Id"))
	}
	if got.Get("Content-Type") != want.Get("Content-Type") {
		t.Fatalf("Content-Type = %q, want %q", got.Get("Content-Type"), want.Get("Content-Type"))
	}
}

func TestCollapseCapturedBodies(t *testing.T) {
	if got := collapseCapturedBodies(nil); got != nil {
		t.Fatalf("collapseCapturedBodies(nil) = %#v, want nil", got)
	}
	if got := collapseCapturedBodies([]any{"one"}); got != "one" {
		t.Fatalf("collapseCapturedBodies(single) = %#v, want %#v", got, "one")
	}
	got := collapseCapturedBodies([]any{"one", "two"})
	items, ok := got.([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("collapseCapturedBodies(many) = %#v, want 2 items", got)
	}
}

func TestApplyGRPCBodyDetailsGuards(t *testing.T) {
	resetGRPCTestState(t)

	t.Run("returns when disable flag has invalid type", func(t *testing.T) {
		ctxLogger := builder.New(context.Background())
		ctxLogger.Set(disableBodyKey, "bad")
		ctxLogger.Details = formatter.KibanaData{System: "svc"}
		ctxLogger.Set(requestBodyKey, "req")
		applyGRPCBodyDetails(ctxLogger)
		if ctxLogger.Details.Request != nil {
			t.Fatalf("Details.Request = %#v, want nil", ctxLogger.Details.Request)
		}
	})

	t.Run("returns when details key missing and details struct empty", func(t *testing.T) {
		ctxLogger := builder.New(context.Background())
		ctxLogger.Set(requestBodyKey, "req")
		applyGRPCBodyDetails(ctxLogger)
	})

	t.Run("returns when details key has wrong type", func(t *testing.T) {
		ctxLogger := builder.New(context.Background())
		ctxLogger.Set(detailsKey, "bad")
		ctxLogger.Set(requestBodyKey, "req")
		applyGRPCBodyDetails(ctxLogger)
	})
}

func TestWriteGRPCLogBranches(t *testing.T) {
	resetGRPCTestState(t)

	t.Run("keeps existing method and line", func(t *testing.T) {
		ctxLogger := builder.New(context.Background())
		ctxLogger.Method = "preset"
		ctxLogger.Line = 7
		ctxLogger.Set(formatter.InfoLevel, "info")
		writeGRPCLog(ctxLogger, "/pkg.Greeter/SayHello", nil)
		if ctxLogger.Method != "preset" || ctxLogger.Line != 7 {
			t.Fatalf("method/line changed unexpectedly: %q %d", ctxLogger.Method, ctxLogger.Line)
		}
	})

	t.Run("debug branch", func(t *testing.T) {
		ctxLogger := builder.New(context.Background())
		ctxLogger.Set(formatter.DebugLevel, "debug")
		writeGRPCLog(ctxLogger, "/pkg.Greeter/SayHello", nil)
		if ctxLogger.Method != "/pkg.Greeter/SayHello" || ctxLogger.Line != 1 {
			t.Fatalf("method/line = %q/%d, want %q/1", ctxLogger.Method, ctxLogger.Line, "/pkg.Greeter/SayHello")
		}
	})

	t.Run("warn branch", func(t *testing.T) {
		ctxLogger := builder.New(context.Background())
		ctxLogger.Set(formatter.WarnLevel, "warn")
		writeGRPCLog(ctxLogger, "/pkg.Greeter/SayHello", nil)
	})

	t.Run("error level wrong type falls back to handler error", func(t *testing.T) {
		ctxLogger := builder.New(context.Background())
		ctxLogger.Set(formatter.ErrorLevel, "not-an-error")
		writeGRPCLog(ctxLogger, "/pkg.Greeter/SayHello", errors.New("boom"))
	})

	t.Run("error branch", func(t *testing.T) {
		ctxLogger := builder.New(context.Background())
		ctxLogger.Set(formatter.ErrorLevel, errors.New("boom"))
		writeGRPCLog(ctxLogger, "/pkg.Greeter/SayHello", nil)
	})
}

func TestGRPCMethodName(t *testing.T) {
	if got := grpcMethodName("/pkg.Greeter/SayHello"); got != "SayHello" {
		t.Fatalf("grpcMethodName() = %q, want %q", got, "SayHello")
	}
	if got := grpcMethodName("SayHello"); got != "SayHello" {
		t.Fatalf("grpcMethodName() = %q, want %q", got, "SayHello")
	}
}

func TestHasError(t *testing.T) {
	if hasError(nil) {
		t.Fatal("hasError(nil) = true, want false")
	}

	var typedNilErr error = (*nilPointerError)(nil)
	if hasError(typedNilErr) {
		t.Fatal("hasError(typed nil) = true, want false")
	}

	if !hasError(errors.New("boom")) {
		t.Fatal("hasError(non-nil error) = false, want true")
	}
}
