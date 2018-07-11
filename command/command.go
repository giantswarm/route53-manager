package command

import (
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"

	"github.com/giantswarm/route53-manager/command/sync"
	"github.com/giantswarm/route53-manager/flag"
)

var (
	f = flag.New()
)

// Config represents the configuration used to create a new root command.
type Config struct {
	Logger micrologger.Logger

	Description string
	GitCommit   string
	Name        string
	Source      string
}

// New creates a new root command.
func New(config Config) (*Command, error) {
	var err error

	newCommand := &Command{
		cobraCommand: nil,
	}

	newCommand.cobraCommand = &cobra.Command{
		Use:   config.Name,
		Short: config.Description,
		Long:  config.Description,
		Run:   newCommand.Execute,
	}

	var syncCommand *sync.Command
	{
		c := sync.Config{
			Logger: config.Logger,
		}

		syncCommand, err = sync.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	newCommand.CobraCommand().AddCommand(syncCommand.CobraCommand())

	// Add config dirs and files so flags can be parsed from a config map.
	newCommand.cobraCommand.PersistentFlags().StringSlice(f.Config.Dirs, []string{"."}, "List of config file directories.")
	newCommand.cobraCommand.PersistentFlags().StringSlice(f.Config.Files, []string{"config"}, "List of the config file names. All viper supported extensions can be used.")

	return newCommand, nil
}

type Command struct {
	// Internals.
	cobraCommand *cobra.Command
}

func (c *Command) CobraCommand() *cobra.Command {
	return c.cobraCommand
}

func (c *Command) Execute(cmd *cobra.Command, args []string) {
	cmd.HelpFunc()(cmd, nil)
}
