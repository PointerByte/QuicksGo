// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package client_gRPC provides a small wrapper around grpc.ClientConn for generated proto clients.
//
// The package is transport-oriented: it does not depend on a concrete generated client.
// Any client generated in the proto package can be created through BuildClient.
//
// When Connect creates the connection itself, it also attaches:
//   - TLS or mTLS transport credentials resolved from viper under client.grpc.*
//   - unary and stream interceptors that trace outbound requests through the logger package
//
// Supported configuration keys:
//   - client.grpc.tls.enable
//   - client.grpc.tls.caFile
//   - client.grpc.tls.serverName
//   - client.grpc.tls.version
//   - client.grpc.tls.insecureSkipVerify
//   - client.grpc.mtls.enable
//   - client.grpc.mtls.certFile
//   - client.grpc.mtls.keyFile
//
// You can also bypass viper-based transport resolution by calling SetTLSConfig
// with a prebuilt *tls.Config before Connect.
//
// Tracing stores gRPC metadata, target, method name, status code, request body,
// and response body in formatter.Service entries, mirroring the HTTP client
// tracing implemented in clientHttp.
//
// Basic usage:
//
//	package main
//
//	import (
//		"context"
//		"log"
//
//		"github.com/PointerByte/QuicksGo/config/client_gRPC"
//		pb "github.com/PointerByte/QuicksGo/config/proto"
//	)
//
//	func main() {
//		cli := client_gRPC.NewIClient(nil, nil)
//		cli.SetAddress("localhost:50051")
//
//		greeter, err := client_gRPC.BuildClient(cli, pb.NewGreeterClient)
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		resp, err := greeter.SayHello(context.Background(), &pb.HelloRequest{Name: "Manuel"})
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		log.Println(resp.GetMessage())
//	}
package client_gRPC
