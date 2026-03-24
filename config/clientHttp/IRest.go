// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -source=IRest.go -destination=./mocksIRest.go -package=clientHttp

package clientHttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// IRest defines an interface for making HTTP requests in a generic manner.
//
// Implementations of this interface (such as `subRest` or `mocks`) allow
// you to execute HTTP methods (`GET`, `POST`, `PUT`, `PATCH`) and handle headers,
// context, and response decoding in a target object.
//
// All functions return the HTTP status code (`int`) and a possible
// error (`error`) describing transport, encoding, or response failures.
type IRest interface {
	// SetContext sets a context (`context.Context`) that will be used
	// by HTTP requests. It allows you to control cancellations and timeouts.
	SetContext(ctx context.Context)

	// SetRequest defines a custom request (*http.Request).
	SetRequest(req *http.Request)

	// SetHeaders defines the base HTTP headers that will be included in every
	// request. If called again, the previous values are overwritten.
	SetHeaders(header http.Header)

	// Executes a pre-built HTTP request and decodes the response
	// into the provided object, if applicable.
	Do(object any) (*http.Response, error)

	// Get sends an HTTP GET request to the specified `url`, with the specified
	// content type, and decodes the response into an `object`.
	Get(url, contentType string, object any) (*http.Response, error)

	// Post sends an HTTP POST request to the specified `url`, using `body`
	// as the request body, and decodes the response into `object`.
	Post(url, contentType string, body io.Reader, object any) (*http.Response, error)

	// Put sends a PUT HTTP request to the specified `url`, using `body`
	// as the request body, and decodes the response into `object`.
	Put(url, contentType string, body io.Reader, object any) (*http.Response, error)

	// Patch sends an HTTP PATCH request to the specified `url`, using `body`
	// as the request body, and decodes the response into `object`.
	Patch(url, contentType string, body io.Reader, object any) (*http.Response, error)

	// Option sends an HTTP OPTIONS request to the specified `url`, using `body`
	// as the request body, and decodes the response into `object`.
	Option(url, contentType string, body io.Reader, object any) (*http.Response, error)
}

type Rest struct {
	mocks       IRest
	restClient  *http.Client
	hdr         atomic.Value // stores http.Header (immutable after a Store operation)
	ctx         atomic.Value // stores context.Context
	withContext atomic.Bool
	mux         sync.Mutex
	req         *http.Request
}

// NewIRest creates a new instance of IRest.
//
// The timeOut parameter defines the maximum duration for each request.
// The tr parameter allows injecting a custom HTTP transport.
func NewIRest(mocks IRest, timeout time.Duration, tr *http.Transport) IRest {
	s := &Rest{
		mocks:      mocks,
		restClient: NewRestClient(timeout, tr),
		req:        &http.Request{},
	}
	s.hdr.Store(http.Header{}) // Init value imutable
	return s
}

func (sr *Rest) SetRequest(req *http.Request) {
	sr.req = req
}

func (sr *Rest) cloneRequest(req *http.Request) *http.Request {
	sr.mux.Lock()
	defer sr.mux.Unlock()
	if sr.req == nil {
		return req
	}
	ctx := sr.req.Context()
	return req.Clone(ctx)
}

func (sr *Rest) doRequest(object any) (*http.Response, error) {
	sr.mux.Lock()
	defer sr.mux.Unlock()

	// Merge the base header with the one from the request, without overwriting what's already there.
	base := sr.hdr.Load().(http.Header)
	if sr.req.Header == nil {
		sr.req.Header = make(http.Header)
	}
	for k, vals := range base {
		// If the request already includes a header (e.g., Content-Type), we respect it.
		if _, exists := sr.req.Header[k]; exists {
			continue
		}
		for _, v := range vals {
			sr.req.Header.Add(k, v)
		}
	}

	resp, err := sr.restClient.Do(sr.req)
	if err != nil {
		return nil, fmt.Errorf("problema al ejecutar la solicitud:: %w", err)
	}
	return resp, nil
}

func (sr *Rest) Do(object any) (*http.Response, error) {
	if sr.mocks != nil {
		return sr.mocks.Do(object)
	}
	return sr.doRequest(object)
}

func (sr *Rest) SetHeaders(header http.Header) {
	if header == nil {
		return
	}
	sr.hdr.Store(header.Clone())
}

