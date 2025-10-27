package main

import (
	"errors"
	"quicksgo/logger"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func testMain(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "Success",
		},
		{
			name: "Error CreateApp",
			err:  errors.New("test error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set Mocks
			EnableMocksMain()
			engine, _ := createApp()
			MocksMain.On("CreateApp").Return(engine, tt.err)
			MocksMain.On("Start", mock.Anything).Return().Maybe()
			// Asserts Mocks
			defer MocksMain.AssertExpectations(t)

			// Run Main
			if tt.name != "Error CreateApp" {
				main()
				return
			}
			assert.Panics(t, func() {
				main()
			}, "expected panic from MustFail")
			logger.ClearFile()
		})
	}
}

func Test_main(t *testing.T) {
	testSetMode(t)
	testConfig(t)
	testMain(t)
}
