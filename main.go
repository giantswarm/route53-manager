package main

import (
	"log"
	"os"

	"github.com/giantswarm/micrologger"

	"github.com/giantswarm/route53-manager/pkg/client"
	"github.com/giantswarm/route53-manager/pkg/recordset"
)

func main() {
	logger, _ := micrologger.New(micrologger.Config{})

	targetClientConfig := &client.Config{
		AccessKeyID:     os.Getenv("TARGET_AWS_ACCESS_KEY_ID"),
		AccessKeySecret: os.Getenv("TARGET_AWS_SECRET_ACCESS_KEY"),
		Region:          os.Getenv("TARGET_REGION"),
	}
	sourceClientConfig := &client.Config{
		AccessKeyID:     os.Getenv("SOURCE_AWS_ACCESS_KEY_ID"),
		AccessKeySecret: os.Getenv("SOURCE_AWS_SECRET_ACCESS_KEY"),
		Region:          os.Getenv("SOURCE_REGION"),
	}

	c := &recordset.Config{
		Logger:       logger,
		SourceClient: client.NewClients(sourceClientConfig),
		TargetClient: client.NewClients(targetClientConfig),

		TargetHostedZoneID:   os.Getenv("TARGET_HOSTEDZONE_ID"),
		TargetHostedZoneName: os.Getenv("TARGET_HOSTEDZONE_NAME"),
	}

	m, err := recordset.NewManager(c)
	if err != nil {
		log.Fatalf("could not create recordset manager %v", err)
	}

	err = m.Sync()
	if err != nil {
		log.Fatalf("could not sync recordsets %v", err)
	}
}
