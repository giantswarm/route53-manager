package recordset

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"

	"github.com/giantswarm/route53-manager/pkg/client"
)

const (
	// legacySourceStackNamePattern is the pattern for Cloud Formation Stack names
	// of Tenant Clusters below Giant Swarm Release version 10.0.0, aka legacy
	// clusters, aka non Node Pool clusters.
	legacySourceStackNamePattern = "cluster-.*-guest-main"
	sourceStackNamePattern       = "cluster-.*-tccp$"
	targetStackNamePattern       = "cluster-.*-guest-recordsets"
)

const (
	installationTag = "giantswarm.io/installation"
)

var (
	// Predefined set of cloudformation stack statuses
	// which allow for valid data to be retrieved from the stack.
	stackStatusValidSource = []string{
		cloudformation.StackStatusCreateComplete,
		cloudformation.StackStatusUpdateComplete,
	}
	// Predefined set of cloudformation stack statuses
	// which allow for write operations to the stack.
	stackStatusValidTarget = []string{
		cloudformation.StackStatusCreateComplete,
		cloudformation.StackStatusCreateFailed,
		cloudformation.StackStatusDeleteFailed,
		cloudformation.StackStatusRollbackComplete,
		cloudformation.StackStatusRollbackFailed,
		cloudformation.StackStatusUpdateComplete,
		cloudformation.StackStatusUpdateRollbackComplete,
		cloudformation.StackStatusUpdateRollbackFailed,
	}
	// Predefined set of cloudformation stack statuses
	// which indicates a stack has been deleted.
	stackStatusValidDelete = []string{
		cloudformation.StackStatusDeleteComplete,
	}
	// Predefined set of cloudformation stack statuses used to read from AWS API.
	// Note: this includes all statuses except cloudformation.StackStatusDeleteComplete.
	stackStatusValid = []*string{
		aws.String(cloudformation.StackStatusCreateComplete),
		aws.String(cloudformation.StackStatusCreateInProgress),
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
	IsLegacyCluster bool
	APIELBDNS       string
	EtcdELBDNS      string
	EtcdEniList     []EtcdEni
}

type EtcdEni struct {
	DNSName   string
	IPAddress string
	Name      string
}

var (
	sourceStackNameREs []*regexp.Regexp
	targetStackNameREs []*regexp.Regexp
)

func init() {
	sourceStackNameREs = []*regexp.Regexp{
		regexp.MustCompile(legacySourceStackNamePattern),
		regexp.MustCompile(sourceStackNamePattern),
	}
	targetStackNameREs = []*regexp.Regexp{
		regexp.MustCompile(targetStackNamePattern),
	}
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
	result, err := getStacks(m.sourceClient, sourceStackNameREs, m.installation)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	m.logger.Log("level", "debug", "message", fmt.Sprintf("found source stacks: %v", getStacksName(result)))
	return result, nil
}

func (m *Manager) targetStacks() ([]cloudformation.Stack, error) {
	result, err := getStacks(m.targetClient, targetStackNameREs, m.installation)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	m.logger.Log("level", "debug", "message", fmt.Sprintf("found target stacks: %v", getStacksName(result)))
	return result, nil
}

func getStacks(cl client.StackDescribeLister, res []*regexp.Regexp, installation string) ([]cloudformation.Stack, error) {
	input := &cloudformation.ListStacksInput{
		StackStatusFilter: stackStatusValid,
	}
	output, err := cl.ListStacks(input)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var result []cloudformation.Stack

	for _, item := range output.StackSummaries {
		// filter stack by name.
		if !validStackName(*item, res) {
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

// stackHasStatus checks if stack.StackStatus matches any of statues status.
func stackHasStatus(stack cloudformation.Stack, statuses []string) bool {
	if stack.StackStatus != nil {
		for _, status := range statuses {
			if *stack.StackStatus == status {
				return true
			}
		}
	}

	return false
}

func getStacksName(stacks []cloudformation.Stack) (names []string) {
	for _, stack := range stacks {
		names = append(names, *stack.StackName)
	}

	return names
}

func validStackName(stack cloudformation.StackSummary, res []*regexp.Regexp) bool {
	for _, re := range res {
		if re.Match([]byte(*stack.StackName)) {
			return true
		}
	}

	return false
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

// createMissingTargetStacks ensures each source stack has a corresponding target stack created.
// only source stack with StackStatus matching stackStatusValidSource are processed.
// only target stack with StackStatus not matching stackStatusValidDelete are processed.
func (m *Manager) createMissingTargetStacks(sourceStacks, targetStacks []cloudformation.Stack) error {
	m.logger.Log("level", "debug", "message", "create missing target stacks")
	for _, source := range sourceStacks {
		found := false

		if !stackHasStatus(source, stackStatusValidSource) {
			m.logger.Log("level", "debug", "message", fmt.Sprintf("skipped source stack %#q with status %#q", *source.StackName, *source.StackStatus))
			continue
		}

		sourceClusterName, err := extractClusterName(*source.StackName)
		if err != nil {
			m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get source stack name %#q", *source.StackName), "stack", microerror.JSON(err))
			continue
		}

		for _, target := range targetStacks {
			if stackHasStatus(target, stackStatusValidDelete) {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("skipped target stack %#q with status %#q", *target.StackName, *target.StackStatus))
				continue
			}

			targetClusterName, err := extractClusterName(*target.StackName)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get target stack name %#q", *target.StackName), "stack", microerror.JSON(err))
				continue
			}

			if sourceClusterName == targetClusterName {
				found = true
				break
			}
		}
		if !found {
			isLegacyStack, err := sourceStackIsLegacy(*source.StackName)
			if err != nil {
				return microerror.Mask(err)
			}

			targetStackName := targetStackName(sourceClusterName)
			data, err := m.getSourceStackData(sourceClusterName, isLegacyStack)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get source stack data %#q", sourceClusterName), "stack", microerror.JSON(err))
				continue
			}

			input, err := m.getCreateStackInput(targetStackName, data, source)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to create target stack input %#q", targetStackName), "stack", microerror.JSON(err))
				continue
			}

			_, err = m.targetClient.CreateStack(input)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to create target stack %#q", targetStackName), "stack", microerror.JSON(err))
				continue
			}

			m.logger.Log("level", "debug", "message", fmt.Sprintf("created target stack %#q", targetStackName))
		}
	}
	m.logger.Log("level", "debug", "message", "created missing target stacks")
	return nil
}

