// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/PointerByte/QuicksGo/logger/formatter"
	"github.com/PointerByte/QuicksGo/logger/utilities"
	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	"go.opentelemetry.io/otel/attribute"
)

func (c *Context) getMethodLine(skip int) (funcName string, line int) {
	if c.Method != "" && c.Line != 0 {
		return c.Method, c.Line
	}
	return utilities.TraceCaller(skip + 1)
}

func (c *Context) customLogFormat() map[string]any {
	// ---------- Get Line ----------
	funcName, line := c.getMethodLine(6)

	// ---------- TraceID ----------
	var traceID string
	if v, ok := c.Get(traceIDKey); ok {
		traceID = v.(string)
	}

	// ---------- Datos Kibana ----------
	var details formatter.KibanaData
	if v, ok := c.Get(detailsKey); ok {
		details = v.(formatter.KibanaData)
	}
	if details.System == "" {
		details.System = viperdata.GetViperData(string(viperdata.AppAtribute)).(string)
	}

	// ---------- SatÃ©lites ----------
	var services *[]formatter.Service
	if v, ok := c.Get(servicesKey); ok {
		services = v.(*[]formatter.Service)
	}
	defer func() {
		c.mux.Lock()
		*services = make([]formatter.Service, 0)
		c.mux.Unlock()
	}()

	// ---------- Format Logger ----------
	entry := formatter.LogFormat{
		TraceID:  traceID,
		Details:  details,
		Services: *services,
		Method:   funcName,
		Line:     line,
	}
	jsonBytes, _ := json.Marshal(entry)
	var m map[string]any
	_ = json.Unmarshal(jsonBytes, &m)
	return m
}

func convertStr(input any) string {
	valueStr, ok := input.(string)
	if !ok {
		marshal, err := json.Marshal(input)
		if err == nil {
			return string(marshal)
		}
	}
	return valueStr
}

