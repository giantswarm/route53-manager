package sync

import (
	"fmt"
	"log"
	"os"

	"github.com/giantswarm/microerror"
	microflag "github.com/giantswarm/microkit/flag"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/giantswarm/route53-manager/flag"
	"github.com/giantswarm/route53-manager/pkg/client"
	"github.com/giantswarm/route53-manager/pkg/recordset"
)

var (
	f = flag.New()
)

type Config struct {
	Logger micrologger.Logger

	Viper *viper.Viper
}

func New(config Config) (*Command, error) {
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", config)
	}
	if config.Viper == nil {
		config.Viper = viper.New()
	}

	newCommand := &Command{
		logger: config.Logger,

		cobraCommand: nil,

		viper: config.Viper,
	}

	newCommand.cobraCommand = &cobra.Command{
		Use:   "sync",
		Short: "Synchronize recordsets between AWS accounts.",
		Long:  "Creates, deletes and updates recordsets on a target AWS account related to resources on a source AWS account.",
		Run:   newCommand.Execute,
	}

	newCommand.cobraCommand.PersistentFlags().String(f.Service.Installation.Name, "", "Installation name")

	newCommand.cobraCommand.PersistentFlags().String(f.Service.Source.AccessKey, "", "Source account access key")
	newCommand.cobraCommand.PersistentFlags().String(f.Service.Source.SecretAccessKey, "", "Source account secret access key")
	newCommand.cobraCommand.PersistentFlags().String(f.Service.Source.Region, "", "Source account region")

	newCommand.cobraCommand.PersistentFlags().String(f.Service.Target.AccessKey, "", "Target account access key")
	newCommand.cobraCommand.PersistentFlags().String(f.Service.Target.SecretAccessKey, "", "Target account secret access key")
	newCommand.cobraCommand.PersistentFlags().String(f.Service.Target.Region, "", "Target account region")
	newCommand.cobraCommand.PersistentFlags().String(f.Service.Target.HostedZone.Name, "", "Target account Hosted Zone name")
	newCommand.cobraCommand.PersistentFlags().String(f.Service.Target.HostedZone.ID, "", "Target account Hosted Zone ID")

	return newCommand, nil
}

type Command struct {
	logger micrologger.Logger

	cobraCommand *cobra.Command

	viper *viper.Viper
}

func (c *Command) CobraCommand() *cobra.Command {
	return c.cobraCommand
}

func (c *Command) Execute(cmd *cobra.Command, args []string) {
	// We have to parse the flags given via command line first. Only that way we
	// are able to use the flag configuration for the location of configuration
	// directories and files in the next step below.
	microflag.Parse(c.viper, cmd.Flags())

	// Merge the given command line flags with the given environment variables and
	// the given config files, if any. The merged flags will be applied to the
	// given viper.
	err := microflag.Merge(c.viper, cmd.Flags(), c.viper.GetStringSlice(f.Config.Dirs), c.viper.GetStringSlice(f.Config.Files))
	if err != nil {
		panic(err)
	}

	err = c.execute()
	if err != nil {
		c.logger.Log("level", "error", "message", fmt.Sprintf("command %#q failed", cmd.Name()), "stack", microerror.JSON(microerror.Mask(err)), "verbosity", 0)
		os.Exit(1)
	}
}

func (c *Command) execute() error {
	installationName := c.viper.GetString(f.Service.Installation.Name)

	targetClientConfig := &client.Config{
		AccessKeyID:     c.viper.GetString(f.Service.Target.AccessKey),
		AccessKeySecret: c.viper.GetString(f.Service.Target.SecretAccessKey),
		Region:          c.viper.GetString(f.Service.Target.Region),
	}
	sourceClientConfig := &client.Config{
		AccessKeyID:     c.viper.GetString(f.Service.Source.AccessKey),
		AccessKeySecret: c.viper.GetString(f.Service.Source.SecretAccessKey),
		Region:          c.viper.GetString(f.Service.Source.Region),
	}

	cfg := &recordset.Config{
		Logger:       c.logger,
		Installation: installationName,
		SourceClient: client.NewClients(sourceClientConfig),
		TargetClient: client.NewClients(targetClientConfig),

		TargetHostedZoneID:   c.viper.GetString(f.Service.Target.HostedZone.ID),
		TargetHostedZoneName: c.viper.GetString(f.Service.Target.HostedZone.Name),
	}

	m, err := recordset.NewManager(cfg)
	if err != nil {
		log.Fatalf("could not create recordset manager %v", err)
	}

	err = m.Sync()
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
