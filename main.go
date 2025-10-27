package main

import (
	"net/http"
	"quicksgo/controller"
	"quicksgo/logger"

	"github.com/spf13/viper"
)

func main() {
	engine, err := CreateApp()
	if err != nil {
		logger.Panic(ctxMain, err)
	}

	prefix := viper.GetStringSlice("server.basePaths")[0]
	route := GetRoute(prefix)
	route.GET("/status", controller.Status)

	srv := &http.Server{
		Addr:    viper.GetString("server.port"),
		Handler: engine,
	}
	Start(srv)
}
