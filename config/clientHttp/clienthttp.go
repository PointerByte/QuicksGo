// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package clientHttp

import (
	"net/http"
	"time"
)

// NewRestClient creates an http client
func NewRestClient(timeout time.Duration, tr *http.Transport) *http.Client {
	if tr == nil {
		tr = http.DefaultTransport.(*http.Transport).Clone()
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
}
