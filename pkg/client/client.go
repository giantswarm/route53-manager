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
	"github.com/aws/aws-sdk-go/service/route53"
)

type Config struct {
	AccessKeyID     string
	AccessKeySecret string
	SessionToken    string
	Region          string
}

type StackDescribeLister interface {
	DescribeStacks(*cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
	ListStacks(*cloudformation.ListStacksInput) (*cloudformation.ListStacksOutput, error)
}

type SourceInterface interface {
	StackDescribeLister
	DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	DescribeLoadBalancers(*elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error)
}

type TargetInterface interface {
	StackDescribeLister
	CreateStack(*cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
	ChangeResourceRecordSets(*route53.ChangeResourceRecordSetsInput) (*route53.ChangeResourceRecordSetsOutput, error)
	DeleteStack(*cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error)
	ListResourceRecordSets(*route53.ListResourceRecordSetsInput) (*route53.ListResourceRecordSetsOutput, error)
	UpdateStack(*cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error)
}

type Clients struct {
	*cloudformation.CloudFormation
	ec2iface.EC2API
	elbiface.ELBAPI
	*route53.Route53
}

func NewClients(config *Config) *Clients {
	s := newSession(config)

	return &Clients{
		cloudformation.New(s),
		ec2.New(s),
		elb.New(s),
		route53.New(s),
	}
}

func newSession(config *Config) *session.Session {
	awsCfg := &aws.Config{
		Credentials: credentials.NewStaticCredentials(config.AccessKeyID, config.AccessKeySecret, config.SessionToken),
		Region:      aws.String(config.Region),
	}
	s, err := session.NewSession(awsCfg)
	if err != nil {
		panic(err)
	}
	return s
}
