package recordset

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/route53"
)

type sourceClientMock struct {
	sourceStacks []cloudformation.Stack
}

func newSourceWithStacks(stacks []cloudformation.Stack) *sourceClientMock {
	return &sourceClientMock{
		sourceStacks: stacks,
	}
}

func (s *sourceClientMock) DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	if s == nil || input == nil || input.StackName == nil {
		return nil, mockClientError
	}

	for _, stack := range s.sourceStacks {
		if stack.StackName != nil && *stack.StackName == *input.StackName {
			output := &cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					&stack,
				},
			}

			return output, nil
		}
	}

	return nil, mockClientError
}

func (s *sourceClientMock) ListStacks(input *cloudformation.ListStacksInput) (*cloudformation.ListStacksOutput, error) {
	if s == nil {
		return nil, mockClientError
	}

	filters := []string{}
	if input != nil {
		for _, f := range input.StackStatusFilter {
			if f != nil {
				filters = append(filters, *f)
			}
		}
	}

	output := &cloudformation.ListStacksOutput{}
	for _, stack := range s.sourceStacks {
		add := false
		if len(filters) > 0 && stack.StackStatus != nil {
			for _, f := range filters {
				if f == *stack.StackStatus {
					add = true
				}
			}
		} else {
			add = true
		}

		if add {
			s := &cloudformation.StackSummary{
				StackId:     stack.StackName,
				StackName:   stack.StackName,
				StackStatus: stack.StackStatus,
			}
			output.StackSummaries = append(output.StackSummaries, s)
		}
	}

	return output, nil
}

func (s *sourceClientMock) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	output := &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			&ec2.Reservation{
				Instances: []*ec2.Instance{
					&ec2.Instance{
						PrivateDnsName: aws.String("ec2.dns.test"),
					},
				},
			},
		},
	}
	return output, nil
}
func (s *sourceClientMock) DescribeLoadBalancers(*elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error) {
	output := &elb.DescribeLoadBalancersOutput{
		LoadBalancerDescriptions: []*elb.LoadBalancerDescription{
			&elb.LoadBalancerDescription{
				DNSName: aws.String("elb.dns.test"),
			},
		},
	}

	return output, nil
}
func (s *sourceClientMock) DescribeNetworkInterfaces(input *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
	output := &ec2.DescribeNetworkInterfacesOutput{
		NetworkInterfaces: []*ec2.NetworkInterface{
			&ec2.NetworkInterface{
				PrivateIpAddress: aws.String("10.1.0.1"),
			},
		},
	}

	return output, nil
}

type targetClientMock struct {
	createdStacks []string
	deletedStacks []string
	updatedStacks []string
	targetStacks  []cloudformation.Stack
}

func newTargetWithStacks(stacks []cloudformation.Stack) *targetClientMock {
	return &targetClientMock{
		targetStacks: stacks,
	}
}
func (t *targetClientMock) DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	if t == nil || input == nil || input.StackName == nil {
		return nil, mockClientError
	}

	for _, stack := range t.targetStacks {
		if stack.StackName != nil && *stack.StackName == *input.StackName {
			output := &cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					&stack,
				},
			}

			return output, nil
		}
	}

	return nil, mockClientError
}

func (t *targetClientMock) ListResourceRecordSets(input *route53.ListResourceRecordSetsInput) (*route53.ListResourceRecordSetsOutput, error) {
	if t == nil {
		return nil, mockClientError
	}

	output := &route53.ListResourceRecordSetsOutput{}

	return output, nil
}

func (t *targetClientMock) ListStacks(input *cloudformation.ListStacksInput) (*cloudformation.ListStacksOutput, error) {
	if t == nil {
		return nil, mockClientError
	}

	filters := []string{}
	if input != nil {
		for _, f := range input.StackStatusFilter {
			if f != nil {
				filters = append(filters, *f)
			}
		}
	}

	output := &cloudformation.ListStacksOutput{}
	for _, stack := range t.targetStacks {
		add := false
		if len(filters) > 0 && stack.StackStatus != nil {
			for _, f := range filters {
				if f == *stack.StackStatus {
					add = true
				}
			}
		} else {
			add = true
		}

		if add {
			s := &cloudformation.StackSummary{
				StackId:     stack.StackName,
				StackName:   stack.StackName,
				StackStatus: stack.StackStatus,
			}
			output.StackSummaries = append(output.StackSummaries, s)
		}
	}

	return output, nil
}

func (t *targetClientMock) ChangeResourceRecordSets(input *route53.ChangeResourceRecordSetsInput) (*route53.ChangeResourceRecordSetsOutput, error) {
	if t == nil {
		return nil, mockClientError
	}

	output := &route53.ChangeResourceRecordSetsOutput{}

	return output, nil
}

func (t *targetClientMock) CreateStack(input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	if input == nil || input.StackName == nil {
		return nil, mockClientError
	}

	t.createdStacks = append(t.createdStacks, *input.StackName)

	return nil, nil
}

func (t *targetClientMock) DeleteStack(input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	if input == nil || input.StackName == nil {
		return nil, mockClientError
	}

	t.deletedStacks = append(t.deletedStacks, *input.StackName)

	return nil, nil
}

func (t *targetClientMock) UpdateStack(input *cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error) {
	if input == nil || input.StackName == nil {
		return nil, mockClientError
	}

	t.updatedStacks = append(t.updatedStacks, *input.StackName)

	return nil, nil
}
