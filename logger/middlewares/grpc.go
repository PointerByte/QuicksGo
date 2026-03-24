// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package middlewares

import (
	"context"
	"net/http"
	"net/textproto"
	"reflect"
	"strings"

	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/PointerByte/QuicksGo/logger/formatter"
	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type grpcMetadataCarrier metadata.MD

func (c grpcMetadataCarrier) Get(key string) string {
	values := metadata.MD(c).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c grpcMetadataCarrier) Set(key string, value string) {
	key = strings.ToLower(key)
	md := metadata.MD(c)
	md[key] = append(md[key], value)
}

func (c grpcMetadataCarrier) Keys() []string {
	md := metadata.MD(c)
	keys := make([]string, 0, len(md))
	for key := range md {
		keys = append(keys, key)
	}
	return keys
}

type grpcCaptureStream struct {
	grpc.ServerStream
	ctx       context.Context
	requests  []any
	responses []any
}

func (s *grpcCaptureStream) Context() context.Context {
	return s.ctx
}

func (s *grpcCaptureStream) RecvMsg(m any) error {
	err := s.ServerStream.RecvMsg(m)
	if err == nil {
		s.requests = append(s.requests, m)
	}
	return err
}

func (s *grpcCaptureStream) SendMsg(m any) error {
	if err := s.ServerStream.SendMsg(m); err != nil {
		return err
	}
	s.responses = append(s.responses, m)
	return nil
}

// InitLoggerUnaryServerInterceptor creates the request-scoped logger context
// for unary gRPC calls.
//
// It extracts any incoming distributed-tracing headers from gRPC metadata,
// starts the logger span, attaches the base gRPC metadata used by structured
// logs, and passes the enriched context to the next handler.
func InitLoggerUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		parent := extractGRPCContext(ctx)
		ctxLogger, span := newGRPCLoggerContext(parent, ctx, info.FullMethod)
		defer span.End()
		return handler(ctxLogger, req)
	}
}

// InitLoggerStreamServerInterceptor creates the request-scoped logger context
// for streaming gRPC calls.
//
// It performs the same setup as InitLoggerUnaryServerInterceptor, but wraps the
// grpc.ServerStream so downstream handlers observe the enriched context through
// stream.Context().
func InitLoggerStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		parent := extractGRPCContext(stream.Context())
		ctxLogger, span := newGRPCLoggerContext(parent, stream.Context(), info.FullMethod)
		defer span.End()

		return handler(srv, &grpcContextStream{
			ServerStream: stream,
			ctx:          ctxLogger,
		})
	}
}

// CaptureBodyUnaryServerInterceptor stores the unary request and response
// payloads in the logger context.
//
// The captured values are later consumed by LoggerWithConfigUnaryServerInterceptor
// to populate details.request and details.response when body logging is enabled.
func CaptureBodyUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctxLogger := builder.New(ctx)
		ctxLogger.Set(requestBodyKey, req)
		resp, err := handler(ctxLogger, req)
		ctxLogger.Set(responseBodyKey, resp)
		return resp, err
	}
}

// CaptureBodyStreamServerInterceptor captures inbound and outbound stream
// messages and stores them in the logger context.
//
// When only one request or response message is observed, the stored value is
// that message directly. When multiple messages are exchanged, the stored value
// becomes a slice so the final log can reflect the full interaction.
func CaptureBodyStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctxLogger := builder.New(stream.Context())
		captureStream := &grpcCaptureStream{
			ServerStream: stream,
			ctx:          ctxLogger,
		}

		err := handler(srv, captureStream)
		ctxLogger.Set(requestBodyKey, collapseCapturedBodies(captureStream.requests))
		ctxLogger.Set(responseBodyKey, collapseCapturedBodies(captureStream.responses))
		return err
	}
}

// LoggerWithConfigUnaryServerInterceptor emits the final structured log entry
// for a unary gRPC call.
//
// It merges any captured request and response bodies into the logger details
// and then writes the log using the level previously stored in the logger
// context. If neither a log level nor a non-nil error is present, the function
// treats that condition as a developer error.
func LoggerWithConfigUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctxLogger := builder.New(ctx)
		resp, err := handler(ctxLogger, req)
		applyGRPCBodyDetails(ctxLogger)
		writeGRPCLog(ctxLogger, info.FullMethod, err)
		return resp, err
	}
}

// LoggerWithConfigStreamServerInterceptor emits the final structured log entry
// for a streaming gRPC call.
//
// It applies the same finalization flow as the unary variant, but operates on
// the stream context and writes the log once the stream handler completes.
func LoggerWithConfigStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctxLogger := builder.New(stream.Context())
		err := handler(srv, &grpcContextStream{
			ServerStream: stream,
			ctx:          ctxLogger,
		})
		applyGRPCBodyDetails(ctxLogger)
		writeGRPCLog(ctxLogger, info.FullMethod, err)
		return err
	}
}

type grpcContextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *grpcContextStream) Context() context.Context {
	return s.ctx
}

func extractGRPCContext(ctx context.Context) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)
	if md == nil {
		md = metadata.MD{}
	}
	return otel.GetTextMapPropagator().Extract(ctx, grpcMetadataCarrier(md.Copy()))
}

func newGRPCLoggerContext(parent context.Context, incoming context.Context, fullMethod string) (context.Context, trace.Span) {
	ctxLogger := builder.New(parent)
	appName := viperdata.GetViperData(string(viperdata.AppAtribute)).(string)
	tracer := otel.Tracer(appName)

	var span trace.Span
	ctxLogger.Context, span = tracer.Start(
		ctxLogger.Context,
		appName,
		trace.WithSpanKind(trace.SpanKindServer),
	)

	traceID := span.SpanContext().TraceID()
	if traceID.IsValid() {
		ctxLogger.Set(traceIDKey, traceID.String())
	}

	details := formatter.KibanaData{
		System:   appName,
		Protocol: "gRPC",
		Method:   grpcMethodName(fullMethod),
		Path:     fullMethod,
	}
	if p, ok := peer.FromContext(incoming); ok && p.Addr != nil {
		details.Client = p.Addr.String()
	}
	if md, ok := metadata.FromIncomingContext(incoming); ok {
		details.SetHeaders(metadataToHTTPHeader(md))
	}

	ctxLogger.Details = details
	ctxLogger.Set(detailsKey, details)
	return ctxLogger, span
}

func metadataToHTTPHeader(md metadata.MD) http.Header {
	if len(md) == 0 {
		return nil
	}
	headers := make(http.Header, len(md))
	for key, values := range md {
		valueCopy := make([]string, len(values))
		copy(valueCopy, values)
		headers[textproto.CanonicalMIMEHeaderKey(key)] = valueCopy
	}
	return headers
}

func collapseCapturedBodies(items []any) any {
	switch len(items) {
	case 0:
		return nil
	case 1:
		return items[0]
	default:
		return items
	}
}

func applyGRPCBodyDetails(ctxLogger *builder.Context) {
	v, ok := ctxLogger.Get(disableBodyKey)
	if ok {
		disabled, typeOK := v.(bool)
		if !typeOK || disabled {
			return
		}
	}

	details := ctxLogger.Details
	if details.System == "" {
		detailsAny, ok := ctxLogger.Get(detailsKey)
		if !ok {
			return
		}
		castDetails, castOK := detailsAny.(formatter.KibanaData)
		if !castOK {
			return
		}
		details = castDetails
	}
	if requestBody, ok := ctxLogger.Get(requestBodyKey); ok {
		details.Request = requestBody
	}
	if responseBody, ok := ctxLogger.Get(responseBodyKey); ok {
		details.Response = responseBody
	}
	ctxLogger.Details = details
	ctxLogger.Set(detailsKey, details)
}

func writeGRPCLog(ctxLogger *builder.Context, fullMethod string, err error) {
	if ctxLogger.Method == "" {
		ctxLogger.Method = fullMethod
		ctxLogger.Line = 1
	}

	if value, ok := ctxLogger.Get(formatter.InfoLevel); ok {
		if msg, castOK := value.(string); castOK {
			ctxLogger.Info(msg)
			return
		}
	}
	if value, ok := ctxLogger.Get(formatter.DebugLevel); ok {
		if msg, castOK := value.(string); castOK {
			ctxLogger.Debug(msg)
			return
		}
	}
	if value, ok := ctxLogger.Get(formatter.WarnLevel); ok {
		if msg, castOK := value.(string); castOK {
			ctxLogger.Warn(msg)
			return
		}
	}
	if value, ok := ctxLogger.Get(formatter.ErrorLevel); ok {
		if loggedErr, castOK := value.(error); castOK && hasError(loggedErr) {
			ctxLogger.Error(loggedErr)
			return
		}
	}
	if !hasError(err) {
		return
	}
	ctxLogger.Error(err)
}

func hasError(err error) bool {
	if err == nil {
		return false
	}
	value := reflect.ValueOf(err)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}

func grpcMethodName(fullMethod string) string {
	parts := strings.Split(strings.TrimPrefix(fullMethod, "/"), "/")
	if len(parts) == 0 {
		return fullMethod
	}
	return parts[len(parts)-1]
}
