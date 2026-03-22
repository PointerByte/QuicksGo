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
		MaxIdleConns:          100,              // MÃ¡ximo de conexiones ociosas
		MaxConnsPerHost:       200,              // LÃ­mite de conexiones por host simultÃ¡neas
		MaxIdleConnsPerHost:   10,               // MÃ¡ximo de conexiones ociosas por host
		IdleConnTimeout:       90 * time.Second, // tiempo de espera para conexiones ociosas
		TLSHandshakeTimeout:   10 * time.Second, // tiempo mÃ¡ximo para el handshake TLS
		ExpectContinueTimeout: 1 * time.Second,  // tiempo de espera para Expect: 100-continue
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // tiempo mÃ¡ximo para conexiones
			KeepAlive: 30 * time.Second, // Mantener conexiones activas
		}).DialContext,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		ForceAttemptHTTP2: true, // Intenta HTTP/2 incluso para HTTP
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
