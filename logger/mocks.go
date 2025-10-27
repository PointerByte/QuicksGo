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

var InitLogger = func() error {
	if MocksLogger != nil {
		return MocksLogger.InitLogger()
	}
	return initLogger()
}

type Mocks struct {
	mock.Mock
}

func (m *Mocks) InitLogger() error {
	return m.Called().Error(0)
}

func (m *Mocks) emitOtel(ctx context.Context, _level level, result LogEntry) {
	m.Called(ctx, _level, result)
}
