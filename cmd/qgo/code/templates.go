// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package code

import (
	"encoding/json"
	"fmt"
)

// buildProjectFiles returns the generated file set for the requested service type and config format.
func buildProjectFiles(serviceType string, options scaffoldOptions) (map[string]string, error) {
	files := map[string]string{
		"main.go": buildMainTemplate(serviceType, options.appName),
	}

	switch options.configFormat {
	case configYAML:
		files["application.yaml"] = buildApplicationYAML(serviceType, options.appName)
	case configJSON:
		content, err := buildApplicationJSON(serviceType, options.appName)
		if err != nil {
			return nil, err
		}
		files["application.json"] = content
	default:
		return nil, fmt.Errorf("unsupported config format %q", options.configFormat)
	}

	return files, nil
}

// buildMainTemplate renders the starter main.go for the selected transport.
func buildMainTemplate(serviceType string, appName string) string {
	switch serviceType {
	case serviceTypeGin:
		return fmt.Sprintf(`package main

import (
	"log"

	serverGin "github.com/PointerByte/QuicksGo/config/server/gin"
	"github.com/gin-gonic/gin"
)

func main() {
	srv, err := serverGin.CreateApp()
	if err != nil {
		log.Fatal(err)
	}

	api := serverGin.GetRoute("/api/v1")
	api.GET("/hello", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"app":     "%s",
			"message": "hello from QuicksGo Gin",
		})
	})

	serverGin.Start(srv)
}
`, appName)
	case serviceTypeGRPC:
		return `package main

import (
	"log"

	serverGRPC "github.com/PointerByte/QuicksGo/config/server/grpc"
)

func main() {
	srv := serverGRPC.NewIConfig(nil, nil)
	if err := srv.Serve(); err != nil {
		log.Fatal(err)
	}
}
`
	default:
		return ""
	}
}

// buildApplicationYAML renders the default YAML configuration for the generated service.
func buildApplicationYAML(serviceType string, appName string) string {
	if serviceType == serviceTypeGRPC {
		return fmt.Sprintf(`app:
  name: %s
  version: 0.0.1

server:
  grpc:
    port: ":50051"
    tls:
      enable: false
      certFile: ./certs/cert.pem
      keyFile: ./certs/key.pem
      version: tlsv12
    mtls:
      enable: false
      clientCAFile: ./certs/ca.pem
      clientAuth: require_and_verify_client_cert

client:
  grpc:
    tls:
      enable: false
      caFile: ./certs/ca.pem
      serverName: localhost
      version: tlsv12
      insecureSkipVerify: false
    mtls:
      enable: false
      certFile: ./certs/cert.pem
      keyFile: ./certs/key.pem

logger:
  dir: logs
  level: info
  ignoredHeaders:
    - Authorization
    - Cookie
  formatter: json
  formatDate: "2006-01-02T15:04:05.000"
  rotate:
    enable: true
    maxSize: 10
    maxBackups: 5
    maxAge: 30
    compress: true

traces:
  SkipPaths:
    - /health

jwt:
  enable: false
  transport: header
  algorithm: HS256
  hmac:
    secret: change-me-hmac-secret
  rsa:
    private_key: ""
    public_key: ""
  eddsa:
    private_key: ""
    public_key: ""
`, appName)
	}

	return fmt.Sprintf(`app:
  name: %s
  version: 0.0.1

server:
  groups:
    - /api/v1
  gin:
    port: ":8080"
    mode: release
    UseH2C: true
    rate:
      limit: 1000
      burst: 2000
    LoggerWithConfig:
      enabled: true
      SkipPaths:
        - /api/v1/health
      SkipQueryString: false

gin:
  autotls:
    enable: false
    domain: api.example.com
    dirCache: ./certs
    version: tlsv13

client:
  grpc:
    tls:
      enable: false
      caFile: ./certs/ca.pem
      serverName: localhost
      version: tlsv12
      insecureSkipVerify: false
    mtls:
      enable: false
      certFile: ./certs/cert.pem
      keyFile: ./certs/key.pem

logger:
  dir: logs
  level: info
  ignoredHeaders:
    - Authorization
    - Cookie
  formatter: json
  formatDate: "2006-01-02T15:04:05.000"
  rotate:
    enable: true
    maxSize: 10
    maxBackups: 5
    maxAge: 30
    compress: true

traces:
  SkipPaths:
    - /api/v1/health

jwt:
  enable: false
  transport: header
  algorithm: HS256
  hmac:
    secret: change-me-hmac-secret
`, appName)
}

