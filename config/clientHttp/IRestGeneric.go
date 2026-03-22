// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -source=IRestGeneric.go -destination=./mocksIRestGeneric.go -package=clientHttp

// Package clientHttp provides a generic implementation for consuming HTTP
// services using common REST operations such as POST, GET, PUT, PATCH,
// and OPTIONS.
//
// This package abstracts HTTP client configuration, header handling,
// context propagation, and response deserialization into user-defined
// structures.
//
// Example usage:
//
//	package main
//
//	import (
//		"context"
//		"fmt"
//		"net/http"
//		"net/url"
//		"time"
//
//		"your_project/clientHttp"
//	)
//
//	type ExampleResponse struct {
//		Message string `json:"message"`
//	}
//
//	func main() {
//		client := clientHttp.NewGenericRest(10*time.Second, nil)
//
//		ctx := context.Background()
//
//		// Example POST request
//		postInput := clientHttp.RequestGeneric{
//			System:  "example-system",
//			Process: "create-resource",
//			Url:     "https://httpbin.org/post",
//			Header: http.Header{
//				"Authorization": []string{"Bearer token"},
//			},
//			Request: map[string]any{
//				"name": "Manuel",
//			},
//			Response: &ExampleResponse{},
//		}
//
//		err := client.PostGeneric(ctx, postInput)
//		if err != nil {
//			panic(err)
//		}
//
//		fmt.Printf("POST Response: %+v\n", postInput.Response)
//
//		// Example GET request with query params
//		params := url.Values{}
//		params.Add("page", "1")
//		params.Add("limit", "10")
//
//		getInput := clientHttp.RequestGeneric{
//			System:  "example-system",
//			Process: "get-resources",
//			Host:    "https://httpbin.org",
//			Path:    "get",
//			Params:  params,
//			Header: http.Header{
//				"Authorization": []string{"Bearer token"},
//			},
//			Response: &ExampleResponse{},
//		}
//
//		err = client.GetGeneric(ctx, getInput)
//		if err != nil {
//			panic(err)
//		}
//
//		fmt.Printf("GET Response: %+v\n", getInput.Response)
//	}
package clientHttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/PointerByte/QuicksGo/logger/formatter"
	"github.com/gin-gonic/gin/binding"
)

// IRestGeneric defines the contract for executing generic REST requests.
//
// Each method receives a context to support cancellation and timeouts,
// along with a RequestGeneric instance that contains all required data
// to build and execute the HTTP request.
//
// The implementation is responsible for serializing the request body,
// setting headers, propagating context, and deserializing the response
// into the provided Response object.
type IRestGeneric interface {
	// DisableTrace disables automatic tracing for subsequent requests.
	DisableTrace()

	// PostGeneric executes an HTTP POST request.
	//
	// The input parameter contains the URL or Host/Path combination,
	// request body, headers, and the object where the response will
	// be deserialized.
	PostGeneric(context.Context, RequestGeneric) error

	// GetGeneric executes an HTTP GET request.
	//
	// Query parameters are taken from input.Params, and the response
	// is deserialized into input.Response.
	GetGeneric(ctx context.Context, input RequestGeneric) error

	// PutGeneric executes an HTTP PUT request.
	//
	// Typically used to replace or update existing resources.
	PutGeneric(ctx context.Context, input RequestGeneric) error

	// PatchGeneric executes an HTTP PATCH request.
	//
	// Used for partial updates of a resource.
	PatchGeneric(ctx context.Context, input RequestGeneric) error

	// OptionGeneric executes an HTTP OPTIONS request.
	//
	// Useful for retrieving the supported capabilities or methods
	// of a remote resource.
	OptionGeneric(ctx context.Context, input RequestGeneric) error
}

// RestGeneric implements the IRestGeneric interface using an internal
// IRest instance to execute HTTP requests and process responses.
type RestGeneric struct {
	mocks        IRestGeneric
	newIRest     IRest
	disableTrace bool
}

// NewGenericRest creates a new instance of IRestGeneric.
//
// The timeOut parameter defines the maximum duration for each request.
// The tr parameter allows injecting a custom HTTP transport.
func NewGenericRest(mocks IRestGeneric, timeout time.Duration, tr *http.Transport) IRestGeneric {
	return &RestGeneric{
		mocks:    mocks,
		newIRest: NewIRest(nil, timeout, tr),
	}
}

type handlerTrace func(process *formatter.Service)

