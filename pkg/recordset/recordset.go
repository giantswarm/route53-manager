package recordset

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"

	"github.com/giantswarm/route53-manager/pkg/client"
)

const (
	sourceStackNamePattern = "cluster-.*-guest-main"
	targetStackNamePattern = "cluster-.*-guest-recordsets"

	installationTag = "giantswarm.io/installation"
)

type Config struct {
	Logger       micrologger.Logger
	Installation string
	SourceClient client.SourceInterface
	TargetClient client.TargetInterface

	TargetHostedZoneID   string
	TargetHostedZoneName string
}

type Manager struct {
	logger       micrologger.Logger
	installation string
	sourceClient client.SourceInterface
	targetClient client.TargetInterface

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
	if c.Installation == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Installation must not be empty", c)
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
		installation: c.Installation,
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
	result, err := getStackNames(m.sourceClient, sourceStackNameRE)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	m.logger.Log("level", "debug", "message", fmt.Sprintf("source stacks found: %v", result))
	return result, nil
}

func (m *Manager) targetStacks() ([]string, error) {
	result, err := getStackNames(m.targetClient, targetStackNameRE)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	return result, nil
}

func getStackNames(cl client.StackDescribeLister, re *regexp.Regexp) ([]string, error) {
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
		// filter stack by name.
		if re.Match([]byte(*item.StackName)) {
			// filter stack by installation tag.
			describeInput := &cloudformation.DescribeStacksInput{
				StackName: aws.String(*item.StackId),
			}
			stacks, err := cl.DescribeStacks(describeInput)
			if err != nil {
				return nil, microerror.Mask(err)
			}

			for _, stack := range stacks.Stacks {
				for _, tag := range stack.Tags {
					if *tag.Key == installationTag && *tag.Value == "<installation value>" {
						result = append(result, *item.StackName)
					}
				}
			}
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
	for _, target := range targetStacks {
		found := false
		targetClusterName := extractClusterName(target)
		for _, source := range sourceStacks {
			sourceClusterName := extractClusterName(source)
			if sourceClusterName == targetClusterName {
				found = true
				break
			}
		}
		if !found {
			err := m.deleteTargetStack(target)
			if err != nil {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("failed to delete %q stack", target), "stack", fmt.Sprintf("%v", err))
			} else {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("%q stack deleted", target))
			}
		}
	}
	return nil
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
	_, err = m.targetClient.CreateStack(input)
	if err != nil {
		return microerror.Mask(err)
	}
	return nil
}

func (m *Manager) deleteTargetStack(targetStackName string) error {
	input := &cloudformation.DeleteStackInput{
		StackName: aws.String(targetStackName),
	}
	_, err := m.targetClient.DeleteStack(input)
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
