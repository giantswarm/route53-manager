package flag

import (
	"github.com/giantswarm/microkit/flag"

	"github.com/giantswarm/route53-manager/flag/config"
	"github.com/giantswarm/route53-manager/flag/service"
)

type Flag struct {
	Config  config.Config
	Service service.Service
}

func New() *Flag {
	f := &Flag{}
	flag.Init(f)
	return f
}
