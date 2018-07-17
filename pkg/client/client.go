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

type StackLister interface {
	ListStacks(*cloudformation.ListStacksInput) (*cloudformation.ListStacksOutput, error)
}

type SourceInterface interface {
	StackLister
	DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	DescribeLoadBalancers(*elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error)
}

type TargetInterface interface {
	StackLister
	CreateStack(*cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
	DeleteStack(*cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error)
}

type Clients struct {
	*cloudformation.CloudFormation
	ec2iface.EC2API
	elbiface.ELBAPI
}

func NewClients(config *Config) *Clients {
	s := newSession(config)

	return &Clients{
		cloudformation.New(s),
		ec2.New(s),
		elb.New(s),
	}
}

func newSession(config *Config) *session.Session {
	awsCfg := &aws.Config{
		Credentials: credentials.NewStaticCredentials(config.AccessKeyID, config.AccessKeySecret, config.SessionToken),
		Region:      aws.String(config.Region),
	}
	return session.New(awsCfg)
}
