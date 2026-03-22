package codigo

import (
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// IOStreams groups the standard input, output, and error streams used by the CLI.
type IOStreams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// Command defines an executable CLI command that can be converted into a Cobra command.
type Command interface {
	Cobra() *cobra.Command
}

// App wires the qgo CLI, its command tree, and the dependencies required to scaffold services.
type App struct {
	streams IOStreams
	runner  goRunner
}

type goRunner func(dir string, args ...string) error

// NewApp creates the default qgo application ready to execute from main.
func NewApp() *App {
	return &App{
		streams: IOStreams{
			In:  os.Stdin,
			Out: os.Stdout,
			Err: os.Stderr,
		},
		runner: func(dir string, args ...string) error {
			command := exec.Command("go", args...)
			command.Dir = dir
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			return command.Run()
		},
	}
}

// Execute runs the root qgo command tree.
func (app *App) Execute() error {
	return app.rootCommand().Execute()
}

func (app *App) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "qgo",
		Short:         "QuicksGo service generator",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newNewCommand(app).Cobra())
	return root
}
