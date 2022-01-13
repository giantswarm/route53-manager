package recordset

import (
	"bytes"
	"fmt"
	"sort"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/route53-manager/pkg/key"
)

const (
	targetStackTemplate = `AWSTemplateFormatVersion: 2010-09-09
Description: Recordset Guest CloudFormation stack.
Resources:
  {{ if .IsLegacyCluster -}}
  ingressDNSRecord:
    Type: AWS::Route53::RecordSet
    Properties:
      HostedZoneId: {{ .HostedZoneID }}
      Name: 'ingress.{{ .ClusterName }}.{{ .HostedZoneName }}'
      Type: CNAME
      TTL: '30'
      ResourceRecords:
      - {{ .IngressELBDNS }}
  {{ end -}}

  ingressWildcardDNSRecord:
    Type: AWS::Route53::RecordSet
    Properties:
      HostedZoneId: {{ .HostedZoneID }}
      Name: '*.{{ .ClusterName }}.{{ .HostedZoneName }}'
      Type: CNAME
      TTL: '30'
      ResourceRecords:
      - 'ingress.{{ .ClusterName }}.{{ .HostedZoneName }}'

  apiDNSRecord:
    Type: AWS::Route53::RecordSet
    Properties:
      HostedZoneId: {{ .HostedZoneID }}
      Name: 'api.{{ .ClusterName }}.{{ .HostedZoneName }}'
      Type: CNAME
      TTL: '30'
      ResourceRecords:
      - {{ .APIELBDNS }}

  etcdDNSRecord:
    Type: AWS::Route53::RecordSet
    Properties:
      HostedZoneId: {{ .HostedZoneID }}
      Name: 'etcd.{{ .ClusterName }}.{{ .HostedZoneName }}'
      Type: CNAME
      TTL: '30'
      ResourceRecords:
      - {{ .EtcdELBDNS }}

  {{ $hz := .HostedZoneID }}
  {{- range .EtcdEniList }}
  {{ .Name }}:
    Type: AWS::Route53::RecordSet
    Properties:
      HostedZoneId: {{ $hz }}
      Name: '{{ .DNSName }}'
      Type: A
      TTL: '30'
      ResourceRecords:
      - {{ .IPAddress }}
  {{- end }} 
`
)

func (m *Manager) getCreateStackInput(targetStackName string, data *sourceStackData, sourceStack cloudformation.Stack) (*cloudformation.CreateStackInput, error) {
	templateBody, err := m.getStackTemplateBody(data)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	input := &cloudformation.CreateStackInput{
		StackName:        aws.String(targetStackName),
		Tags:             sourceStack.Tags,
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
		Tags:         sourceStack.Tags,
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

func (m *Manager) getSourceStackData(clusterName string, isLegacyCluster bool) (*sourceStackData, error) {
	var err error
	var ingressELBDNS string

	if isLegacyCluster {
		ingressELBName := clusterName + "-ingress"
		ingressELBDNS, err = m.getELBDNS(ingressELBName)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	apiELBName := clusterName + "-api"
	apiELBDNS, err := m.getELBDNS(apiELBName)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	etcdELBName := clusterName + "-etcd"
	etcdELBDNS, err := m.getELBDNS(etcdELBName)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	eniList, err := m.getEniList(clusterName, key.BaseDomain(clusterName, m.targetHostedZoneName))
	if err != nil {
		return nil, microerror.Mask(err)
	}

	output := &sourceStackData{
		HostedZoneID:    m.targetHostedZoneID,
		HostedZoneName:  m.targetHostedZoneName,
		ClusterName:     clusterName,
		IngressELBDNS:   ingressELBDNS,
		IsLegacyCluster: isLegacyCluster,
		APIELBDNS:       apiELBDNS,
		EtcdELBDNS:      etcdELBDNS,
		EtcdEniList:     eniList,
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

func (m *Manager) getEniList(clusterID string, baseDomain string) ([]EtcdEni, error) {
	var eniList []EtcdEni

	input := &ec2.DescribeNetworkInterfacesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String(fmt.Sprintf("tag:%s", key.TagCluster)),
				Values: []*string{
					aws.String(clusterID),
				},
			},
		},
	}

	output, err := m.sourceClient.DescribeNetworkInterfaces(input)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	nicList := output.NetworkInterfaces
	sortNetworkInterfacesByName(nicList)

	for i, nic := range nicList {
		e := EtcdEni{
			DNSName:   key.EtcdENIDNSName(baseDomain, i),
			IPAddress: *nic.PrivateIpAddress,
			Name:      key.EtcdEniResourceName(i),
		}
		eniList = append(eniList, e)
	}
	// always add `etcd0` dns record to avoid issues with single master in china
	if len(nicList) > 0 {
		etcdRecordZero := EtcdEni{
			// the key function will add `1` to the index so  the  dns name will be `etcd0` in this case
			DNSName:   key.EtcdENIDNSName(baseDomain, -1),
			IPAddress: *nicList[0].PrivateIpAddress,
			Name:      key.EtcdEniResourceName(-1),
		}
		eniList = append(eniList, etcdRecordZero)
	}

	return eniList, nil
}

func sortNetworkInterfacesByName(nicList []*ec2.NetworkInterface) {
	sort.Slice(nicList, func(i, j int) bool {
		nameI := getNICNameFromTag(nicList[i].TagSet)
		nameJ := getNICNameFromTag(nicList[j].TagSet)
		return nameI < nameJ
	})
}

func getNICNameFromTag(tags []*ec2.Tag) string {
	for _, tag := range tags {
		if *tag.Key == "Name" {
			return *tag.Value
		}
	}
	// failed to find name tag
	return ""
}