func traceClient(ctx context.Context, system, process string, disableTraceBody bool) (*formatter.Service, handlerTrace) {
	ctxLogger := builder.New(ctx)
	service := &formatter.Service{
		System:      system,
		Process:     process,
		Status:      formatter.SUCCESS,
		DisableBody: disableTraceBody,
	}
	ctxLogger.TraceInit(service)
	return service, ctxLogger.TraceEnd
}

// buildService enriches the trace service entry with data extracted from the
// HTTP response and request objects used by the REST client.
//
// When tracing is enabled, it copies transport metadata such as host, headers,
// method, path, HTTP status code, and protocol into service. It also stores the
// outbound request body and attempts to deserialize the response body into
// object so the same value can be recorded in service.Response.
//
// The response body is always drained and closed before the function returns.
// If JSON decoding fails, the function tries to read the remaining body and
// stores its raw string representation instead.
func (gr *RestGeneric) buildService(service *formatter.Service, reqBody, object any, resp *http.Response) error {
	if gr.disableTrace {
		return nil
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.Request != nil {
		req := resp.Request
		service.Server = req.URL.Host
		service.Headers = &req.Header
		service.Method = req.Method
		service.Path = req.URL.Path
		if !gr.disableTrace {
			service.Request = reqBody
		}
	}
	if resp != nil && !gr.disableTrace {
		service.Code = int64(resp.StatusCode)
		service.Protocol = resp.Proto

		if err := json.NewDecoder(resp.Body).Decode(object); err != nil {
			_respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("problem decoding the response: %w", err)
			}
			service.Response = string(_respBody)
		}
		service.Response = object
	}
	return nil
}

// DisableTrace disables request tracing for the current instance.
func (gr *RestGeneric) DisableTrace() {
	gr.disableTrace = false
}

// PostGeneric executes an HTTP POST request with JSON content.
//
// If input.Url is provided, it is used directly; otherwise, the URL
// is constructed using input.Host and input.Path.
//
// The input.Request is serialized into JSON and the response is
// deserialized into input.Response.
func (gr *RestGeneric) PostGeneric(ctx context.Context, input RequestGeneric) error {
	if gr.mocks != nil {
		return gr.mocks.PostGeneric(ctx, input)
	}

	var process *formatter.Service
	var traceEnd handlerTrace
	if !gr.disableTrace {
		process, traceEnd = traceClient(ctx, input.System, input.Process, input.DisableTraceBody)
		defer traceEnd(process)
	}

	var pathEncode string
	if input.Url != "" {
		pathEncode = input.Url
	} else {
		pathEncode = fmt.Sprintf("%s/%s", input.Host, input.Path)
	}

	req, err := json.Marshal(input.Request)
	if err != nil {
		if gr.disableTrace {
			process.Status = formatter.ERROR
		}
		return err
	}
	gr.newIRest.SetRequest(input.HttpRequest)
	gr.newIRest.SetContext(ctx)
	gr.newIRest.SetHeaders(input.Header)

	resp, err := gr.newIRest.Post(pathEncode, binding.MIMEJSON, bytes.NewReader(req), input.Response)
	if err != nil {
		return fmt.Errorf("error consuming the service status[%d] error[%s]", resp.StatusCode, err.Error())
	}
	return gr.buildService(process, input.Request, input.Response, resp)
}

