package recordset

import (
	"fmt"
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

	err = m.updateCurrentTargetStacks(sourceStacks, targetStacks)
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
	result, err := getStackNames(m.sourceClient, sourceStackNameRE, m.installation)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	m.logger.Log("level", "debug", "message", fmt.Sprintf("source stacks found: %v", result))
	return result, nil
}

func (m *Manager) targetStacks() ([]string, error) {
	result, err := getStackNames(m.targetClient, targetStackNameRE, m.installation)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	return result, nil
}

func getStackNames(cl client.StackDescribeLister, re *regexp.Regexp, installation string) ([]string, error) {
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
		if !validStackName(*item, re) {
			continue
		}

		// filter stack by installation tag.
		describeInput := &cloudformation.DescribeStacksInput{
			StackName: aws.String(*item.StackId),
		}
		stacks, err := cl.DescribeStacks(describeInput)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		if !validStackInstallationTag(stacks, installation) {
			continue
		}

		result = append(result, *item.StackName)
	}

	return result, nil
}

func validStackName(stack cloudformation.StackSummary, re *regexp.Regexp) bool {
	return re.Match([]byte(*stack.StackName))
}

func validStackInstallationTag(stacks *cloudformation.DescribeStacksOutput, installation string) bool {
	for _, stack := range stacks.Stacks {
		for _, tag := range stack.Tags {
			if *tag.Key == installationTag && *tag.Value == installation {
				return true
			}
		}
	}

	return false
}

func (m *Manager) createMissingTargetStacks(sourceStacks, targetStacks []string) error {
	m.logger.Log("level", "debug", "message", "create missing target stacks")
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

			input, err := m.getCreateStackInput(targetStackName, data)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("could not create target stack input %q: %v", targetStackName, err))
			}

			_, err = m.targetClient.CreateStack(input)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("could not create target stack %q: %v", targetStackName, err))
			} else {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("target stack %q created", targetStackName))
			}
		}
	}
	return nil
}

func (m *Manager) updateCurrentTargetStacks(sourceStacks, targetStacks []string) error {
	m.logger.Log("level", "debug", "message", "update current target stacks")
	for _, source := range sourceStacks {
		found := false
		sourceClusterName := extractClusterName(source)
		m.logger.Log("level", "debug", "message", fmt.Sprintf("source: %q", sourceClusterName))
		for _, target := range targetStacks {
			targetClusterName := extractClusterName(target)
			m.logger.Log("level", "debug", "message", fmt.Sprintf("target: %q", targetClusterName))
			if sourceClusterName == targetClusterName {
				found = true
				break
			}
		}
		if found {
			targetStackName := targetStackName(sourceClusterName)
			data, err := m.getSourceStackData(sourceClusterName)
			m.logger.Log("level", "debug", "message", fmt.Sprintf("data for %q: %#v", sourceClusterName, data))
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("could not get data about %q: %v", sourceClusterName, err))
			}

			input, err := m.getUpdateStackInput(targetStackName, data)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("could not create target stack input %q: %v", targetStackName, err))
			}

			_, err = m.targetClient.UpdateStack(input)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("could not update target stack %q: %v", targetStackName, err))
			} else {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("target stack %q updated", targetStackName))
			}

		}
	}

	return nil
}

func (m *Manager) deleteOrphanTargetStacks(sourceStacks, targetStacks []string) error {
	m.logger.Log("level", "debug", "message", "delete orphan target stacks")
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
