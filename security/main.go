// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
	"github.com/PointerByte/QuicksGo/security/middlewares"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// No use in production!
const demoJWTSecret = "oXPZp-Y9yu2zmfECMU*_"

const (
	hmacAlgorithmKey = "jwt.hmac.algorithm"
	hmacSecretKey    = jwtservice.DefaultHMACSecretKey
	rsaAlgorithmKey  = "jwt.rsa.algorithm"
	rsaPrivateKeyKey = jwtservice.DefaultRSAPrivateKeyKey
	rsaPublicKeyKey  = jwtservice.DefaultRSAPublicKeyKey
)

// Example requests:
//
//  1. Start the server:
//     go run .
//
//  2. Request an HMAC token:
//     curl -X POST http://localhost:8080/hmac/login ^
//     -H "Content-Type: application/json" ^
//     -d "{\"user_id\":\"42\",\"role\":\"admin\"}"
//
//  3. Call an HMAC protected endpoint with the returned token:
//     curl http://localhost:8080/hmac/api/me ^
//     -H "Authorization: Bearer <HMAC_TOKEN>"
//
//  4. Request an RSA token:
//     curl -X POST http://localhost:8080/rsa/login ^
//     -H "Content-Type: application/json" ^
//     -d "{\"user_id\":\"42\",\"role\":\"admin\"}"
//
//  5. Call an RSA protected endpoint with the returned token:
//     curl http://localhost:8080/rsa/api/admin ^
//     -H "Authorization: Bearer <RSA_TOKEN>"
//
//  6. Try a blocked user to see the extra validator reject the token:
//     curl -X POST http://localhost:8080/hmac/login ^
//     -H "Content-Type: application/json" ^
//     -d "{\"user_id\":\"blocked-user\",\"role\":\"admin\"}"
//
//     Then use that token on /hmac/api/me or /rsa/api/me and the validator
//     validateActiveSession will reject the request.
type loginRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"`
}

type sessionClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

var (
	runRouterFn = func(router *gin.Engine) error {
		return router.Run(":8080")
	}
	logFatalfFn = log.Fatalf
)

func main() {
	if err := runApp(); err != nil {
		logFatalfFn("application startup failed: %v", err)
	}
}

func runApp() error {
	configureViper()

	if err := ensureDefaultHMACSecret(); err != nil {
		return fmt.Errorf("prepare hmac config: %w", err)
	}

	if err := ensureDefaultRSAKeys(); err != nil {
		return fmt.Errorf("prepare rsa config: %w", err)
	}

	router := newRouter()
	if err := runRouterFn(router); err != nil {
		return fmt.Errorf("run gin server: %w", err)
	}
	return nil
}

func ensureDefaultHMACSecret() error {
	if viper.GetString(hmacSecretKey) != "" {
		return nil
	}
	viper.Set(hmacSecretKey, demoJWTSecret)
	return nil
}

func ensureDefaultRSAKeys() error {
	if viper.GetString(rsaPrivateKeyKey) != "" && viper.GetString(rsaPublicKeyKey) != "" {
		return nil
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}

	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}

	viper.Set(rsaPrivateKeyKey, base64.StdEncoding.EncodeToString(privateDER))
	viper.Set(rsaPublicKeyKey, base64.StdEncoding.EncodeToString(publicDER))
	return nil
}

func configureViper() {
	viper.SetDefault(hmacAlgorithmKey, "HS256")
	viper.SetDefault(rsaAlgorithmKey, "RS256")
}

func newRouter() *gin.Engine {
	configureViper()

	router := gin.Default()
	router.Use(middlewares.SecurityHeaders())

	router.GET("/health", healthHandler(""))
	registerJWTExampleRoutes(router.Group("/hmac"))
	registerJWTExampleRoutes(router.Group("/rsa"))
	return router
}

func registerJWTExampleRoutes(group *gin.RouterGroup) {
	jwtService, exampleName := jwtExampleServiceAndName(group.BasePath())

	group.GET("/health", healthHandler(exampleName))
	group.POST("/login", loginHandler(jwtService))

	protected := group.Group("/api")
	protected.Use(middlewares.RequireJWT(
		middlewares.WithJWTServiceConfig(jwtConfigForExample(exampleName)),
		middlewares.WithJWTClaimsFactory(func() any { return &sessionClaims{} }),
		middlewares.WithJWTValidator(validateActiveSession),
	))
	protected.GET("/me", meHandler(exampleName))
	protected.GET("/admin", adminHandler(exampleName))
}

func jwtExampleServiceAndName(basePath string) (*jwtservice.Service, string) {
	config := jwtConfigForExample(basePath)
	service, err := jwtservice.NewConfiguredService(config)
	if err != nil {
		panic(fmt.Sprintf("build jwt service for %s: %v", basePath, err))
	}

	if strings.Contains(strings.ToLower(basePath), "rsa") {
		return service, "RSA / RS256"
	}
	return service, "HMAC / HS256"
}

func jwtConfigForExample(basePath string) jwtservice.ConfigServiceInput {
	if strings.Contains(strings.ToLower(basePath), "rsa") {
		return jwtservice.ConfigServiceInput{
			Algorithm: "RS256",
		}
	}

	return jwtservice.ConfigServiceInput{
		Algorithm: "HS256",
	}
}

func healthHandler(exampleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		response := gin.H{"status": "ok"}
		if exampleName != "" {
			response["example"] = exampleName
		}
		c.JSON(http.StatusOK, response)
	}
}

func loginHandler(jwtService *jwtservice.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request loginRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid login payload",
			})
			return
		}

		token, err := jwtService.Create(sessionClaims{
			UserID: request.UserID,
			Role:   request.Role,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not create token",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"token": token,
		})
	}
}

func meHandler(exampleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := claimsFromContext(c)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"example": exampleName,
			"user_id": claims.UserID,
			"role":    claims.Role,
		})
	}
}

func adminHandler(exampleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := claimsFromContext(c)
		if !ok {
			return
		}

		if claims.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "admin role required",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"example": exampleName,
			"message": "welcome admin",
		})
	}
}

func claimsFromContext(c *gin.Context) (*sessionClaims, bool) {
	claimsValue, _ := c.Get(middlewares.JWTClaimsContextKey.String())
	claims, ok := claimsValue.(*sessionClaims)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "claims not available in context",
		})
		return nil, false
	}
	return claims, true
}

func validateActiveSession(ctx context.Context, token jwtservice.Token) error {
	var claims sessionClaims
	if err := json.Unmarshal(token.Claims, &claims); err != nil {
		return err
	}

	// Example of an extra validation hook, such as a database lookup.
	if claims.UserID == "blocked-user" {
		return errors.New("user session is blocked")
	}
	return nil
}
