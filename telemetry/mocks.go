package telemetry

import (
	"context"

	"github.com/stretchr/testify/mock"
)

var MocksOtel *Mocks

func EnableMocks() {
	MocksOtel = new(Mocks)
}

func DisableMocks() {
	MocksOtel = nil
}

var InitOtel = func(ctx context.Context) (shutdown ShutdownOtel, err error) {
	if MocksOtel != nil {
		return MocksOtel.InitOtel(ctx)
	}
	return initOtel(ctx)
}

type Mocks struct {
	mock.Mock
}

func (m *Mocks) InitOtel(ctx context.Context) (shutdown ShutdownOtel, err error) {
	args := m.Called(ctx)
	return args.Get(0).(ShutdownOtel), args.Error(1)
}
