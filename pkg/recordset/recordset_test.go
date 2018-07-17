package recordset

import (
	"reflect"
	"testing"

	"github.com/giantswarm/micrologger"
)

func Test_deleteTargetOrphanStacks(t *testing.T) {
	tcs := []struct {
		description           string
		sourceStacks          []string
		targetStacks          []string
		expectedDeletedStacks []string
	}{
		{
			description:           "empty target and source stacks, nothing should be deleted",
			sourceStacks:          []string{},
			targetStacks:          []string{},
			expectedDeletedStacks: []string{},
		},
		{
			description:  "empty source stack, all should be deleted",
			sourceStacks: []string{},
			targetStacks: []string{
				"cluster-bbbbb-guest-recordset",
			},
			expectedDeletedStacks: []string{
				"cluster-bbbbb-guest-recordset",
			},
		},
		{
			description: "empty target stacks, nothing should be deleted",
			sourceStacks: []string{
				"cluster-aaaaa-guest-main",
			},
			targetStacks:          []string{},
			expectedDeletedStacks: []string{},
		},
		{
			description: "no orphaned stacks, no need to delete",
			sourceStacks: []string{
				"cluster-aaaaa-guest-main",
			},
			targetStacks: []string{
				"cluster-aaaaa-guest-recordset",
			},
			expectedDeletedStacks: []string{},
		},
		{
			description: "one orphaned stack, needs to be deleted",
			sourceStacks: []string{
				"cluster-aaaaa-guest-main",
			},
			targetStacks: []string{
				"cluster-bbbbb-guest-recordset",
			},
			expectedDeletedStacks: []string{
				"cluster-bbbbb-guest-recordset",
			},
		},
		{
			description: "multiple orphaned stack, need to be deleted",
			sourceStacks: []string{
				"cluster-aaaaa-guest-main",
			},
			targetStacks: []string{
				"cluster-bbbbb-guest-recordset",
				"cluster-ccccc-guest-main",
			},
			expectedDeletedStacks: []string{
				"cluster-bbbbb-guest-recordset",
				"cluster-ccccc-guest-main",
			},
		},
		{
			description: "mixed orphaned and not-orphaned stacks",
			sourceStacks: []string{
				"cluster-aaaaa-guest-main",
			},
			targetStacks: []string{
				"cluster-bbbbb-guest-recordset",
				"cluster-aaaaa-guest-recordset",
				"cluster-ccccc-guest-main",
			},
			expectedDeletedStacks: []string{
				"cluster-bbbbb-guest-recordset",
				"cluster-ccccc-guest-main",
			},
		},
	}

	logger, _ := micrologger.New(micrologger.Config{})
	sourceClient := &sourceClientMock{}
	targetClient := &targetClientMock{}

	c := &Config{
		Logger:       logger,
		SourceClient: sourceClient,
		TargetClient: targetClient,

		TargetHostedZoneID:   "mytarget-hostedzpne-id",
		TargetHostedZoneName: "mytarget-hostedzpne-name",
	}

	m, err := NewManager(c)
	if err != nil {
		t.Fatalf("could not create manager %#v", err)
	}
	for _, tc := range tcs {
		targetClient.deletedStacks = []string{}
		t.Run(tc.description, func(t *testing.T) {
			err := m.deleteOrphanTargetStacks(tc.sourceStacks, tc.targetStacks)
			if err != nil {
				t.Fatalf("could not create manager %#v", err)
			}

			if !reflect.DeepEqual(tc.expectedDeletedStacks, targetClient.deletedStacks) {
				t.Fatalf("expected stacks were not deleted, want %v, got %v", tc.expectedDeletedStacks, targetClient.deletedStacks)
			}
		})
	}
}
