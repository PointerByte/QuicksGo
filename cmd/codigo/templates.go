// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package codigo

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

	serverGin "github.com/PointerByte/QuicksGo/config/server_Gin"
	"github.com/gin-gonic/gin"
)

func main() {
	srv, err := serverGin.CreateApp()
	if err != nil {
		log.Fatal(err)
	}

	api := serverGin.GetRoute("/api/v1")
	if api == nil {
		log.Fatal("route group /api/v1 is not configured")
	}

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

	serverGRPC "github.com/PointerByte/QuicksGo/config/server_gRPC"
)

func main() {
	srv := serverGRPC.NewIUnitary(nil, nil)
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
      certFile: ./certs/server.crt
      keyFile: ./certs/server.key
      version: tlsv12
    mtls:
      enable: false
      clientCAFile: ./certs/ca.crt
      clientAuth: require_and_verify_client_cert

logger:
  dir: logs
  level: info
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

logger:
  dir: logs
  level: info

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
		},
	}

	if serviceType == serviceTypeGRPC {
		data["server"] = map[string]any{
			"grpc": map[string]any{
				"port": ":50051",
				"tls": map[string]any{
					"enable":   false,
					"certFile": "./certs/server.crt",
					"keyFile":  "./certs/server.key",
					"version":  "tlsv12",
				},
				"mtls": map[string]any{
					"enable":       false,
					"clientCAFile": "./certs/ca.crt",
					"clientAuth":   "require_and_verify_client_cert",
				},
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
	}

	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal application json: %w", err)
	}
	return string(payload) + "\n", nil
}
