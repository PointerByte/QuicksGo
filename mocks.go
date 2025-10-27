package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
)

var MocksConfig *Mocks

func EnableMocksConfig() {
	MocksConfig = new(Mocks)
}

func DisableMocksConfig() {
	MocksConfig = nil
}

var ReadInConfig = func() error {
	if MocksConfig != nil {
		return MocksConfig.ReadInConfig()
	}
	return viper.ReadInConfig()
}

type Mocks struct {
	mock.Mock
}

func (m *Mocks) ReadInConfig() error {
	return m.Called().Error(0)
}

var MocksMain *Mocks

func EnableMocksMain() {
	MocksMain = new(Mocks)
}

var CreateApp = func() (*gin.Engine, error) {
	if MocksMain != nil {
		return MocksMain.CreateApp()
	}
	return createApp()
}

func (m *Mocks) CreateApp() (*gin.Engine, error) {
	args := m.Called()
	return args.Get(0).(*gin.Engine), args.Error(1)
}

var Start = func(srv *http.Server) {
	if MocksMain != nil {
		MocksMain.Start(srv)
		return
	}
	start(srv)
}

func (m *Mocks) Start(srv *http.Server) {
	m.Called(srv)
}