func (sr *Rest) SetContext(ctx context.Context) {
	sr.ctx.Store(ctx)
	sr.withContext.Store(true)
}

func (sr *Rest) Get(url, contentType string, object any) (*http.Response, error) {
	if sr.mocks != nil {
		return sr.mocks.Get(url, contentType, object)
	}

	var req *http.Request
	var err error
	if sr.withContext.Load() {
		ctx, _ := sr.ctx.Load().(context.Context)
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	} else {
		req, err = http.NewRequest(http.MethodGet, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("Problem creating the request GET: %w", err)
	}
	sr.req = sr.cloneRequest(req)

	// Take a snapshot of the header and include the Content-Type field ONLY in this request
	h := sr.hdr.Load().(http.Header)
	sr.req.Header = h.Clone()
	sr.req.Header.Set("Content-Type", contentType)
	return sr.doRequest(object)
}

func (sr *Rest) Post(url, contentType string, body io.Reader, object any) (*http.Response, error) {
	if sr.mocks != nil {
		return sr.mocks.Post(url, contentType, body, object)
	}

	var req *http.Request
	var err error
	if sr.withContext.Load() {
		ctx, _ := sr.ctx.Load().(context.Context)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	} else {
		req, err = http.NewRequest(http.MethodPost, url, body)
	}
	if err != nil {
		return nil, fmt.Errorf("Problem creating the request POST: %w", err)
	}
	sr.req = sr.cloneRequest(req)

	// Take a snapshot of the header and include the Content-Type field ONLY in this request
	h := sr.hdr.Load().(http.Header)
	sr.req.Header = h.Clone()
	sr.req.Header.Set("Content-Type", contentType)
	return sr.doRequest(object)
}

func (sr *Rest) Put(url, contentType string, body io.Reader, object any) (*http.Response, error) {
	if sr.mocks != nil {
		return sr.mocks.Put(url, contentType, body, object)
	}

	var req *http.Request
	var err error
	if sr.withContext.Load() {
		ctx, _ := sr.ctx.Load().(context.Context)
		req, err = http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	} else {
		req, err = http.NewRequest(http.MethodPut, url, body)
	}
	if err != nil {
		return nil, fmt.Errorf("Problem creating the request PUT: %w", err)
	}
	sr.req = sr.cloneRequest(req)

	// Take a snapshot of the header and include the Content-Type field ONLY in this request
	h := sr.hdr.Load().(http.Header)
	sr.req.Header = h.Clone()
	sr.req.Header.Set("Content-Type", contentType)
	return sr.doRequest(object)
}

func (sr *Rest) Patch(url, contentType string, body io.Reader, object any) (*http.Response, error) {
	if sr.mocks != nil {
		return sr.mocks.Patch(url, contentType, body, object)
	}

	var req *http.Request
	var err error
	if sr.withContext.Load() {
		ctx, _ := sr.ctx.Load().(context.Context)
		req, err = http.NewRequestWithContext(ctx, http.MethodPatch, url, body)
	} else {
		req, err = http.NewRequest(http.MethodPatch, url, body)
	}
	if err != nil {
		return nil, fmt.Errorf("Problem creating the request PATCH: %w", err)
	}
	sr.req = sr.cloneRequest(req)

	// Take a snapshot of the header and include the Content-Type field ONLY in this request
	h := sr.hdr.Load().(http.Header)
	sr.req.Header = h.Clone()
	sr.req.Header.Set("Content-Type", contentType)
	return sr.doRequest(object)
}

func (sr *Rest) Option(url, contentType string, body io.Reader, object any) (*http.Response, error) {
	if sr.mocks != nil {
		return sr.mocks.Option(url, contentType, body, object)
	}

	var req *http.Request
	var err error
	if sr.withContext.Load() {
		ctx, _ := sr.ctx.Load().(context.Context)
		req, err = http.NewRequestWithContext(ctx, http.MethodOptions, url, body)
	} else {
		req, err = http.NewRequest(http.MethodOptions, url, body)
	}
	if err != nil {
		return nil, fmt.Errorf("Problem creating the request Option: %w", err)
	}
	sr.req = sr.cloneRequest(req)

	// Take a snapshot of the header and include the Content-Type field ONLY in this request
	h := sr.hdr.Load().(http.Header)
	sr.req.Header = h.Clone()
	sr.req.Header.Set("Content-Type", contentType)
	return sr.doRequest(object)
}
