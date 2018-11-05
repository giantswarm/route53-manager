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

var (
	stackStatusCompleteNotDelete = []string{
		cloudformation.StackStatusCreateComplete,
		cloudformation.StackStatusRollbackComplete,
		cloudformation.StackStatusUpdateComplete,
		cloudformation.StackStatusUpdateRollbackComplete,
	}
	stackStatusCompleteNotDeleteAndFail = []string{
		cloudformation.StackStatusCreateComplete,
		cloudformation.StackStatusCreateFailed,
		cloudformation.StackStatusDeleteFailed,
		cloudformation.StackStatusRollbackComplete,
		cloudformation.StackStatusRollbackFailed,
		cloudformation.StackStatusUpdateComplete,
		cloudformation.StackStatusUpdateRollbackComplete,
		cloudformation.StackStatusUpdateRollbackFailed,
	}
	stackStatusDeleted = []string{
		cloudformation.StackStatusDeleteComplete,
	}
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

func (m *Manager) sourceStacks() ([]cloudformation.Stack, error) {
	result, err := getStacks(m.sourceClient, sourceStackNameRE, m.installation)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	m.logger.Log("level", "debug", "message", fmt.Sprintf("found source stacks: %v", getStacksName(result)))
	return result, nil
}

func (m *Manager) targetStacks() ([]cloudformation.Stack, error) {
	result, err := getStacks(m.targetClient, targetStackNameRE, m.installation)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	m.logger.Log("level", "debug", "message", fmt.Sprintf("found target stacks: %v", getStacksName(result)))
	return result, nil
}

func getStacks(cl client.StackDescribeLister, re *regexp.Regexp, installation string) ([]cloudformation.Stack, error) {
	input := &cloudformation.ListStacksInput{
		StackStatusFilter: []*string{
			aws.String(cloudformation.StackStatusCreateComplete),
			aws.String(cloudformation.StackStatusCreateFailed),
			aws.String(cloudformation.StackStatusRollbackInProgress),
			aws.String(cloudformation.StackStatusRollbackFailed),
			aws.String(cloudformation.StackStatusRollbackComplete),
			aws.String(cloudformation.StackStatusDeleteInProgress),
			aws.String(cloudformation.StackStatusDeleteFailed),
			aws.String(cloudformation.StackStatusUpdateInProgress),
			aws.String(cloudformation.StackStatusUpdateCompleteCleanupInProgress),
			aws.String(cloudformation.StackStatusUpdateComplete),
			aws.String(cloudformation.StackStatusUpdateRollbackInProgress),
			aws.String(cloudformation.StackStatusUpdateRollbackFailed),
			aws.String(cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress),
			aws.String(cloudformation.StackStatusUpdateRollbackComplete),
			aws.String(cloudformation.StackStatusReviewInProgress),
		},
	}
	output, err := cl.ListStacks(input)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var result []cloudformation.Stack

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
		key := validStackInstallationTag(stacks, installation)
		if key == -1 {
			continue
		}

		result = append(result, *stacks.Stacks[key])
	}

	return result, nil
}

func filterStacksByStatus(input []cloudformation.Stack, statuses []string, exclude bool) []cloudformation.Stack {
	output := []cloudformation.Stack{}

	for _, stack := range input {
		match := false
		if stack.StackStatus != nil {
			for _, status := range statuses {
				if *stack.StackStatus == status {
					match = true
					break
				}
			}
		}

		if (exclude && !match) ||
			(!exclude && match) {
			output = append(output, stack)
		}

	}

	return output
}

func getStacksName(stacks []cloudformation.Stack) (names []string) {
	for _, stack := range stacks {
		names = append(names, *stack.StackName)
	}

	return names
}

func validStackName(stack cloudformation.StackSummary, re *regexp.Regexp) bool {
	return re.Match([]byte(*stack.StackName))
}

func validStackInstallationTag(stacks *cloudformation.DescribeStacksOutput, installation string) int {
	for key, stack := range stacks.Stacks {
		for _, tag := range stack.Tags {
			if *tag.Key == installationTag && *tag.Value == installation {
				return key
			}
		}
	}

	return -1
}

