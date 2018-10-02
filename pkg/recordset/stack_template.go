package recordset

import (
	"bytes"
	"html/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/giantswarm/microerror"
)

const (
	targetStackTemplate = `AWSTemplateFormatVersion: 2010-09-09
Description: Recordset Guest CloudFormation stack.
Resources:
  ingressDNSRecord:
    Type: AWS::Route53::RecordSet
    Properties:
      HostedZoneId: {{ .HostedZoneID }}
      Name: 'ingress.{{ .ClusterName }}.{{ .HostedZoneName }}'
      Type: CNAME
      TTL: '900'
      ResourceRecords:
      - {{ .IngressELBDNS }}

  ingressWildcardDNSRecord:
    Type: AWS::Route53::RecordSet
    Properties:
      HostedZoneId: {{ .HostedZoneID }}
      Name: '*.{{ .ClusterName }}.{{ .HostedZoneName }}'
      Type: CNAME
      TTL: '900'
      ResourceRecords:
      - {{ .IngressELBDNS }}

  apiDNSRecord:
    Type: AWS::Route53::RecordSet
    Properties:
      HostedZoneId: {{ .HostedZoneID }}
      Name: 'api.{{ .ClusterName }}.{{ .HostedZoneName }}'
      Type: CNAME
      TTL: '900'
      ResourceRecords:
      - {{ .APIELBDNS }}

  etcdDNSRecord:
    Type: AWS::Route53::RecordSet
    Properties:
      HostedZoneId: {{ .HostedZoneID }}
      Name: 'etcd.{{ .ClusterName }}.{{ .HostedZoneName }}'
      Type: CNAME
      TTL: '900'
      ResourceRecords:
      - {{ .EtcdInstanceDNS }}
`
)

func (m *Manager) getCreateStackInput(targetStackName string, data *sourceStackData, sourceStack cloudformation.Stack) (*cloudformation.CreateStackInput, error) {
	templateBody, err := m.getStackTemplateBody(data)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	input := &cloudformation.CreateStackInput{
		StackName:        aws.String(targetStackName),
		TemplateBody:     aws.String(templateBody),
		TimeoutInMinutes: aws.Int64(2),
	}

	return input, nil
}

func (m *Manager) getUpdateStackInput(targetStackName string, data *sourceStackData, sourceStack cloudformation.Stack) (*cloudformation.UpdateStackInput, error) {
	templateBody, err := m.getStackTemplateBody(data)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	input := &cloudformation.UpdateStackInput{
		StackName:    aws.String(targetStackName),
		TemplateBody: aws.String(templateBody),
	}

	return input, nil
}

func (m *Manager) getStackTemplateBody(data *sourceStackData) (string, error) {
	tmpl, err := template.New("recordsets").Parse(targetStackTemplate)
	if err != nil {
		return "", microerror.Mask(err)
	}

	var templateBody bytes.Buffer
	err = tmpl.Execute(&templateBody, data)
	if err != nil {
		return "", microerror.Mask(err)
	}

	return templateBody.String(), nil
}

func (m *Manager) getSourceStackData(clusterName string) (*sourceStackData, error) {
	ingressELBName := clusterName + "-ingress"
	ingressELBDNS, err := m.getELBDNS(ingressELBName)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	apiELBName := clusterName + "-api"
	apiELBDNS, err := m.getELBDNS(apiELBName)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	etcInstanceNameTag := clusterName + "-master"
	etcdInstanceDNS, err := m.getInstanceDNS(etcInstanceNameTag)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	output := &sourceStackData{
		HostedZoneID:    m.targetHostedZoneID,
		HostedZoneName:  m.targetHostedZoneName,
		ClusterName:     clusterName,
		IngressELBDNS:   ingressELBDNS,
		APIELBDNS:       apiELBDNS,
		EtcdInstanceDNS: etcdInstanceDNS,
	}
	return output, nil
}

func (m *Manager) getELBDNS(elbName string) (string, error) {
	input := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{
			aws.String(elbName),
		},
	}
	output, err := m.sourceClient.DescribeLoadBalancers(input)
	if err != nil {
		return "", microerror.Mask(err)
	}

	if len(output.LoadBalancerDescriptions) == 0 {
		return "", microerror.Mask(tooFewResultsError)
	}

	return *output.LoadBalancerDescriptions[0].DNSName, nil
}

func (m *Manager) getInstanceDNS(nameTag string) (string, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name: aws.String("tag:Name"),
				Values: []*string{
					aws.String(nameTag),
				},
			},
		},
	}
	output, err := m.sourceClient.DescribeInstances(input)
	if err != nil {
		return "", microerror.Mask(err)
	}

	if len(output.Reservations) == 0 {
		return "", microerror.Mask(tooFewResultsError)
	}
	if len(output.Reservations[0].Instances) == 0 {
		return "", microerror.Mask(tooFewResultsError)
	}

	return *output.Reservations[0].Instances[0].PrivateDnsName, nil
}