func buildURL(input RequestGeneric) (string, error) {
	var raw string
	if input.Url != "" {
		raw = input.Url
	} else {
		raw = strings.TrimRight(input.Host, "/") + "/" + strings.TrimLeft(input.Path, "/")
	}
	if len(input.Params) == 0 {
		return raw, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for key, vals := range input.Params {
		for _, val := range vals {
			q.Add(key, val)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// GetGeneric executes an HTTP GET request.
//
// If input.Params contains values, they are encoded as a query string
// and appended to the final URL. The response is deserialized into
// input.Response.
func (gr *RestGeneric) GetGeneric(ctx context.Context, input RequestGeneric) error {
	if gr.mocks != nil {
		return gr.mocks.GetGeneric(ctx, input)
	}

	var process *formatter.Service
	var traceEnd handlerTrace
	if !gr.disableTrace {
		process, traceEnd = traceClient(ctx, input.System, input.Process, input.DisableTraceBody)
		defer traceEnd(process)
	}

	pathEncode, err := buildURL(input)
	if err != nil {
		return err
	}
	gr.newIRest.SetRequest(input.HttpRequest)
	gr.newIRest.SetContext(ctx)
	gr.newIRest.SetHeaders(input.Header)

	resp, err := gr.newIRest.Get(pathEncode, binding.MIMEJSON, input.Response)
	if err != nil {
		return fmt.Errorf("error consuming the service status[%d] error[%s]", resp.StatusCode, err.Error())
	}
	return gr.buildService(process, input.Request, input.Response, resp)
}

// PutGeneric executes an HTTP PUT request with JSON content.
//
// The input.Request is serialized into JSON and the response is
// deserialized into input.Response.
func (gr *RestGeneric) PutGeneric(ctx context.Context, input RequestGeneric) error {
	if gr.mocks != nil {
		return gr.mocks.PutGeneric(ctx, input)
	}

	var process *formatter.Service
	var traceEnd handlerTrace
	if !gr.disableTrace {
		process, traceEnd = traceClient(ctx, input.System, input.Process, input.DisableTraceBody)
		defer traceEnd(process)
	}

	var pathEncode string
	if input.Url != "" {
		pathEncode = input.Url
	} else {
		pathEncode = fmt.Sprintf("%s/%s", input.Host, input.Path)
	}

	req, err := json.Marshal(input.Request)
	if err != nil {
		if gr.disableTrace {
			process.Status = formatter.ERROR
		}
		return err
	}
	gr.newIRest.SetRequest(input.HttpRequest)
	gr.newIRest.SetContext(ctx)
	gr.newIRest.SetHeaders(input.Header)

	resp, err := gr.newIRest.Put(pathEncode, binding.MIMEJSON, bytes.NewReader(req), input.Response)
	if err != nil {
		return fmt.Errorf("error consuming the service status[%d] error[%s]", resp.StatusCode, err.Error())
	}
	return gr.buildService(process, input.Request, input.Response, resp)
}

// PatchGeneric executes an HTTP PATCH request with JSON content.
//
// The input.Request is serialized into JSON and the response is
// deserialized into input.Response.
func (gr *RestGeneric) PatchGeneric(ctx context.Context, input RequestGeneric) error {
	if gr.mocks != nil {
		return gr.mocks.PatchGeneric(ctx, input)
	}

	var process *formatter.Service
	var traceEnd handlerTrace
	if !gr.disableTrace {
		process, traceEnd = traceClient(ctx, input.System, input.Process, input.DisableTraceBody)
		defer traceEnd(process)
	}

	var pathEncode string
	if input.Url != "" {
		pathEncode = input.Url
	} else {
		pathEncode = fmt.Sprintf("%s/%s", input.Host, input.Path)
	}

	req, err := json.Marshal(input.Request)
	if err != nil {
		if gr.disableTrace {
			process.Status = formatter.ERROR
		}
		return err
	}
	gr.newIRest.SetRequest(input.HttpRequest)
	gr.newIRest.SetContext(ctx)
	gr.newIRest.SetHeaders(input.Header)

	resp, err := gr.newIRest.Patch(pathEncode, binding.MIMEJSON, bytes.NewReader(req), input.Response)
	if err != nil {
		return fmt.Errorf("error consuming the service status[%d] error[%s]", resp.StatusCode, err.Error())
	}
	return gr.buildService(process, input.Request, input.Response, resp)
}

// OptionGeneric executes an HTTP OPTIONS request.
//
// This method can be used to retrieve the capabilities supported by
// a remote resource. If the response contains a body, it will be
// deserialized into input.Response.
func (gr *RestGeneric) OptionGeneric(ctx context.Context, input RequestGeneric) error {
	if gr.mocks != nil {
		return gr.mocks.OptionGeneric(ctx, input)
	}

	var process *formatter.Service
	var traceEnd handlerTrace
	if !gr.disableTrace {
		process, traceEnd = traceClient(ctx, input.System, input.Process, input.DisableTraceBody)
		defer traceEnd(process)
	}

	var pathEncode string
	if input.Url != "" {
		pathEncode = input.Url
	} else {
		pathEncode = fmt.Sprintf("%s/%s", input.Host, input.Path)
	}

	req, err := json.Marshal(input.Request)
	if err != nil {
		if gr.disableTrace {
			process.Status = formatter.ERROR
		}
		return err
	}
	gr.newIRest.SetRequest(input.HttpRequest)
	gr.newIRest.SetContext(ctx)
	gr.newIRest.SetHeaders(input.Header)

	resp, err := gr.newIRest.Option(pathEncode, binding.MIMEJSON, bytes.NewReader(req), input.Response)
	if err != nil {
		return fmt.Errorf("error consuming the service status[%d] error[%s]", resp.StatusCode, err.Error())
	}
	return gr.buildService(process, input.Request, input.Response, resp)
}
