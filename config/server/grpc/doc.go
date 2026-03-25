// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package grpc provides server-side gRPC bootstrap helpers for the config
// module.
//
// It wraps grpc.Server creation so services can be started with shared
// configuration loading, tracing, TLS, mTLS, and graceful lifecycle handling.
//
// The package is transport-oriented: it does not depend on a concrete
// generated service. Any service generated in the proto package can be
// registered through Register.
//
// Before the server starts serving, Serve loads configuration through
// utilities.LoadEnv("."), so values such as server.grpc.port, TLS, and mTLS
// settings can be sourced from application.yml or application.json plus
// environment overrides.
//
// Basic usage:
//
//	package main
//
//	import (
//		"context"
//		"log"
//
//		pb "github.com/PointerByte/QuicksGo/config/proto"
//		servergrpc "github.com/PointerByte/QuicksGo/config/server/grpc"
//		grpcstd "google.golang.org/grpc"
//	)
//
//	type greeterServer struct {
//		pb.UnimplementedGreeterServer
//	}
//
//	func (s greeterServer) SayHello(_ context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
//		return &pb.HelloReply{Message: "hello " + req.GetName()}, nil
//	}
//
//	func (s greeterServer) CreateChat(stream pb.Greeter_CreateChatServer) error {
//		return nil
//	}
//
//	func (s greeterServer) StreamAlerts(stream pb.Greeter_StreamAlertsServer) error {
//		return nil
//	}
//
//	func main() {
//		srv := servergrpc.NewIConfig(nil, nil)
//		srv.SetAddress(":50051")
//
//		err := srv.Register(func(r grpcstd.ServiceRegistrar) {
//			pb.RegisterGreeterServer(r, greeterServer{})
//		})
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		log.Fatal(srv.Serve())
//	}
//
// If you already have your own listener, inject it with SetListener instead of
// SetAddress.
package grpc
