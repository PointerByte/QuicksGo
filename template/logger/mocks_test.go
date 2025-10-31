package logger

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MocksControl(t *testing.T) {
	t.Run("Enable and Disable Mocks", func(t *testing.T) {
		EnableMocks()
		assert.NotNil(t, MocksLogger)
		DisableMocks()
		assert.Nil(t, MocksLogger)
	})

	t.Run("InitLogger with mocks returns nil", func(t *testing.T) {
		mocked := new(Mocks)
		mocked.On("InitLogger").Return(nil)
		MocksLogger = mocked

		err := InitLogger()
		assert.NoError(t, err)
		mocked.AssertExpectations(t)

		DisableMocks()
	})

	t.Run("InitLogger with mocks returns error", func(t *testing.T) {
		mocked := new(Mocks)
		mocked.On("InitLogger").Return(errors.New("mocked error"))
		MocksLogger = mocked

		err := InitLogger()
		assert.EqualError(t, err, "mocked error")
		mocked.AssertExpectations(t)

		DisableMocks()
	})

	t.Run("InitLogger without mocks uses real", func(t *testing.T) {
		DisableMocks()
		// Just test that real initLogger is called without panic
		_ = InitLogger()
	})
}
