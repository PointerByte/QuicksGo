// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package server_gRPC

import (
	"context"
	"errors"
	"net"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/PointerByte/QuicksGo/config/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

func resetServerGRPCHandlerTestState(t *testing.T) {
	t.Helper()

	resetServerGRPCTestState(t)

	origFunctionsRefresh := functionsRefresh
	origRefreshHosts := refreshHosts
	origInvokeRefreshFn := invokeRefreshFn
	origRestartJobs := restartJobs

	functionsRefresh = nil
	refreshHosts = nil

	t.Cleanup(func() {
		functionsRefresh = origFunctionsRefresh
		refreshHosts = origRefreshHosts
		invokeRefreshFn = origInvokeRefreshFn
		restartJobs = origRestartJobs
	})
}

func startBufconnConfigServer(t *testing.T) (*Config, *grpc.ClientConn, func()) {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	srv := NewIConfig(nil, nil).(*Config)
	srv.SetListener(listener)
	if err := srv.Register(func(r grpc.ServiceRegistrar) {
		pb.RegisterGreeterServer(r, greeterService{})
	}); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve()
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() failed: %v", err)
	}

	cleanup := func() {
		conn.Close()
		srv.GracefulStop()
		select {
		case serveErr := <-errCh:
			if serveErr != nil {
				t.Fatalf("Serve() returned error: %v", serveErr)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Serve() did not stop after GracefulStop()")
		}
	}

	return srv, conn, cleanup
}

func TestSetFunctionsRefreshAndHosts(t *testing.T) {
	resetServerGRPCHandlerTestState(t)

	fn1 := func(context.Context) error { return nil }
	fn2 := func(context.Context) error { return nil }

	SetFunctionsRefresh(fn1)
	SetFunctionsRefresh(fn2)
	SetHostsRefresh("127.0.0.1:50051", "127.0.0.1:50052")

	if len(functionsRefresh) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(functionsRefresh))
	}
	if reflect.ValueOf(functionsRefresh[0]).Pointer() != reflect.ValueOf(fn1).Pointer() {
		t.Fatal("expected first function to be preserved")
	}
	if reflect.ValueOf(functionsRefresh[1]).Pointer() != reflect.ValueOf(fn2).Pointer() {
		t.Fatal("expected second function to be appended")
	}
	if want := []string{"127.0.0.1:50051", "127.0.0.1:50052"}; !reflect.DeepEqual(refreshHosts, want) {
		t.Fatalf("expected hosts %v, got %v", want, refreshHosts)
	}
}

