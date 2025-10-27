package main

import (
	"context"
	"fmt"
	"net/http"
	"quicksgo/controller"
	"quicksgo/logger"

	"github.com/spf13/viper"
)

func main() {
	ctx := logger.ContextLogger(context.Background())
	engine, err := CreateApp()
	if err != nil {
		logger.Panic(ctx, err)
	}

	prefix := viper.GetStringSlice("server.basePaths")[0]
	route := GetRoute(prefix)
	route.GET("/status", controller.Status)

	srv := &http.Server{
		Addr:    viper.GetString("server.port"),
		Handler: engine,
	}
	logger.Info(ctx, fmt.Sprintf("Server started on port %s", viper.GetString("server.port")))
	Start(srv)
}
