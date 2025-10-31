package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PointerByte/QuicksGo/controller"
	"github.com/PointerByte/QuicksGo/models"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestStatus(t *testing.T) {
	// Setup: configuramos los valores que usa viper internamente
	viper.Set("service.version", "1.2.3")
	viper.Set("service.name", "TestService")
	viper.Set("service.api.name", "TestAPI")

	// Set Gin en modo test
	gin.SetMode(gin.TestMode)

	// Preparamos el ResponseRecorder para capturar la respuesta
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Ejecutamos la función
	controller.Status(c)

	// Verificamos el código HTTP
	assert.Equal(t, http.StatusOK, w.Code)

	// Estructura esperada
	expected := models.Response[map[string]any]{
		Status:  models.StatusSuccess,
		Service: "TestService",
		API:     "TestAPI",
		Details: map[string]any{
			"version": "1.2.3",
		},
	}

	var got models.Response[map[string]any]
	err := json.Unmarshal(w.Body.Bytes(), &got)
	assert.NoError(t, err)

	// Validamos todos los campos
	assert.Equal(t, expected.Status, got.Status)
	assert.Equal(t, expected.Service, got.Service)
	assert.Equal(t, expected.API, got.API)
	assert.Equal(t, expected.Details["version"], got.Details["version"])
}