// updateCurrentTargetStacks ensures each source stack has its corresponding target stack updated.
// only source stack with StackStatus matching stackStatusValidSource are processed.
// only target stack with StackStatus matching stackStatusValidTarget are processed.
func (m *Manager) updateCurrentTargetStacks(sourceStacks, targetStacks []cloudformation.Stack) error {
	m.logger.Log("level", "debug", "message", "update current target stacks")
	for _, source := range sourceStacks {
		found := false

		if !stackHasStatus(source, stackStatusValidSource) {
			m.logger.Log("level", "debug", "message", fmt.Sprintf("skipped source stack %#q with status %#q", *source.StackName, *source.StackStatus))
			continue
		}

		sourceClusterName, err := extractClusterName(*source.StackName)
		if err != nil {
			m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get source stack name %#q", *source.StackName), "stack", microerror.JSON(err))
			continue
		}

		for _, target := range targetStacks {
			if !stackHasStatus(target, stackStatusValidTarget) {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("skipped target stack %#q with status %#q", *target.StackName, *target.StackStatus))
				continue
			}

			targetClusterName, err := extractClusterName(*target.StackName)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get target stack name %#q", *target.StackName), "stack", microerror.JSON(err))
				continue
			}

			if sourceClusterName == targetClusterName {
				found = true
				break
			}
		}
		if found {
			isLegacyStack, err := sourceStackIsLegacy(*source.StackName)
			if err != nil {
				return microerror.Mask(err)
			}

			targetStackName := targetStackName(sourceClusterName)
			data, err := m.getSourceStackData(sourceClusterName, isLegacyStack)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get source stack data %#q", sourceClusterName), "stack", microerror.JSON(err))
				continue
			}

			input, err := m.getUpdateStackInput(targetStackName, data, source)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to create target stack input %#q", targetStackName), "stack", microerror.JSON(err))
				continue
			}

			_, err = m.targetClient.UpdateStack(input)
			if IsNoUpdateNeededError(err) {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("skipped target stack %#q (already up to date)", targetStackName))
			} else if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to update target stack %#q", targetStackName), "stack", microerror.JSON(err))
			} else {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("updated target stack %#q", targetStackName))
			}
		}
	}
	m.logger.Log("level", "debug", "message", "updated current target stacks")
	return nil
}

