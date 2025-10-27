package logger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLogs(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		messageErr error
	}{
		{
			name:    "test Info",
			message: "unit test log",
		},
		{
			name:       "test Error",
			messageErr: errors.New("unit test log"),
		},
		{
			name:    "test Warning",
			message: "unit test log",
		},
		{
			name:    "test Panic",
			message: "unit test log",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocksLogger := new(Mocks)
			mocksLogger.On("emitOtel", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
			emitOtel = mocksLogger.emitOtel

			ctx := ContextLogger(context.Background())
			attributesMap := make(map[string]any)
			attributesMap["test"] = true
			ctx = context.WithValue(ctx, attributesKey, attributesMap)
			switch tt.name {
			case "test Info":
				Info(ctx, tt.message)
			case "test Error":
				Error(ctx, tt.messageErr)
			case "test Warning":
				Warning(ctx, tt.message)
			default:
				assert.Panics(t, func() {
					Panic(ctx, tt.messageErr)
				}, "expected panic from MustFail")
			}
			time.Sleep(time.Second)
			ClearFile()

			mocksLogger.AssertExpectations(t)
		})
	}
}
