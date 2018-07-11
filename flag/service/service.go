package service

import (
	"github.com/giantswarm/route53-manager/flag/service/source"
	"github.com/giantswarm/route53-manager/flag/service/target"
)

type Service struct {
	Source source.Source
	Target target.Target
}