func TestUnknownServiceHandlerNotFound(t *testing.T) {
	resetServerGRPCHandlerTestState(t)
	_, conn, cleanup := startBufconnConfigServer(t)
	defer cleanup()

	err := conn.Invoke(context.Background(), "/helloworld.Unknown/SayHello", &emptypb.Empty{}, &emptypb.Empty{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", status.Code(err))
	}
	if status.Convert(err).Message() != "Path not found" {
		t.Fatalf("unexpected message: %q", status.Convert(err).Message())
	}
}

func TestUnknownServiceHandlerNoMethod(t *testing.T) {
	resetServerGRPCHandlerTestState(t)
	_, conn, cleanup := startBufconnConfigServer(t)
	defer cleanup()

	err := conn.Invoke(context.Background(), "/helloworld.Greeter/Unknown", &emptypb.Empty{}, &emptypb.Empty{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected Unimplemented, got %v", status.Code(err))
	}
	if status.Convert(err).Message() != "Method not allow" {
		t.Fatalf("unexpected message: %q", status.Convert(err).Message())
	}
}

func TestUnknownServiceHandlerRefresh(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resetServerGRPCHandlerTestState(t)

		var callbackCalls int32
		var restartCalls int32
		SetFunctionsRefresh(func(context.Context) error {
			atomic.AddInt32(&callbackCalls, 1)
			return nil
		})
		SetHostsRefresh("node-a", "node-b")
		restartJobs = func() {
			atomic.AddInt32(&restartCalls, 1)
		}

		var (
			mu        sync.Mutex
			addresses []string
			mdValues  []string
		)
		invokeRefreshFn = func(ctx context.Context, address string) error {
			mu.Lock()
			addresses = append(addresses, address)
			md, _ := metadata.FromOutgoingContext(ctx)
			mdValues = append(mdValues, firstMetadataValue(md, "broadcast-refresh"))
			mu.Unlock()
			return nil
		}

		_, conn, cleanup := startBufconnConfigServer(t)
		defer cleanup()

		if err := conn.Invoke(context.Background(), refreshMethod, &emptypb.Empty{}, &emptypb.Empty{}); err != nil {
			t.Fatalf("refresh invoke failed: %v", err)
		}
		if atomic.LoadInt32(&callbackCalls) != 1 {
			t.Fatalf("expected 1 callback call, got %d", callbackCalls)
		}
		if atomic.LoadInt32(&restartCalls) != 1 {
			t.Fatalf("expected 1 restart call, got %d", restartCalls)
		}
		sort.Strings(addresses)
		if want := []string{"node-a", "node-b"}; !reflect.DeepEqual(addresses, want) {
			t.Fatalf("expected addresses %v, got %v", want, addresses)
		}
		sort.Strings(mdValues)
		if want := []string{"true", "true"}; !reflect.DeepEqual(mdValues, want) {
			t.Fatalf("expected outgoing metadata %v, got %v", want, mdValues)
		}
	})

	t.Run("broadcast skips callbacks and fanout", func(t *testing.T) {
		resetServerGRPCHandlerTestState(t)

		var callbackCalls int32
		var restartCalls int32
		SetFunctionsRefresh(func(context.Context) error {
			atomic.AddInt32(&callbackCalls, 1)
			return nil
		})
		SetHostsRefresh("node-a")
		restartJobs = func() {
			atomic.AddInt32(&restartCalls, 1)
		}

		var invokeCalls int32
		invokeRefreshFn = func(context.Context, string) error {
			atomic.AddInt32(&invokeCalls, 1)
			return nil
		}

		_, conn, cleanup := startBufconnConfigServer(t)
		defer cleanup()

		ctx := metadata.AppendToOutgoingContext(context.Background(), "broadcast-refresh", "true")
		if err := conn.Invoke(ctx, refreshMethod, &emptypb.Empty{}, &emptypb.Empty{}); err != nil {
			t.Fatalf("refresh broadcast invoke failed: %v", err)
		}
		if atomic.LoadInt32(&callbackCalls) != 0 {
			t.Fatalf("expected 0 callback calls, got %d", callbackCalls)
		}
		if atomic.LoadInt32(&invokeCalls) != 0 {
			t.Fatalf("expected 0 fanout calls, got %d", invokeCalls)
		}
		if atomic.LoadInt32(&restartCalls) != 0 {
			t.Fatalf("expected 0 restart calls, got %d", restartCalls)
		}
	})

	t.Run("callback error", func(t *testing.T) {
		resetServerGRPCHandlerTestState(t)

		var restartCalls int32
		SetFunctionsRefresh(func(context.Context) error {
			return errors.New("callback failed")
		})
		restartJobs = func() {
			atomic.AddInt32(&restartCalls, 1)
		}

		_, conn, cleanup := startBufconnConfigServer(t)
		defer cleanup()

		err := conn.Invoke(context.Background(), refreshMethod, &emptypb.Empty{}, &emptypb.Empty{})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal, got %v", status.Code(err))
		}
		if atomic.LoadInt32(&restartCalls) != 1 {
			t.Fatalf("expected 1 restart call, got %d", restartCalls)
		}
	})

	t.Run("fanout error", func(t *testing.T) {
		resetServerGRPCHandlerTestState(t)

		var restartCalls int32
		SetHostsRefresh("node-a")
		invokeRefreshFn = func(context.Context, string) error {
			return errors.New("fanout failed")
		}
		restartJobs = func() {
			atomic.AddInt32(&restartCalls, 1)
		}

		_, conn, cleanup := startBufconnConfigServer(t)
		defer cleanup()

		err := conn.Invoke(context.Background(), refreshMethod, &emptypb.Empty{}, &emptypb.Empty{})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal, got %v", status.Code(err))
		}
		if atomic.LoadInt32(&restartCalls) != 1 {
			t.Fatalf("expected 1 restart call, got %d", restartCalls)
		}
	})
}

func TestHandlerHelpers(t *testing.T) {
	resetServerGRPCHandlerTestState(t)

	if service, method := splitFullMethod("/helloworld.Greeter/SayHello"); service != "helloworld.Greeter" || method != "SayHello" {
		t.Fatalf("unexpected split result: %q %q", service, method)
	}
	if service, method := splitFullMethod("bad"); service != "" || method != "" {
		t.Fatalf("expected empty split result, got %q %q", service, method)
	}
	if got := firstMetadataValue(metadata.Pairs("broadcast-refresh", "true"), "broadcast-refresh"); got != "true" {
		t.Fatalf("expected metadata value true, got %q", got)
	}
	if status.Code(notFound()) != codes.NotFound {
		t.Fatalf("expected notFound code %v", codes.NotFound)
	}
	if status.Code(noMethod()) != codes.Unimplemented {
		t.Fatalf("expected noMethod code %v", codes.Unimplemented)
	}

	srv := grpc.NewServer()
	pb.RegisterGreeterServer(srv, greeterService{})
	if !registeredService(srv, "/helloworld.Greeter/Unknown") {
		t.Fatal("expected registered service to be detected")
	}
	if registeredService(srv, "/helloworld.Unknown/SayHello") {
		t.Fatal("expected missing service to return false")
	}
}
