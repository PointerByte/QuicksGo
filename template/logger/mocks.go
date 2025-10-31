package logger

import (
	"context"

	"github.com/stretchr/testify/mock"
)

var MocksLogger *Mocks

func EnableMocks() {
	MocksLogger = new(Mocks)
}

func DisableMocks() {
	MocksLogger = nil
}

type Mocks struct {
	mock.Mock
}

func (m *Mocks) InitLogger() error {
	return m.Called().Error(0)
}

func (m *Mocks) emitOtel(ctx context.Context, TraceID, SpanID string, level Level, result string) {
	m.Called(ctx, TraceID, SpanID, level, result)
}
