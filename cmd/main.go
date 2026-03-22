package main

import (
	"fmt"
	"os"

	"github.com/PointerByte/QuicksGo/cmd/qgo/codigo"
)

var (
	newAppFn      = codigo.NewApp
	executeAppFn  = executeApp
	writeErrorFn  = writeError
	exitProcessFn = os.Exit
)

func main() {
	if err := executeAppFn(newAppFn()); err != nil {
		writeErrorFn(err.Error())
		exitProcessFn(1)
	}
}

func executeApp(app *codigo.App) error {
	return app.Execute()
}

func writeError(message string) {
	fmt.Fprintln(os.Stderr, message)
}