// deleteOrphanTargetStacks ensures each target stack with no corresponding source stack is deleted.
// only source stack with StackStatus not matching stackStatusValidDelete are processed.
// only target stack with StackStatus not matching stackStatusValidDelete are processed.
func (m *Manager) deleteOrphanTargetStacks(sourceStacks, targetStacks []cloudformation.Stack) error {
	m.logger.Log("level", "debug", "message", "delete orphan target stacks")
	for _, target := range targetStacks {
		found := false

		if stackHasStatus(target, stackStatusValidDelete) {
			m.logger.Log("level", "debug", "message", fmt.Sprintf("skipped target stack %#q with status %#q", *target.StackName, *target.StackStatus))
			continue
		}

		targetClusterName, err := extractClusterName(*target.StackName)
		if err != nil {
			m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get target stack name %#q", *target.StackName), "stack", microerror.JSON(err))
			continue
		}

		for _, source := range sourceStacks {
			if stackHasStatus(source, stackStatusValidDelete) {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("skipped source stack %#q with status %#q", *source.StackName, *source.StackStatus))
				continue
			}

			sourceClusterName, err := extractClusterName(*source.StackName)
			if err != nil {
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to get source stack name %#q", *source.StackName), "stack", microerror.JSON(err))
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
				m.logger.Log("level", "error", "message", fmt.Sprintf("failed to delete target stack %#q", *target.StackName), "stack", microerror.JSON(err))
			} else {
				m.logger.Log("level", "debug", "message", fmt.Sprintf("deleted target stack %#q", *target.StackName))
			}

			err = m.deleteTargetLeftovers(targetClusterName)
			if err != nil {
				m.logger.Log("level", "error", "message", "failed to delete target record sets leftovers")
			} else {
				m.logger.Log("level", "debug", "message", "deleted target record sets leftovers")
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

func (m *Manager) deleteTargetLeftovers(targetClusterName string) error {
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: &m.targetHostedZoneID,
	}
	o, err := m.targetClient.ListResourceRecordSets(input)

	if err != nil {
		return microerror.Mask(err)
	}

	resourceRecordSets := o.ResourceRecordSets

	route53Changes := []*route53.Change{}
	for _, rr := range resourceRecordSets {
		rrPattern := fmt.Sprintf("^*.%s.%s.$", targetClusterName, m.targetHostedZoneName)
		match, err := regexp.Match(rrPattern, []byte(*rr.Name))
		if err != nil {
			return microerror.Mask(err)
		}

		managedRecordSets := getManagedRecordSets(targetClusterName, m.targetHostedZoneName)
		if match && !stringInSlice(*rr.Name, managedRecordSets) {
			route53Change := &route53.Change{
				Action: aws.String("DELETE"),
				ResourceRecordSet: &route53.ResourceRecordSet{
					AliasTarget:     rr.AliasTarget,
					Name:            rr.Name,
					ResourceRecords: rr.ResourceRecords,
					TTL:             rr.TTL,
					Type:            rr.Type,
					Weight:          rr.Weight,
					SetIdentifier:   rr.SetIdentifier,
				},
			}

			route53Changes = append(route53Changes, route53Change)

			m.logger.Log("level", "debug", "message", fmt.Sprintf("found non-managed record set %#q in hosted zone %#q", *rr.Name, m.targetHostedZoneID))
		}
	}

	if len(route53Changes) > 0 {
		m.logger.Log("level", "debug", "message", fmt.Sprintf("deleting non-managed record sets in hosted zone %#q", m.targetHostedZoneID))

		changeRecordSetInput := &route53.ChangeResourceRecordSetsInput{
			ChangeBatch: &route53.ChangeBatch{
				Changes: route53Changes,
			},
			HostedZoneId: &m.targetHostedZoneID,
		}

		_, err = m.targetClient.ChangeResourceRecordSets(changeRecordSetInput)
		if err != nil {
			return microerror.Mask(err)
		}

		m.logger.Log("level", "debug", "message", fmt.Sprintf("deleted non-managed record sets in hosted zone %#q", m.targetHostedZoneID))
	}

	if err != nil {
		return microerror.Mask(err)
	}
	return nil
}

func sourceStackIsLegacy(sourceStackName string) (bool, error) {
	return regexp.Match(legacySourceStackNamePattern, []byte(sourceStackName))
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

	return "", microerror.Maskf(invalidClusterNameError, "cluster name %#q", sourceStackName)
}

func getManagedRecordSets(clusterID, baseDomain string) []string {
	return []string{
		fmt.Sprintf("\\052.%s.%s.", clusterID, baseDomain), // \\052 - `*` wildcard record
		fmt.Sprintf("api.%s.%s.", clusterID, baseDomain),
		fmt.Sprintf("etcd.%s.%s.", clusterID, baseDomain),
		fmt.Sprintf("etcd1.%s.%s.", clusterID, baseDomain),
		fmt.Sprintf("etcd2.%s.%s.", clusterID, baseDomain),
		fmt.Sprintf("etcd3.%s.%s.", clusterID, baseDomain),
		fmt.Sprintf("ingress.%s.%s.", clusterID, baseDomain),
	}
}

func stringInSlice(str string, list []string) bool {
	for _, value := range list {
		if value == str {
			return true
		}
	}
	return false
}
