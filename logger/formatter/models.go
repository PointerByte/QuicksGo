// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"net/http"
	"strings"
	"sync"
	"time"

	viperdata "github.com/PointerByte/GoForge/logger/viperData"
	"go.opentelemetry.io/otel/trace"
)

type LogFormat struct {
	Level     Level     `json:"level"`
	TraceID   string    `json:"traceID"`
	Message   string    `json:"message"`
	Details   Details   `json:"details"`
	Services  []Service `json:"services"`
	Timestamp string    `json:"timestamp"`
	Method    string    `json:"method"`
	Line      int       `json:"line"`
	Latency   int64     `json:"latency"`
}

type Details struct {
	System   string      `json:"system"`
	Client   string      `json:"client,omitempty"`
	Protocol string      `json:"protocol,omitempty"`
	Method   string      `json:"method,omitempty"`
	Path     string      `json:"path,omitempty"`
	Headers  http.Header `json:"headers,omitempty"`
	Request  any         `json:"request,omitempty"`
	Response any         `json:"response,omitempty"`
}

var mux sync.Mutex

func (k *Details) SetHeaders(headers http.Header) {
	if headers == nil {
		return
	}
	mux.Lock()
	defer mux.Unlock()

	if k.Headers == nil {
		k.Headers = make(http.Header, len(headers))
	}
	loggerIgnoredHeadersAtribute := string(viperdata.LoggerIgnoredHeadersAtribute)
	_ignoredHeaders := viperdata.GetViperData(loggerIgnoredHeadersAtribute).([]string)
	ignoredHeaders := strings.Join(_ignoredHeaders, ",")
	for key, vv := range headers {
		if strings.Contains(ignoredHeaders, key) {
			continue
		}
		vvCopy := make([]string, len(vv))
		copy(vvCopy, vv)
		k.Headers[key] = vvCopy
	}
}

func (k *Details) SetRequest(request any) {
	k.Request = request
}

func (k *Details) SetResponse(response any) {
	k.Response = response
}

type Service struct {
	TraceID string `json:"traceID,omitempty"`
	System  string `json:"system"`
	Process string `json:"process"`

	Server   string       `json:"server,omitempty"`
	Headers  *http.Header `json:"headers,omitempty"`
	Protocol string       `json:"protocol,omitempty"`
	Method   string       `json:"method,omitempty"`
	Code     int64        `json:"code,omitempty"`
	Path     string       `json:"path,omitempty"`

	DisableBody bool `json:"-"`
	Request     any  `json:"request,omitempty"`
	Response    any  `json:"response,omitempty"`

	Status  Status `json:"status"`
	Latency int64  `json:"latency"`

	TimeInit time.Time  `json:"-"`
	Span     trace.Span `json:"-"`
}
