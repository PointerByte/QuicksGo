package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
)

var mocksConfig *Mocks

func EnableMocksConfig() {
	mocksConfig = new(Mocks)
}

func DisableMocksConfig() {
	mocksConfig = nil
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

func (m *Mocks) CreateApp() (*gin.Engine, error) {
	args := m.Called()
	return args.Get(0).(*gin.Engine), args.Error(1)
}

func (m *Mocks) Start(srv *http.Server) {
	m.Called(srv)
}
