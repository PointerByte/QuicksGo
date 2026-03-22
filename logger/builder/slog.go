// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"time"

	"log/slog"

	"github.com/PointerByte/QuicksGo/logger/formatter"
	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
)

type jsonHandler struct {
	level    slog.Level
	w        io.Writer
	attrs    []slog.Attr
	groups   []string
	handlers []slog.Handler
}

func newHandler(level slog.Level, w io.Writer, handlers ...slog.Handler) *jsonHandler {
	return &jsonHandler{level: level, w: w, handlers: handlers}
}

func (h *jsonHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *jsonHandler) Handle(ctx context.Context, r slog.Record) error {
	data := make(map[string]any)
	ctxLogger := New(ctx)
	maps.Copy(data, ctxLogger.customLogFormat())
	layout := viperdata.GetViperData(string(viperdata.LoggerFormatDateAtribute)).(string)
	data[string(timestampAtribute)] = time.Now().Format(layout)
	data[string(loggerMessage)] = r.Message
	data[string(levelAtribute)] = r.Level.String()

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

	var logObj formatter.LogFormat
	if err = json.Unmarshal(jsonBytes, &logObj); err != nil {
		panic(err)
	}

	formatterAtribute := viperdata.GetViperData(string(viperdata.LoggerFormatterAtribute)).(string)
	formatter := formatter.New(formatterAtribute)
	logObj.Latency = ctxLogger.GetLatency()
	jsonBytes, err = formatter.Format(logObj)
	if err != nil {
		panic(err)
	}
	var jsonMap any
	if err := json.Unmarshal(jsonBytes, &jsonMap); err != nil {
		if err = h.writeData(jsonBytes); err != nil {
			panic(err)
		}
		return nil
	}
	if err = h.writeData(jsonBytes); err != nil {
		panic(err)
	}
	return nil
}

func (h *jsonHandler) writeData(jsonBytes []byte) error {
	_, err := h.w.Write(jsonBytes)
	if err != nil {
		return err
	}
	_, err = h.w.Write([]byte("\n"))
	if err != nil {
		return err
	}
	return nil

}

func (h *jsonHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(clone.attrs, attrs...)
	return &clone
}

func (h *jsonHandler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.groups = append(clone.groups, name)
	return &clone
}
