package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/PointerByte/QuicksGo/logger"

	"github.com/PointerByte/QuicksGo/controller"

	"github.com/spf13/viper"
)

func main() {
	ctx := logger.New(context.Background())
	engine, err := CreateApp()
	if err != nil {
		ctx.Panic(err)
	}

	prefix := viper.GetStringSlice("server.basePaths")[0]
	route := GetRoute(prefix)
	route.GET("/status", controller.Status)

	srv := &http.Server{
		Addr:    viper.GetString("server.port"),
		Handler: engine,
	}
	ctx.Info(fmt.Sprintf("Server started on port %s", viper.GetString("server.port")))
	Start(srv)
}
