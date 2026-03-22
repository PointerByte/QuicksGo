package codigo

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	serviceTypeGin  = "gin"
	serviceTypeGRPC = "grpc"
)

type newCommand struct {
	app *App
}

type newServiceCommand struct {
	app         *App
	serviceType string
	options     *scaffoldOptions
}

func newNewCommand(app *App) Command {
	return &newCommand{app: app}
}

func newServiceGeneratorCommand(app *App, serviceType string) Command {
	return &newServiceCommand{
		app:         app,
		serviceType: serviceType,
		options:     &scaffoldOptions{},
	}
}

func (command *newCommand) Cobra() *cobra.Command {
	newCmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new QuicksGo service",
	}

	newCmd.AddCommand(newServiceGeneratorCommand(command.app, serviceTypeGin).Cobra())
	newCmd.AddCommand(newServiceGeneratorCommand(command.app, serviceTypeGRPC).Cobra())
	return newCmd
}

func (command *newServiceCommand) Cobra() *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   command.serviceType,
		Short: fmt.Sprintf("Create a new %s service", strings.ToUpper(command.serviceType)),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedOptions, err := resolveScaffoldOptions(command.app.streams, command.options)
			if err != nil {
				return err
			}

			scaffolder := newScaffolder(command.app.streams, command.app.runner)
			return scaffolder.createService(command.serviceType, resolvedOptions)
		},
	}

	cobraCmd.Flags().StringVarP(&command.options.modulePath, "module", "m", "", "Go module/package name for the new service")
	cobraCmd.Flags().StringVarP(&command.options.appName, "app-name", "a", "", "Value for app.name in application config")
	cobraCmd.Flags().StringVarP(&command.options.configFormat, "config-format", "c", "", "Configuration format: yaml or json")
	cobraCmd.Flags().StringVarP(&command.options.outputDir, "dir", "d", "", "Output directory for the generated service")
	return cobraCmd
}