// TraceInit marks the start of tracing for a process or subprocess.
//
// It records the start time in process.TimeInit and, if tracing is
// enabled, creates a span associated with the process.
//
// If the logger is in test mode, it does nothing.
//
// Recommended usage:
//
//	process := &formatter.Services{
//	    Process: â€œprocessPaymentâ€,
//	    System:  â€œpaymentsâ€,
//	}
//	ctx.TraceInit(process)
//	defer ctx.TraceEnd(process)
func (c *Context) TraceInit(process *formatter.Service) {
	if viperdata.GetViperData(string(viperdata.LoggerModeTestAtribute)).(bool) {
		return
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	c.startSpan(process)
	process.TimeInit = time.Now()
}

func (c *Context) startSpan(process *formatter.Service) {
	if c.disableTrace {
		return
	}
	c.Context, process.Span = c.tracer.Start(c.Context, string(process.Process))
	process.Span.SetAttributes(attribute.String(string(systemAtribute), process.System))
}

// TraceEnd completes the measurement started with TraceInit.
//
// This function:
//
//   - assigns the trace ID to the process, if applicable
//   - classifies the status based on the HTTP code
//   - calculates the process latency
//   - adds the process to the context's service list
//   - records attributes in the span and closes it
//
// If the logger is in test mode, it performs no action.
//
// It should normally be used with defer immediately after TraceInit.
func (c *Context) TraceEnd(process *formatter.Service) {
	if viperdata.GetViperData(string(viperdata.LoggerModeTestAtribute)).(bool) {
		return
	}
	c.mux.Lock()
	defer c.mux.Unlock()

	c.setTraceID(process)
	classifyStatus(process)
	ignoreHeaders(process)

	services, _ := c.Get(servicesKey)
	vl := services.(*[]formatter.Service)
	process.Latency = time.Since(process.TimeInit).Milliseconds()
	*vl = append(*vl, *process)

	c.setSpanAttributes(process)
}

func ignoreHeaders(process *formatter.Service) {
	if process.Headers == nil || *process.Headers == nil {
		return
	}

	headers := make(http.Header, len(*process.Headers))
	loggerIgnoredHeadersAtribute := string(viperdata.LoggerIgnoredHeadersAtribute)
	_ignoredHeaders := viperdata.GetViperData(loggerIgnoredHeadersAtribute).([]string)
	ignoredHeaders := strings.Join(_ignoredHeaders, ",")
	for key, vv := range *process.Headers {
		if strings.Contains(ignoredHeaders, key) {
			continue
		}
		vvCopy := make([]string, len(vv))
		copy(vvCopy, vv)
		headers[key] = vvCopy
	}
	cloned := headers.Clone()
	process.Headers = &cloned
}

func (c *Context) setSpanAttributes(process *formatter.Service) {
	if c.disableTrace {
		return
	}
	defer process.Span.End()
	process.Span.SetAttributes(attribute.String(string(statusAtribute), string(process.Status)))
}

func (c *Context) setTraceID(process *formatter.Service) {
	if c.disableTrace {
		return
	}
	traceID := process.Span.SpanContext().TraceID()
	if traceID.IsValid() {
		process.TraceID = traceID.String()
	}
}

func classifyStatus(process *formatter.Service) {
	if process.Code == 0 {
		return
	}
	switch {
	case process.Code >= 200 && process.Code < 300:
		process.Status = formatter.SUCCESS
	case process.Code >= 400 && process.Code < 600:
		process.Status = formatter.ERROR
	default:
		process.Status = formatter.OTHER
	}
}

// Info logs an informational message using the current context.
//
// If the context already contains Kibana base information, it copies the System,
// Service, and Client fields from the received data before storing it again.
//
// If the logger is in test mode, it does not generate any output.
//
// This function only logs the message; it does not create spans or measure latency on its own.
func (c *Context) Info(message string) {
	if viperdata.GetViperData(string(viperdata.LoggerModeTestAtribute)).(bool) {
		return
	}
	v, ok := c.Get(detailsKey)
	if ok {
		KibanaData := v.(formatter.KibanaData)
		c.Details.System = KibanaData.System
		c.Details.Client = KibanaData.Client
		c.Details.Method = KibanaData.Method
		c.Details.Protocol = KibanaData.Protocol
	}
	c.Set(detailsKey, c.Details)
	slog.InfoContext(c, message)
	c.startTime = time.Now()
}

// Debug logs a debug-level message using the current context.
//
// If the context already contains Kibana base information, it copies the System,
// Service, and Client fields from the received data before storing it again.
//
// If the logger is in test mode, it does not generate any output.
//
// This function is intended for development or troubleshooting purposes and
// should not be relied upon for critical operational logging in production
// environments unless debug logging is explicitly enabled.
//
// This function only logs the message; it does not create spans or measure latency on its own.
func (c *Context) Debug(message string) {
	if viperdata.GetViperData(string(viperdata.LoggerModeTestAtribute)).(bool) {
		return
	}
	v, ok := c.Get(detailsKey)
	if ok {
		KibanaData := v.(formatter.KibanaData)
		c.Details.System = KibanaData.System
		c.Details.Client = KibanaData.Client
		c.Details.Method = KibanaData.Method
		c.Details.Protocol = KibanaData.Protocol
	}
	c.Set(detailsKey, c.Details)
	slog.DebugContext(c, message)
}

// Warn logs a warning message using the current context.
//
// If the context already contains Kibana base information, it copies the System,
// Service, and Client fields from the received data before storing it again.
//
// If the logger is in test mode, it does not generate any output.
//
// This function only logs the message; it does not create spans or measure latency on its own.
func (c *Context) Warn(message string) {
	if viperdata.GetViperData(string(viperdata.LoggerModeTestAtribute)).(bool) {
		return
	}
	v, ok := c.Get(detailsKey)
	if ok {
		KibanaData := v.(formatter.KibanaData)
		c.Details.System = KibanaData.System
		c.Details.Client = KibanaData.Client
		c.Details.Method = KibanaData.Method
		c.Details.Protocol = KibanaData.Protocol
	}
	c.Set(detailsKey, c.Details)
	slog.WarnContext(c, message)
}

// Error logs an error message using `slog` with the current context.
//
// If the context already contains Kibana base information, it copies `System`,
// `Service`, and `Client` to the received details before storing them again.
//
// If the logger is in test mode, it does not generate any output.
//
// This function logs err.Error() and does not modify the execution flow;
// error handling remains the callerâ€™s responsibility.
func (c *Context) Error(err error) {
	if viperdata.GetViperData(string(viperdata.LoggerModeTestAtribute)).(bool) {
		return
	}
	v, ok := c.Get(detailsKey)
	if ok {
		KibanaData := v.(formatter.KibanaData)
		c.Details.System = KibanaData.System
		c.Details.Client = KibanaData.Client
		c.Details.Method = KibanaData.Method
		c.Details.Protocol = KibanaData.Protocol
	}
	c.Set(detailsKey, c.Details)
	slog.ErrorContext(c, err.Error())
}
