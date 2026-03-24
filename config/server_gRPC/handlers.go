// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package server_gRPC

import (
	"context"
	"strings"
	"sync"

	"github.com/PointerByte/QuicksGo/config/client_gRPC"
	"github.com/PointerByte/QuicksGo/config/utilities/jobs"
	"github.com/PointerByte/QuicksGo/logger/builder"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type HandlerFunctionsRefresh func(ctx context.Context) error

var functionsRefresh []HandlerFunctionsRefresh
var refreshHosts []string

// refreshMethod is the administrative RPC used to restart package-level jobs,
// execute local refresh callbacks, and fan out the same refresh operation to
// other configured nodes.
const refreshMethod = "/quicksgo.admin/Refresh"

var invokeRefreshFn = func(ctx context.Context, address string) error {
	cli := client_gRPC.NewIClient(nil, nil)
	cli.SetAddress(address)
	cli.SetContext(ctx)
	if err := cli.Connect(); err != nil {
		return err
	}
	defer cli.Close()

	return cli.GetConn().Invoke(ctx, refreshMethod, &emptypb.Empty{}, &emptypb.Empty{})
}

// SetFunctionsRefresh registra callbacks locales que se ejecutarán cuando el
// servidor reciba la invocación administrativa de refresh.
func SetFunctionsRefresh(input ...HandlerFunctionsRefresh) {
	functionsRefresh = append(functionsRefresh, input...)
}

// SetHostsRefresh agrega hosts remotos que recibirán la propagación del
// refresh gRPC.
func SetHostsRefresh(input ...string) {
	refreshHosts = append(refreshHosts, input...)
}

func unknownServiceHandler(cfg *Config) grpc.StreamHandler {
	return func(_ any, stream grpc.ServerStream) error {
		method, ok := grpc.MethodFromServerStream(stream)
		if !ok {
			return notFound()
		}
		if method == refreshMethod {
			return refresh(stream)
		}
		if registeredService(cfg.GetServer(), method) {
			return noMethod()
		}
		return notFound()
	}
}

func notFound() error {
	return status.Error(codes.NotFound, "Path not found")
}

func noMethod() error {
	return status.Error(codes.Unimplemented, "Method not allow")
}

var restartJobs = jobs.RestartJobs

// refresh handles the administrative gRPC refresh flow exposed through
// `refreshMethod`.
//
// If the incoming metadata already includes `broadcast-refresh=true`, the
// method acknowledges the request without propagating it again. Otherwise it
// restarts package-level jobs, executes local refresh callbacks, and forwards
// the same administrative RPC to the configured remote hosts.
func refresh(stream grpc.ServerStream) error {
	var req emptypb.Empty
	if err := stream.RecvMsg(&req); err != nil {
		return status.Errorf(codes.Internal, "problem receiving refresh request: %v", err)
	}

	ctx := stream.Context()
	ctxLogger := builder.New(ctx)
	if incoming, ok := metadata.FromIncomingContext(ctx); ok && strings.EqualFold(firstMetadataValue(incoming, "broadcast-refresh"), "true") {
		ctxLogger.Info("Se valida que la tarea ya fue actualizada")
		return stream.SendMsg(&emptypb.Empty{})
	}

	restartJobs()
	for _, fn := range functionsRefresh {
		if err := fn(ctx); err != nil {
			return status.Errorf(codes.Internal, "problem executing refresh handlers: %v", err)
		}
	}
	if err := sendRefreshToHosts(ctx); err != nil {
		return status.Errorf(codes.Internal, "problem broadcasting refresh: %v", err)
	}
	return stream.SendMsg(&emptypb.Empty{})
}

func sendRefreshToHosts(ctx context.Context) error {
	if len(refreshHosts) == 0 {
		return nil
	}

	outgoingCtx := metadata.AppendToOutgoingContext(ctx, "broadcast-refresh", "true")
	errChan := make(chan error, len(refreshHosts))
	var wg sync.WaitGroup

	for _, host := range refreshHosts {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			errChan <- invokeRefreshFn(outgoingCtx, address)
		}(host)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}
	return nil
}

func registeredService(server *grpc.Server, fullMethod string) bool {
	if server == nil {
		return false
	}

	service, _ := splitFullMethod(fullMethod)
	if service == "" {
		return false
	}

	_, ok := server.GetServiceInfo()[service]
	return ok
}

func splitFullMethod(fullMethod string) (string, string) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(fullMethod), "/")
	if trimmed == "" {
		return "", ""
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func firstMetadataValue(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
