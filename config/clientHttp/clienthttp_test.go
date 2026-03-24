// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package clientHttp

import (
	"crypto/tls"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRestClient(t *testing.T) {
	tr := &http.Transport{
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

	got := NewRestClient(time.Second, tr)
	if got == nil {
		t.Fatalf("RestClient() returned nil client")
	}

	// Transport debe ser *http.Transport
	tr, ok := got.Transport.(*http.Transport)
	if !ok || tr == nil {
		t.Fatalf("client.Transport = %#v, want *http.Transport", got.Transport)
	}
}