func (m *Manager) createMissingTargetStacks(sourceStacks, targetStacks []cloudformation.Stack) error {
	m.logger.Log("level", "debug", "message", "create missing target stacks")
	for _, source := range filterStacksByStatus(sourceStacks, stackStatusCompleteNotDelete, false) {
		found := false
		sourceClusterName, err := extractClusterName(*source.StackName)
		if err != nil {
			m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get source stack name %#q", *source.StackName), "stack", fmt.Sprintf("%#v", err))
			continue
		}

		for _, target := range filterStacksByStatus(targetStacks, stackStatusDeleted, true) {
			targetClusterName, err := extractClusterName(*target.StackName)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get target stack name %#q", *target.StackName), "stack", fmt.Sprintf("%#v", err))
				continue
			}

			if sourceClusterName == targetClusterName {
				found = true
				break
			}
		}
		if !found {
			targetStackName := targetStackName(sourceClusterName)
			data, err := m.getSourceStackData(sourceClusterName)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get source stack data %#q", sourceClusterName), "stack", fmt.Sprintf("%#v", err))
				continue
			}

			input, err := m.getCreateStackInput(targetStackName, data, source)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to create target stack input %#q", targetStackName), "stack", fmt.Sprintf("%#v", err))
				continue
			}

			_, err = m.targetClient.CreateStack(input)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to create target stack %#q", targetStackName), "stack", fmt.Sprintf("%#v", err))

				continue
			}

			m.logger.Log("level", "debug", "message", fmt.Sprintf("created target stack %#q", targetStackName))
		}
	}
	m.logger.Log("level", "debug", "message", "created missing target stacks")
	return nil
}

func (m *Manager) updateCurrentTargetStacks(sourceStacks, targetStacks []cloudformation.Stack) error {
	m.logger.Log("level", "debug", "message", "update current target stacks")
	for _, source := range filterStacksByStatus(sourceStacks, stackStatusCompleteNotDelete, false) {
		found := false
		sourceClusterName, err := extractClusterName(*source.StackName)
		if err != nil {
			m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get source stack name %#q", *source.StackName), "stack", fmt.Sprintf("%#v", err))
			continue
		}

		for _, target := range filterStacksByStatus(targetStacks, stackStatusCompleteNotDeleteAndFail, false) {
			targetClusterName, err := extractClusterName(*target.StackName)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get target stack name %#q", *target.StackName), "stack", fmt.Sprintf("%#v", err))
				continue
			}

			if sourceClusterName == targetClusterName {
				found = true
				break
			}
		}
		if found {
			targetStackName := targetStackName(sourceClusterName)
			data, err := m.getSourceStackData(sourceClusterName)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get srouce stack data %#q", sourceClusterName), "stack", fmt.Sprintf("%#v", err))
			}

			input, err := m.getUpdateStackInput(targetStackName, data, source)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to create target stack input %#q", targetStackName), "stack", fmt.Sprintf("%#v", err))
			}

			_, err = m.targetClient.UpdateStack(input)
			if IsNoUpdateNeededError(err) {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("skipped target stack %#q (already up to date)", targetStackName))
			} else if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to update target stack %#q", targetStackName), "stack", fmt.Sprintf("%#v", err))

			} else {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("updated target stack %#q", targetStackName))
			}
		}
	}
	m.logger.Log("level", "debug", "message", "updated current target stacks")
	return nil
}

func (m *Manager) deleteOrphanTargetStacks(sourceStacks, targetStacks []cloudformation.Stack) error {
	m.logger.Log("level", "debug", "message", "delete orphan target stacks")
	for _, target := range filterStacksByStatus(targetStacks, stackStatusDeleted, true) {
		found := false
		targetClusterName, err := extractClusterName(*target.StackName)
		if err != nil {
			m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get target stack name %#q", *target.StackName), "stack", fmt.Sprintf("%#v", err))
			continue
		}

		for _, source := range filterStacksByStatus(sourceStacks, stackStatusDeleted, true) {
			sourceClusterName, err := extractClusterName(*source.StackName)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get source stack name %#q", *source.StackName), "stack", fmt.Sprintf("%#v", err))
				continue
			}

			if sourceClusterName == targetClusterName {
				found = true
				break
			}
		}
		if !found {
			err := m.deleteTargetStack(*target.StackName)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to delete target stack %#q", *target.StackName), "stack", fmt.Sprintf("%#v", err))
			} else {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("deleted target stack %#q", *target.StackName))
			}
		}
	}
	m.logger.Log("level", "debug", "message", "deleted orphan target stacks")
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

func extractClusterName(sourceStackName string) (string, error) {
	parts := strings.Split(sourceStackName, "-")
	if len(parts) >= 2 {
		return parts[1], nil
	}

	return "", invalidClusterNameError
}
