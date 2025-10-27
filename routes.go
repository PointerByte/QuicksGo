package main

import "github.com/gin-gonic/gin"

var globalRoutes map[string]*gin.RouterGroup

func GetRoute(prefix string) *gin.RouterGroup {
	return globalRoutes[prefix]
}
