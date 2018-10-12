package recordset

import (
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
)

type sourceClientMock struct{}

func (s *sourceClientMock) DescribeStacks(*cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	return nil, nil
}

func (s *sourceClientMock) ListStacks(*cloudformation.ListStacksInput) (*cloudformation.ListStacksOutput, error) {
	return nil, nil
}

func (s *sourceClientMock) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return nil, nil
}
func (s *sourceClientMock) DescribeLoadBalancers(*elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error) {
	return nil, nil
}

type targetClientMock struct {
	deletedStacks []string
}

func (s *targetClientMock) DescribeStacks(*cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	return nil, nil
}

func (t *targetClientMock) ListStacks(*cloudformation.ListStacksInput) (*cloudformation.ListStacksOutput, error) {
	return nil, nil
}

func (t *targetClientMock) CreateStack(*cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	return nil, nil
}

func (t *targetClientMock) DeleteStack(input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	t.deletedStacks = append(t.deletedStacks, *input.StackName)

	return nil, nil
}

func (t *targetClientMock) UpdateStack(*cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error) {
	return nil, nil
}
