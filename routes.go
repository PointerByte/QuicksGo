package main

import "github.com/gin-gonic/gin"

var globalRoutes map[string]*gin.RouterGroup

// GetRoute returns the Gin route group associated with the given prefix.
// The prefix argument represents the base path used when the route group
// was registered. If no route group is found for the prefix, the function
// returns nil.
//
// Example:
//
//   r := GetRoute("/api")
//   r.GET("/status", statusHandler)
//
// This function is useful for retrieving globally registered route groups.
func GetRoute(prefix string) *gin.RouterGroup {
	return globalRoutes[prefix]
}
