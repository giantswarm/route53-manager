package main

import (
	"fmt"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"

	"github.com/giantswarm/route53-manager/command"
)

var (
	description = "Automation for managing Route53 recordsets."
	gitCommit   = "n/a"
	name        = "route53-manager"
	source      = "https://github.com/giantswarm/route53-manager"
)

func main() {
	err := mainWithError()
	if err != nil {
		panic(fmt.Sprintf("%#v\n", err))
	}
}

func mainWithError() (err error) {
	var newLogger micrologger.Logger
	{
		newLogger, err = micrologger.New(micrologger.Config{})
		if err != nil {
			return microerror.Maskf(err, "micrologger.New")
		}
	}

	var newCommand *command.Command
	{
		c := command.Config{
			Logger: newLogger,

			Description: description,
			GitCommit:   gitCommit,
			Name:        name,
			Source:      source,
		}

		newCommand, err = command.New(c)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	newCommand.CobraCommand().Execute()

	return nil
}
