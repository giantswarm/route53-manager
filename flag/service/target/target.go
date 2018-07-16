package target

import (
	"github.com/giantswarm/route53-manager/flag/service/access"
	"github.com/giantswarm/route53-manager/flag/service/target/hostedzone"
)

type Target struct {
	access.Config
	HostedZone hostedzone.Config
}
