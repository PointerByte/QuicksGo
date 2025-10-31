package security_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PointerByte/QuicksGo/security"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid host", func(t *testing.T) {
		// Configurar Viper con un host que NO matchea
		viper.Set("server.expectedHosts", []string{"allowed.com"})

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request, _ = http.NewRequest("GET", "/", nil)
		c.Request.Host = "evil.com"

		r.Use(security.SecurityHeaders())

		r.GET("/", func(c *gin.Context) {
			// No debe entrar acá
			t.Error("should have aborted due to invalid host")
		})

		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid host header")
	})

	t.Run("valid host and headers applied", func(t *testing.T) {
		// Configurar Viper con un host VÁLIDO
		viper.Set("server.expectedHosts", []string{"localhost"})

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request, _ = http.NewRequest("GET", "/", nil)
		c.Request.Host = "localhost"

		r.Use(security.SecurityHeaders())

		r.GET("/", func(c *gin.Context) {
			c.String(200, "OK")
		})

		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusOK, w.Code)

		headers := w.Result().Header

		assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
		assert.Contains(t, headers.Get("Content-Security-Policy"), "default-src")
		assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"))
		assert.Equal(t, "max-age=31536000; includeSubDomains", headers.Get("Strict-Transport-Security"))
		assert.Equal(t, "strict-origin", headers.Get("Referrer-Policy"))
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
		assert.Contains(t, headers.Get("Permissions-Policy"), "fullscreen")
	})
}
