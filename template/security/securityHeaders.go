package security

import (
	"net/http"
	"slices"

	"github.com/PointerByte/QuicksGo/models"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// SecurityHeaders returns a Gin middleware that applies common HTTP security headers
// to each response and validates the Host header against a configured whitelist.
//
// It performs the following:
//
//   - Validates the request's Host header against `server.expectedHosts` from Viper config.
//     If the host is not in the allowed list, the request is aborted with a 400 Bad Request.
//
//   - Adds security headers including:
//
//   - X-Frame-Options: Prevents clickjacking by disallowing framing.
//
//   - Content-Security-Policy: Restricts the sources for scripts, styles, images, etc.
//
//   - X-XSS-Protection: Enables cross-site scripting (XSS) filter.
//
//   - Strict-Transport-Security: Enforces HTTPS for future requests.
//
//   - Referrer-Policy: Controls what referrer information is included with requests.
//
//   - X-Content-Type-Options: Prevents MIME-type sniffing.
//
//   - Permissions-Policy: Disables access to certain browser features.
//
// This middleware should be included early in the middleware chain to ensure headers
// are applied to all valid requests.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		expectedHosts := viper.GetStringSlice("server.expectedHosts")
		if !slices.Contains(expectedHosts, c.Request.Host) {
			resp := models.GenericResponse[map[string]any](models.StatusError, gin.H{"error": "Invalid host header"})
			c.AbortWithStatusJSON(http.StatusBadRequest, resp)
			return
		}
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "default-src 'self'; connect-src *; font-src *; script-src-elem * 'unsafe-inline'; img-src * data:; style-src * 'unsafe-inline';")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Referrer-Policy", "strict-origin")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Permissions-Policy", "geolocation=(),midi=(),sync-xhr=(),microphone=(),camera=(),magnetometer=(),gyroscope=(),fullscreen=(self),payment=()")
		c.Next()
	}
}
