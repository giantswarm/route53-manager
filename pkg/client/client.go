package client

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elb/elbiface"
)

type Config struct {
	AccessKeyID     string
	AccessKeySecret string
	SessionToken    string
	Region          string
}

type Clients struct {
	CloudFormation *cloudformation.CloudFormation
	EC2            ec2iface.EC2API
	ELB            elbiface.ELBAPI
}

func NewClients(config *Config) *Clients {
	s := newSession(config)

	return &Clients{
		CloudFormation: cloudformation.New(s),
		EC2:            ec2.New(s),
		ELB:            elb.New(s),
	}
}

func newSession(config *Config) *session.Session {
	awsCfg := &aws.Config{
		Credentials: credentials.NewStaticCredentials(config.AccessKeyID, config.AccessKeySecret, config.SessionToken),
		Region:      aws.String(config.Region),
	}
	return session.New(awsCfg)
}
