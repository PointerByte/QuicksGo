// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"net/http"
	"strings"
	"sync"
	"time"

	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	"go.opentelemetry.io/otel/trace"
)

type LogFormat struct {
	Timestamp string     `json:"timestamp"`
	TraceID   string     `json:"traceID"`
	Level     Level      `json:"level"`
	Message   string     `json:"message"`
	Details   KibanaData `json:"details"`
	Services  []Service  `json:"services"`
	Method    string     `json:"method"`
	Line      int        `json:"line"`
	Latency   int64      `json:"latency"`
}

type KibanaData struct {
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

func (k *KibanaData) SetHeaders(headers http.Header) {
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

func (k *KibanaData) SetRequest(request any) {
	k.Request = request
}

func (k *KibanaData) SetResponse(response any) {
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
