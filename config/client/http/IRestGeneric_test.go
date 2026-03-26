// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
)

func getDefaultTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:          100,
		MaxConnsPerHost:       200,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		ForceAttemptHTTP2: true,
	}
}

func TestNewGenericRest(t *testing.T) {
	NewGenericRest(time.Second, getDefaultTransport())
}

func TestRestGeneric_PostGeneric(t *testing.T) {
	type response struct {
		Message string `json:"message"`
		Method  string `json:"method"`
	}

	tests := []struct {
		name    string
		input   RequestGeneric
		wantErr bool
	}{
		{name: "success"},
		{name: "marshal_error", input: RequestGeneric{Request: func() {}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "success":
				var receivedMethod string
				var receivedBody string
				var receivedHeader string
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedMethod = r.Method
					receivedHeader = r.Header.Get("X-Post")
					body, _ := io.ReadAll(r.Body)
					receivedBody = string(body)
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(response{Message: "ok", Method: r.Method})
				}))
				defer ts.Close()

				respObj := &response{}
				input := RequestGeneric{
					Url:      ts.URL + "/create",
					Header:   http.Header{"X-Post": []string{"1"}},
					Request:  map[string]any{"name": "Manuel"},
					Response: respObj,
				}
				gr := NewGenericRest(time.Second, getDefaultTransport()).(*RestGeneric)
				gr.DisableTrace()
				gotErr := gr.PostGeneric(context.Background(), input)
				if gotErr != nil {
					t.Fatalf("PostGeneric() failed: %v", gotErr)
				}
				if receivedMethod != http.MethodPost {
					t.Fatalf("method = %s", receivedMethod)
				}
				if receivedHeader != "1" {
					t.Fatalf("x-post = %s", receivedHeader)
				}
				if !strings.Contains(receivedBody, "Manuel") {
					t.Fatalf("body = %s", receivedBody)
				}
				if respObj.Message != "ok" || respObj.Method != http.MethodPost {
					t.Fatalf("response = %+v", respObj)
				}
			case "marshal_error":
				gr := NewGenericRest(time.Second, getDefaultTransport()).(*RestGeneric)
				gr.DisableTrace()
				gotErr := gr.PostGeneric(context.Background(), tt.input)
				if gotErr == nil {
					t.Fatal("PostGeneric() succeeded unexpectedly")
				}
			}
		})
	}
}

func TestGenericRest_GetGeneric(t *testing.T) {
	type response struct {
		Message string `json:"message"`
		Query   string `json:"query"`
	}

	tests := []struct {
		name    string
		input   RequestGeneric
		wantErr bool
	}{
		{name: "success_with_url"},
		{name: "success_with_params"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "success_with_url", "success_with_params":
				var receivedMethod string
				var receivedQuery string
				var receivedHeader string
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedMethod = r.Method
					receivedQuery = r.URL.RawQuery
					receivedHeader = r.Header.Get("X-Token")
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(response{Message: "ok", Query: r.URL.RawQuery})
				}))
				defer ts.Close()

				respObj := &response{}
				input := RequestGeneric{
					System:   "sys",
					Process:  "get",
					Header:   http.Header{"X-Token": []string{"abc"}},
					Response: respObj,
				}
				if tt.name == "success_with_url" {
					input.Url = ts.URL + "/items"
				} else {
					input.Host = ts.URL
					input.Path = "items"
					input.Params = url.Values{"page": []string{"1"}, "filter": []string{"hello world"}}
				}

				gr := NewGenericRest(time.Second, getDefaultTransport()).(*RestGeneric)
				gr.DisableTrace()
				gotErr := gr.GetGeneric(context.Background(), input)
				if gotErr != nil {
					t.Fatalf("GetGeneric() failed: %v", gotErr)
				}
				if receivedMethod != http.MethodGet {
					t.Fatalf("method = %s", receivedMethod)
				}
				if receivedHeader != "abc" {
					t.Fatalf("x-token = %s", receivedHeader)
				}
				if tt.name == "success_with_params" {
					vals, err := url.ParseQuery(receivedQuery)
					if err != nil {
						t.Fatalf("ParseQuery() failed: %v", err)
					}
					if vals.Get("page") != "1" || vals.Get("filter") != "hello world" {
						t.Fatalf("query = %s", receivedQuery)
					}
				}
				if respObj.Message != "ok" {
					t.Fatalf("response = %+v", respObj)
				}
			}
		})
	}
}

func TestGenericRest_PutGeneric(t *testing.T) {
	type response struct {
		Message string `json:"message"`
		Method  string `json:"method"`
	}

	tests := []struct {
		name    string
		input   RequestGeneric
		wantErr bool
	}{
		{name: "success"},
		{name: "marshal_error", input: RequestGeneric{Request: func() {}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "success":
				var receivedMethod string
				var receivedBody string
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedMethod = r.Method
					body, _ := io.ReadAll(r.Body)
					receivedBody = string(body)
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(response{Message: "ok", Method: r.Method})
				}))
				defer ts.Close()

				respObj := &response{}
				input := RequestGeneric{
					Host:     ts.URL,
					Path:     "update",
					Header:   http.Header{"X-Put": []string{"1"}},
					Request:  map[string]any{"id": 10},
					Response: respObj,
				}
				gr := NewGenericRest(time.Second, getDefaultTransport()).(*RestGeneric)
				gr.DisableTrace()
				gotErr := gr.PutGeneric(context.Background(), input)
				if gotErr != nil {
					t.Fatalf("PutGeneric() failed: %v", gotErr)
				}
				if receivedMethod != http.MethodPut {
					t.Fatalf("method = %s", receivedMethod)
				}
				if !strings.Contains(receivedBody, "10") {
					t.Fatalf("body = %s", receivedBody)
				}
				if respObj.Message != "ok" || respObj.Method != http.MethodPut {
					t.Fatalf("response = %+v", respObj)
				}
			case "marshal_error":
				gr := NewGenericRest(time.Second, getDefaultTransport()).(*RestGeneric)
				gr.DisableTrace()
				gotErr := gr.PutGeneric(context.Background(), tt.input)
				if gotErr == nil {
					t.Fatal("PutGeneric() succeeded unexpectedly")
				}
			}
		})
	}
}

