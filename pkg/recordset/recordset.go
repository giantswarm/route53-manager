package recordset

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"

	"github.com/giantswarm/route53-manager/pkg/client"
)

const (
	sourceStackNamePattern = "cluster-.*-guest-main"
	targetStackNamePattern = "cluster-.*-guest-recordsets"
	targetStackTemplate    = `AWSTemplateFormatVersion: 2010-09-09
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

type Config struct {
	Logger       micrologger.Logger
	SourceClient *client.Clients
	TargetClient *client.Clients

	TargetHostedZoneID   string
	TargetHostedZoneName string
}

type Manager struct {
	logger       micrologger.Logger
	sourceClient *client.Clients
	targetClient *client.Clients

	targetHostedZoneID   string
	targetHostedZoneName string
}

type sourceStackData struct {
	HostedZoneID    string
	HostedZoneName  string
	ClusterName     string
	IngressELBDNS   string
	APIELBDNS       string
	EtcdInstanceDNS string
}

var (
	sourceStackNameRE *regexp.Regexp
	targetStackNameRE *regexp.Regexp
)

func init() {
	sourceStackNameRE = regexp.MustCompile(sourceStackNamePattern)
	targetStackNameRE = regexp.MustCompile(targetStackNamePattern)
}

func NewManager(c *Config) (*Manager, error) {
	if c.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", c)
	}
	if c.SourceClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.SourceClient must not be empty", c)
	}
	if c.TargetClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.TargetClient must not be empty", c)
	}
	if c.TargetHostedZoneID == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.TargetHostedZoneID must not be empty", c)
	}
	if c.TargetHostedZoneName == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.TargetHostedZoneName must not be empty", c)
	}

	m := &Manager{
		logger:       c.Logger,
		sourceClient: c.SourceClient,
		targetClient: c.TargetClient,

		targetHostedZoneID:   c.TargetHostedZoneID,
		targetHostedZoneName: c.TargetHostedZoneName,
	}

	return m, nil
}

func (m *Manager) Sync() error {
	sourceStacks, err := m.sourceStacks()
	if err != nil {
		return microerror.Mask(err)
	}

	targetStacks, err := m.targetStacks()
	if err != nil {
		return microerror.Mask(err)
	}

	err = m.createMissingTargetStacks(sourceStacks, targetStacks)
	if err != nil {
		return microerror.Mask(err)
	}

	err = m.deleteOrphanTargetStacks(sourceStacks, targetStacks)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func (m *Manager) sourceStacks() ([]string, error) {
	result, err := getStackNames(m.sourceClient.CloudFormation, sourceStackNameRE)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	m.logger.Log("level", "debug", "message", fmt.Sprintf("source stacks found: %v", result))
	return result, nil
}

func (m *Manager) targetStacks() ([]string, error) {
	result, err := getStackNames(m.targetClient.CloudFormation, targetStackNameRE)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	return result, nil
}

func getStackNames(cl *cloudformation.CloudFormation, re *regexp.Regexp) ([]string, error) {
	input := &cloudformation.ListStacksInput{
		StackStatusFilter: []*string{
			aws.String(cloudformation.StackStatusCreateComplete),
			aws.String(cloudformation.StackStatusUpdateComplete),
		},
	}
	output, err := cl.ListStacks(input)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var result []string

	for _, item := range output.StackSummaries {
		if re.Match([]byte(*item.StackName)) {
			result = append(result, *item.StackName)
		}
	}

	return result, nil
}

func (m *Manager) createMissingTargetStacks(sourceStacks, targetStacks []string) error {
	for _, source := range sourceStacks {
		found := false
		sourceClusterName := extractClusterName(source)
		for _, target := range targetStacks {
			targetClusterName := extractClusterName(target)
			if sourceClusterName == targetClusterName {
				found = true
				break
			}
		}
		if !found {
			targetStackName := targetStackName(sourceClusterName)
			data, err := m.getSourceStackData(sourceClusterName)
			m.logger.Log("level", "debug", "message", fmt.Sprintf("data for %q: %#v", sourceClusterName, data))
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("could not get data about %q: %v", sourceClusterName, err))
			}
			err = m.createTargetStack(targetStackName, data)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("could not create target stack %q: %v", targetStackName, err))
			} else {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("target stack %q created", targetStackName))
			}
		}
	}
	return nil
}

func (m *Manager) deleteOrphanTargetStacks(sourceStacks, targetStacks []string) error {
	return nil
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
	output, err := m.sourceClient.ELB.DescribeLoadBalancers(input)
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
	output, err := m.sourceClient.EC2.DescribeInstances(input)
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

func (m *Manager) createTargetStack(targetStackName string, data *sourceStackData) error {
	tmpl, err := template.New("recordsets").Parse(targetStackTemplate)
	if err != nil {
		return microerror.Mask(err)
	}

	var templateBody bytes.Buffer
	err = tmpl.Execute(&templateBody, data)
	if err != nil {
		return microerror.Mask(err)
	}

	input := &cloudformation.CreateStackInput{
		StackName:        aws.String(targetStackName),
		TemplateBody:     aws.String(templateBody.String()),
		TimeoutInMinutes: aws.Int64(2),
	}
	_, err = m.targetClient.CloudFormation.CreateStack(input)
	if err != nil {
		return microerror.Mask(err)
	}
	return nil
}

func targetStackName(clusterName string) string {
	targetStackNameFmt := strings.Replace(targetStackNamePattern, ".*", "%s", 1)

	return fmt.Sprintf(targetStackNameFmt, clusterName)
}

func extractClusterName(sourceStackName string) string {
	parts := strings.Split(sourceStackName, "-")
	return parts[1]
}