// buildApplicationJSON renders the default JSON configuration for the generated service.
func buildApplicationJSON(serviceType string, appName string) (string, error) {
	data := map[string]any{
		"app": map[string]any{
			"name":    appName,
			"version": "0.0.1",
		},
		"logger": map[string]any{
			"dir":   "logs",
			"level": "info",
			"ignoredHeaders": []string{
				"Authorization",
				"Cookie",
			},
			"formatter":  "json",
			"formatDate": "2006-01-02T15:04:05.000",
			"rotate": map[string]any{
				"enable":     true,
				"maxSize":    10,
				"maxBackups": 5,
				"maxAge":     30,
				"compress":   true,
			},
		},
	}

	if serviceType == serviceTypeGRPC {
		data["server"] = map[string]any{
			"grpc": map[string]any{
				"port": ":50051",
				"tls": map[string]any{
					"enable":   false,
					"certFile": "./certs/server.pem",
					"keyFile":  "./certs/server.pem",
					"version":  "tlsv12",
				},
				"mtls": map[string]any{
					"enable":       false,
					"clientCAFile": "./certs/ca.pem",
					"clientAuth":   "require_and_verify_client_cert",
				},
			},
		}
		data["client"] = map[string]any{
			"grpc": map[string]any{
				"tls": map[string]any{
					"enable":             false,
					"caFile":             "./certs/ca.pem",
					"serverName":         "localhost",
					"version":            "tlsv12",
					"insecureSkipVerify": false,
				},
				"mtls": map[string]any{
					"enable":   false,
					"certFile": "./certs/cert.pem",
					"keyFile":  "./certs/key.pem",
				},
			},
		}
		data["traces"] = map[string]any{
			"SkipPaths": []string{
				"/health",
				"/refresh",
			},
		}
		data["jwt"] = map[string]any{
			"enable":    false,
			"transport": "header",
			"algorithm": "HS256",
			"hmac": map[string]any{
				"secret": "change-me-hmac-secret",
			},
		}
	} else {
		data["server"] = map[string]any{
			"groups": []string{"/api/v1"},
			"gin": map[string]any{
				"port":   ":8080",
				"mode":   "release",
				"UseH2C": true,
				"rate": map[string]any{
					"limit": 1000,
					"burst": 2000,
				},
				"LoggerWithConfig": map[string]any{
					"enabled": true,
					"SkipPaths": []string{
						"/api/v1/health",
					},
					"SkipQueryString": false,
				},
			},
		}
		data["gin"] = map[string]any{
			"autotls": map[string]any{
				"enable":   false,
				"domain":   "api.example.com",
				"dirCache": "./certs",
				"version":  "tlsv13",
			},
		}
		data["client"] = map[string]any{
			"grpc": map[string]any{
				"tls": map[string]any{
					"enable":             false,
					"caFile":             "./certs/ca.pem",
					"serverName":         "localhost",
					"version":            "tlsv12",
					"insecureSkipVerify": false,
				},
				"mtls": map[string]any{
					"enable":   false,
					"certFile": "./certs/cert.pem",
					"keyFile":  "./certs/key.pem",
				},
			},
		}
		data["traces"] = map[string]any{
			"SkipPaths": []string{
				"/api/v1/health",
				"/api/v1/refresh",
			},
		}
		data["jwt"] = map[string]any{
			"enable":    false,
			"transport": "header",
			"algorithm": "HS256",
			"hmac": map[string]any{
				"secret": "change-me-hmac-secret",
			},
			"rsa": map[string]any{
				"private_key": "./certs/jwt/key.pem",
				"public_key":  "./certs/jwt/public.pem",
			},
			"eddsa": map[string]any{
				"private_key": "./certs/jwt/key.pem",
				"public_key":  "./certs/jwt/public.pem",
			},
		}
	}

	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal application json: %w", err)
	}
	return string(payload) + "\n", nil
}