func TestGenericRest_PatchGeneric(t *testing.T) {
	type response struct {
		Message string `json:"message"`
		Method  string `json:"method"`
	}

	tests := []struct {
		name    string
		input   RequestGeneric
		wantErr bool
	}{
		{name: "success"},
		{name: "marshal_error", input: RequestGeneric{Request: func() {}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "success":
				var receivedMethod string
				var receivedBody string
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedMethod = r.Method
					body, _ := io.ReadAll(r.Body)
					receivedBody = string(body)
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(response{Message: "ok", Method: r.Method})
				}))
				defer ts.Close()

				respObj := &response{}
				input := RequestGeneric{
					Url:      ts.URL + "/patch",
					Request:  map[string]any{"active": true},
					Response: respObj,
				}
				gr := NewGenericRest(time.Second, getDefaultTransport()).(*RestGeneric)
				gr.DisableTrace()
				gotErr := gr.PatchGeneric(context.Background(), input)
				if gotErr != nil {
					t.Fatalf("PatchGeneric() failed: %v", gotErr)
				}
				if receivedMethod != http.MethodPatch {
					t.Fatalf("method = %s", receivedMethod)
				}
				if !strings.Contains(receivedBody, "true") {
					t.Fatalf("body = %s", receivedBody)
				}
				if respObj.Message != "ok" || respObj.Method != http.MethodPatch {
					t.Fatalf("response = %+v", respObj)
				}
			case "marshal_error":
				gr := NewGenericRest(time.Second, getDefaultTransport()).(*RestGeneric)
				gr.DisableTrace()
				gotErr := gr.PatchGeneric(context.Background(), tt.input)
				if gotErr == nil {
					t.Fatal("PatchGeneric() succeeded unexpectedly")
				}
			}
		})
	}
}

func TestGenericRest_OptionGeneric(t *testing.T) {
	type response struct {
		Message string `json:"message"`
		Method  string `json:"method"`
	}

	tests := []struct {
		name    string
		input   RequestGeneric
		wantErr bool
	}{
		{name: "success"},
		{name: "marshal_error", input: RequestGeneric{Request: func() {}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "success":
				var receivedMethod string
				var receivedBody string
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedMethod = r.Method
					body, _ := io.ReadAll(r.Body)
					receivedBody = string(body)
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(response{Message: "ok", Method: r.Method})
				}))
				defer ts.Close()

				respObj := &response{}
				input := RequestGeneric{
					Host:     ts.URL,
					Path:     "options",
					Request:  map[string]any{"check": "yes"},
					Response: respObj,
				}
				gr := NewGenericRest(time.Second, getDefaultTransport()).(*RestGeneric)
				gr.DisableTrace()
				gotErr := gr.OptionGeneric(context.Background(), input)
				if gotErr != nil {
					t.Fatalf("OptionGeneric() failed: %v", gotErr)
				}
				if receivedMethod != http.MethodOptions {
					t.Fatalf("method = %s", receivedMethod)
				}
				if !strings.Contains(receivedBody, "yes") {
					t.Fatalf("body = %s", receivedBody)
				}
				if respObj.Message != "ok" || respObj.Method != http.MethodOptions {
					t.Fatalf("response = %+v", respObj)
				}
			case "marshal_error":
				gr := NewGenericRest(time.Second, getDefaultTransport()).(*RestGeneric)
				gr.DisableTrace()
				gotErr := gr.OptionGeneric(context.Background(), tt.input)
				if gotErr == nil {
					t.Fatal("OptionGeneric() succeeded unexpectedly")
				}
			}
		})
	}
}

func TestMockIRestGenericMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	input := RequestGeneric{System: "sys", Process: "proc"}
	expectedErr := errors.New("mock generic")

	mock := NewMockIRestGeneric(ctrl)
	mock.EXPECT().DisableTrace()
	mock.EXPECT().PostGeneric(ctx, input).Return(expectedErr)
	mock.EXPECT().GetGeneric(ctx, input).Return(expectedErr)
	mock.EXPECT().PutGeneric(ctx, input).Return(expectedErr)
	mock.EXPECT().PatchGeneric(ctx, input).Return(expectedErr)
	mock.EXPECT().OptionGeneric(ctx, input).Return(expectedErr)

	mock.DisableTrace()
	if err := mock.PostGeneric(ctx, input); !errors.Is(err, expectedErr) {
		t.Fatalf("PostGeneric() error = %v", err)
	}
	if err := mock.GetGeneric(ctx, input); !errors.Is(err, expectedErr) {
		t.Fatalf("GetGeneric() error = %v", err)
	}
	if err := mock.PutGeneric(ctx, input); !errors.Is(err, expectedErr) {
		t.Fatalf("PutGeneric() error = %v", err)
	}
	if err := mock.PatchGeneric(ctx, input); !errors.Is(err, expectedErr) {
		t.Fatalf("PatchGeneric() error = %v", err)
	}
	if err := mock.OptionGeneric(ctx, input); !errors.Is(err, expectedErr) {
		t.Fatalf("OptionGeneric() error = %v", err)
	}
}
