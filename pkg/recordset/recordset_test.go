package recordset

import (
	"io/ioutil"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/giantswarm/micrologger"
)

func TestCreateMissingStacks_Cases(t *testing.T) {
	var (
		installation = "installation"
		zoneID       = "zoneID"
		zoneName     = "zoneName"
	)

	tcs := []struct {
		description           string
		sourceStacks          []cloudformation.Stack
		targetStacks          []cloudformation.Stack
		expectedCreatedStacks []string
	}{
		{
			"create 1 stack",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			nil,
			[]string{"cluster-foo-guest-recordsets"},
		},
		{
			"create 2 stacks",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-bar-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			nil,
			[]string{"cluster-foo-guest-recordsets", "cluster-bar-guest-recordsets"},
		},
		{
			"create 2 stacks out of 3",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-bar-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-baz-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-bar-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			[]string{"cluster-foo-guest-recordsets", "cluster-baz-guest-recordsets"},
		},
		{
			"do not create already existing stack",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			nil,
		},
		{
			"do not create stack when there is no source",
			nil,
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			nil,
		},
		{
			"do not create stack when there is no source and target",
			nil,
			nil,
			nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			logger, err := micrologger.New(micrologger.Config{IOWriter: ioutil.Discard})
			if err != nil {
				t.Fatalf("micrologger.New: %v", err)
			}

			sourceClient := newSourceWithStacks(tc.sourceStacks)
			targetClient := newTargetWithStacks(tc.targetStacks)

			c := &Config{
				Logger:               logger,
				Installation:         installation,
				SourceClient:         sourceClient,
				TargetClient:         targetClient,
				TargetHostedZoneID:   zoneID,
				TargetHostedZoneName: zoneName,
			}
			m, err := NewManager(c)
			if err != nil {
				t.Fatalf("NewManager: %v", err)
			}

			err = m.createMissingTargetStacks(tc.sourceStacks, tc.targetStacks)
			if err != nil {
				t.Fatalf("m.Sync: %v", err)
			}

			sort.Strings(targetClient.createdStacks)
			sort.Strings(tc.expectedCreatedStacks)

			if !reflect.DeepEqual(tc.expectedCreatedStacks, targetClient.createdStacks) {
				t.Errorf("created, expected %v  got %v", tc.expectedCreatedStacks, targetClient.createdStacks)
			}
		})
	}
}

// TestCreateMissingStacks_Statuses tests Manager.createMissingTargetStacks
// s: filterIn(*complete not delete)
// s: complete not delete
// t: nil
func TestCreateMissingStacks_Statuses(t *testing.T) {
	var (
		installation = "installation"
		zoneID       = "zoneID"
		zoneName     = "zoneName"
	)

	tcs := []struct {
		description  string
		status       string
		expectCreate bool
	}{
		{
			"create stack when source status is create complete",
			cloudformation.StackStatusCreateComplete,
			true,
		},
		{
			"create stack when source status is update complete",
			cloudformation.StackStatusUpdateComplete,
			true,
		},
		{
			"create stack when source status is rollback complete",
			cloudformation.StackStatusRollbackComplete,
			true,
		},
		{
			"create stack when source status is update rollback complete",
			cloudformation.StackStatusUpdateRollbackComplete,
			true,
		},
		{
			"do not create stack when source status is create in progress",
			cloudformation.StackStatusCreateInProgress,
			false,
		},
		{
			"do not create stack when source status is create failed",
			cloudformation.StackStatusCreateFailed,
			false,
		},
		{
			"do not create stack when source status is rollback in progress",
			cloudformation.StackStatusRollbackInProgress,
			false,
		},
		{
			"do not create stack when source status is rollback failed",
			cloudformation.StackStatusRollbackFailed,
			false,
		},
		{
			"do not create stack when source status is delete in progress",
			cloudformation.StackStatusDeleteInProgress,
			false,
		},
		{
			"do not create stack when source status is delete failed",
			cloudformation.StackStatusDeleteFailed,
			false,
		},
		{
			"do not create stack when source status is delete complete",
			cloudformation.StackStatusDeleteComplete,
			false,
		},
		{
			"do not create stack when source status is update in progress",
			cloudformation.StackStatusUpdateInProgress,
			false,
		},
		{
			"do not create stack when source status is udpdate complete cleanup in progress",
			cloudformation.StackStatusUpdateCompleteCleanupInProgress,
			false,
		},
		{
			"do not create stack when source status is update rollback in progress",
			cloudformation.StackStatusUpdateRollbackInProgress,
			false,
		},
		{
			"do not create stack when source status is update rollback failed",
			cloudformation.StackStatusUpdateRollbackFailed,
			false,
		},
		{
			"do not create stack when source status is update rollback complete cleanup in progress",
			cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
			false,
		},
		{
			"do not create stack when source status is review in progress",
			cloudformation.StackStatusReviewInProgress,
			false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			logger, err := micrologger.New(micrologger.Config{IOWriter: ioutil.Discard})
			if err != nil {
				t.Fatalf("micrologger.New: %v", err)
			}

			sourceStacks := []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(tc.status),
					Tags: []*cloudformation.Tag{
						&cloudformation.Tag{
							Key:   aws.String(installationTag),
							Value: aws.String(installation),
						},
					},
				},
			}

			sourceClient := newSourceWithStacks(sourceStacks)
			targetClient := newTargetWithStacks(nil)

			c := &Config{
				Logger:               logger,
				Installation:         installation,
				SourceClient:         sourceClient,
				TargetClient:         targetClient,
				TargetHostedZoneID:   zoneID,
				TargetHostedZoneName: zoneName,
			}
			m, err := NewManager(c)
			if err != nil {
				t.Fatalf("NewManager: %v", err)
			}

			err = m.createMissingTargetStacks(sourceStacks, nil)
			if err != nil {
				t.Fatalf("m.Sync: %v", err)
			}

			if tc.expectCreate && len(targetClient.createdStacks) <= 0 {
				t.Errorf("creation expected, got nothing")
			} else if !tc.expectCreate && len(targetClient.createdStacks) > 0 {
				t.Errorf("no creation expected, got %v", targetClient.createdStacks)
			}
		})
	}
}

func TestUpdateCurrentTargetStacks_Cases(t *testing.T) {
	var (
		installation = "installation"
		zoneID       = "zoneID"
		zoneName     = "zoneName"
	)

	tcs := []struct {
		description           string
		sourceStacks          []cloudformation.Stack
		targetStacks          []cloudformation.Stack
		expectedUpdatedStacks []string
	}{
		{
			"update 1 stack",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			[]string{"cluster-foo-guest-recordsets"},
		},
		{
			"update 2 stacks",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-bar-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-bar-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			[]string{"cluster-foo-guest-recordsets", "cluster-bar-guest-recordsets"},
		},
		{
			"update 2 stacks out of 3",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-bar-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-baz-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-baz-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			[]string{"cluster-foo-guest-recordsets", "cluster-baz-guest-recordsets"},
		},

		{
			"do not update missing target stack",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			nil,
			nil,
		},
		{
			"do not update missing source stack",
			nil,
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			nil,
		},
		{
			"do not update missing source and target stacks",
			nil,
			nil,
			nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			logger, err := micrologger.New(micrologger.Config{IOWriter: ioutil.Discard})
			if err != nil {
				t.Fatalf("micrologger.New: %v", err)
			}

			sourceClient := newSourceWithStacks(tc.sourceStacks)
			targetClient := newTargetWithStacks(tc.targetStacks)

			c := &Config{
				Logger:               logger,
				Installation:         installation,
				SourceClient:         sourceClient,
				TargetClient:         targetClient,
				TargetHostedZoneID:   zoneID,
				TargetHostedZoneName: zoneName,
			}
			m, err := NewManager(c)
			if err != nil {
				t.Fatalf("NewManager: %v", err)
			}

			err = m.updateCurrentTargetStacks(tc.sourceStacks, tc.targetStacks)
			if err != nil {
				t.Fatalf("m.Sync: %v", err)
			}

			sort.Strings(targetClient.updatedStacks)
			sort.Strings(tc.expectedUpdatedStacks)

			if !reflect.DeepEqual(tc.expectedUpdatedStacks, targetClient.updatedStacks) {
				t.Errorf("updated, expected %v  got %v", tc.expectedUpdatedStacks, targetClient.updatedStacks)
			}
		})
	}
}

// TestUpdateCurrentTargetStacks_SourceStatuses tests Manager.updateCurrentTargetStacks
// filterIn(*complete not delete)
// s: complete not delete
// t: *complete not delete or *failed
func TestUpdateCurrentTargetStacks_SourceStatuses(t *testing.T) {
	var (
		installation = "installation"
		zoneID       = "zoneID"
		zoneName     = "zoneName"
	)

	tcs := []struct {
		description  string
		status       string
		expectUpdate bool
	}{
		{
			"update stack when source status is create complete",
			cloudformation.StackStatusCreateComplete,
			true,
		},
		{
			"update stack when source status is update complete",
			cloudformation.StackStatusUpdateComplete,
			true,
		},
		{
			"update stack when source status is rollback complete",
			cloudformation.StackStatusRollbackComplete,
			true,
		},
		{
			"update stack when source status is update rollback complete",
			cloudformation.StackStatusUpdateRollbackComplete,
			true,
		},
		{
			"do not update stack when source status is create in progress",
			cloudformation.StackStatusCreateInProgress,
			false,
		},
		{
			"do not update stack when source status is create failed",
			cloudformation.StackStatusCreateFailed,
			false,
		},
		{
			"do not update stack when source status is rollback in progress",
			cloudformation.StackStatusRollbackInProgress,
			false,
		},
		{
			"do not update stack when source status is rollback failed",
			cloudformation.StackStatusRollbackFailed,
			false,
		},
		{
			"do not update stack when source status is delete in progress",
			cloudformation.StackStatusDeleteInProgress,
			false,
		},
		{
			"do not update stack when source status is delete failed",
			cloudformation.StackStatusDeleteFailed,
			false,
		},
		{
			"do not update stack when source status is delete complete",
			cloudformation.StackStatusDeleteComplete,
			false,
		},
		{
			"do not update stack when source status is update in progress",
			cloudformation.StackStatusUpdateInProgress,
			false,
		},
		{
			"do not update stack when source status is udpdate complete cleanup in progress",
			cloudformation.StackStatusUpdateCompleteCleanupInProgress,
			false,
		},
		{
			"do not update stack when source status is update rollback in progress",
			cloudformation.StackStatusUpdateRollbackInProgress,
			false,
		},
		{
			"do not update stack when source status is update rollback failed",
			cloudformation.StackStatusUpdateRollbackFailed,
			false,
		},
		{
			"do not update stack when source status is update rollback complete cleanup in progress",
			cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
			false,
		},
		{
			"do not update stack when source status is review in progress",
			cloudformation.StackStatusReviewInProgress,
			false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			logger, err := micrologger.New(micrologger.Config{IOWriter: ioutil.Discard})
			if err != nil {
				t.Fatalf("micrologger.New: %v", err)
			}
			sourceStacks := []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(tc.status),
					Tags: []*cloudformation.Tag{
						&cloudformation.Tag{
							Key:   aws.String(installationTag),
							Value: aws.String(installation),
						},
					},
				},
			}
			targetStacks := []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
					Tags: []*cloudformation.Tag{
						&cloudformation.Tag{
							Key:   aws.String(installationTag),
							Value: aws.String(installation),
						},
					},
				},
			}

			sourceClient := newSourceWithStacks(sourceStacks)
			targetClient := newTargetWithStacks(targetStacks)

			c := &Config{
				Logger:               logger,
				Installation:         installation,
				SourceClient:         sourceClient,
				TargetClient:         targetClient,
				TargetHostedZoneID:   zoneID,
				TargetHostedZoneName: zoneName,
			}
			m, err := NewManager(c)
			if err != nil {
				t.Fatalf("NewManager: %v", err)
			}

			err = m.updateCurrentTargetStacks(sourceStacks, targetStacks)
			if err != nil {
				t.Fatalf("m.Sync: %v", err)
			}

			if tc.expectUpdate && len(targetClient.updatedStacks) <= 0 {
				t.Errorf("update expected, got nothing")
			} else if !tc.expectUpdate && len(targetClient.updatedStacks) > 0 {
				t.Errorf("no update expected, got %v", targetClient.updatedStacks)
			}
		})
	}
}


// TestUpdateCurrentTargetStacks_TargetStatuses tests Manager.updateCurrentTargetStacks
// filterIn(*complete not delete)
// s: complete not delete
// t: *complete not delete or *failed
func TestUpdateCurrentTargetStacks_TargetStatuses(t *testing.T) {
	var (
		installation = "installation"
		zoneID       = "zoneID"
		zoneName     = "zoneName"
	)

	tcs := []struct {
		description  string
		status       string
		expectUpdate bool
	}{
		{
			"update stack when source status is create complete",
			cloudformation.StackStatusCreateComplete,
			true,
		},
		{
			"update stack when source status is update complete",
			cloudformation.StackStatusUpdateComplete,
			true,
		},
		{
			"update stack when source status is rollback complete",
			cloudformation.StackStatusRollbackComplete,
			true,
		},
		{
			"update stack when source status is update rollback complete",
			cloudformation.StackStatusUpdateRollbackComplete,
			true,
		},
		{
			"do not update stack when source status is create failed",
			cloudformation.StackStatusCreateFailed,
			true,
		},
		{
			"do not update stack when source status is rollback failed",
			cloudformation.StackStatusRollbackFailed,
			true,
		},
		{
			"do not update stack when source status is delete failed",
			cloudformation.StackStatusDeleteFailed,
			true,
		},
		{
			"do not update stack when source status is update rollback failed",
			cloudformation.StackStatusUpdateRollbackFailed,
			true,
		},
		{
			"do not update stack when source status is create in progress",
			cloudformation.StackStatusCreateInProgress,
			false,
		},
		{
			"do not update stack when source status is rollback in progress",
			cloudformation.StackStatusRollbackInProgress,
			false,
		},
		{
			"do not update stack when source status is delete in progress",
			cloudformation.StackStatusDeleteInProgress,
			false,
		},
		{
			"do not update stack when source status is delete complete",
			cloudformation.StackStatusDeleteComplete,
			false,
		},
		{
			"do not update stack when source status is update in progress",
			cloudformation.StackStatusUpdateInProgress,
			false,
		},
		{
			"do not update stack when source status is udpdate complete cleanup in progress",
			cloudformation.StackStatusUpdateCompleteCleanupInProgress,
			false,
		},
		{
			"do not update stack when source status is update rollback in progress",
			cloudformation.StackStatusUpdateRollbackInProgress,
			false,
		},
		{
			"do not update stack when source status is update rollback complete cleanup in progress",
			cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
			false,
		},
		{
			"do not update stack when source status is review in progress",
			cloudformation.StackStatusReviewInProgress,
			false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			logger, err := micrologger.New(micrologger.Config{IOWriter: ioutil.Discard})
			if err != nil {
				t.Fatalf("micrologger.New: %v", err)
			}
			sourceStacks := []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
					Tags: []*cloudformation.Tag{
						&cloudformation.Tag{
							Key:   aws.String(installationTag),
							Value: aws.String(installation),
						},
					},
				},
			}
			targetStacks := []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(tc.status),
					Tags: []*cloudformation.Tag{
						&cloudformation.Tag{
							Key:   aws.String(installationTag),
							Value: aws.String(installation),
						},
					},
				},
			}

			sourceClient := newSourceWithStacks(sourceStacks)
			targetClient := newTargetWithStacks(targetStacks)

			c := &Config{
				Logger:               logger,
				Installation:         installation,
				SourceClient:         sourceClient,
				TargetClient:         targetClient,
				TargetHostedZoneID:   zoneID,
				TargetHostedZoneName: zoneName,
			}
			m, err := NewManager(c)
			if err != nil {
				t.Fatalf("NewManager: %v", err)
			}

			err = m.updateCurrentTargetStacks(sourceStacks, targetStacks)
			if err != nil {
				t.Fatalf("m.Sync: %v", err)
			}

			if tc.expectUpdate && len(targetClient.updatedStacks) <= 0 {
				t.Errorf("update expected, got nothing")
			} else if !tc.expectUpdate && len(targetClient.updatedStacks) > 0 {
				t.Errorf("no update expected, got %v", targetClient.updatedStacks)
			}
		})
	}
}

func TestDeleteOrphanTargetStacks_Cases(t *testing.T) {
	tcs := []struct {
		description           string
		sourceStacks          []cloudformation.Stack
		targetStacks          []cloudformation.Stack
		expectedDeletedStacks []string
	}{
		{
			description:           "empty target and source stacks, nothing should be deleted",
			sourceStacks:          []cloudformation.Stack{},
			targetStacks:          []cloudformation.Stack{},
			expectedDeletedStacks: []string{},
		},
		{
			description:  "empty source stack, all should be deleted",
			sourceStacks: []cloudformation.Stack{},
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-bbbbb-guest-recordset")},
			},
			expectedDeletedStacks: []string{
				"cluster-bbbbb-guest-recordset",
			},
		},
		{
			description: "empty target stacks, nothing should be deleted",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-aaaaa-guest-main")},
			},
			targetStacks:          []cloudformation.Stack{},
			expectedDeletedStacks: []string{},
		},
		{
			description: "no orphaned stacks, no need to delete",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-aaaaa-guest-main")},
			},
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-aaaaa-guest-recordset")},
			},
			expectedDeletedStacks: []string{},
		},
		{
			description: "one orphaned stack, needs to be deleted",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-aaaaa-guest-main")},
			},
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-bbbbb-guest-recordset")},
			},
			expectedDeletedStacks: []string{
				"cluster-bbbbb-guest-recordset",
			},
		},
		{
			description: "multiple orphaned stack, need to be deleted",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-aaaaa-guest-main")},
			},
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-bbbbb-guest-recordset")},
				cloudformation.Stack{StackName: aws.String("cluster-ccccc-guest-main")},
			},
			expectedDeletedStacks: []string{
				"cluster-bbbbb-guest-recordset",
				"cluster-ccccc-guest-main",
			},
		},
		{
			description: "mixed orphaned and not-orphaned stacks",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-aaaaa-guest-main")},
			},
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-bbbbb-guest-recordset")},
				cloudformation.Stack{StackName: aws.String("cluster-aaaaa-guest-recordset")},
				cloudformation.Stack{StackName: aws.String("cluster-ccccc-guest-main")},
			},
			expectedDeletedStacks: []string{
				"cluster-bbbbb-guest-recordset",
				"cluster-ccccc-guest-main",
			},
		},
	}

	logger, _ := micrologger.New(micrologger.Config{IOWriter: ioutil.Discard})
	sourceClient := &sourceClientMock{}
	targetClient := &targetClientMock{}

	c := &Config{
		Logger:       logger,
		Installation: "test",
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

// TestDeleteOrphanTargetStacks_Statuses tests Manager.deleteOrphanTargetStacks
// s: filterOut(delete_complete)
// s: nil or delete_complete
// t: not nil and not delete_complete
func TestDeleteOrphanTargetStacks_Statuses(t *testing.T) {
	var (
		installation = "installation"
		zoneID       = "zoneID"
		zoneName     = "zoneName"
	)

	tcs := []struct {
		description  string
		status       string
		expectDelete bool
	}{
		{
			"delete stack when source status is delete complete",
			cloudformation.StackStatusDeleteComplete,
			false,
		},
		{
			"do not delete stack when source status is delete in progress",
			cloudformation.StackStatusDeleteInProgress,
			true,
		},
		{
			"dot not delete stack when source status is create complete",
			cloudformation.StackStatusCreateComplete,
			true,
		},
		{
			"do not delete stack when source status is update complete",
			cloudformation.StackStatusUpdateComplete,
			true,
		},
		{
			"do not delete stack when source status is rollback complete",
			cloudformation.StackStatusRollbackComplete,
			true,
		},
		{
			"do not delete stack when source status is update rollback complete",
			cloudformation.StackStatusUpdateRollbackComplete,
			true,
		},
		{
			"do not delete stack when source status is create in progress",
			cloudformation.StackStatusCreateInProgress,
			true,
		},
		{
			"do not delete stack when source status is create failed",
			cloudformation.StackStatusCreateFailed,
			true,
		},
		{
			"do not delete stack when source status is rollback in progress",
			cloudformation.StackStatusRollbackInProgress,
			true,
		},
		{
			"do not delete stack when source status is rollback failed",
			cloudformation.StackStatusRollbackFailed,
			true,
		},
		{
			"do not delete stack when source status is delete failed",
			cloudformation.StackStatusDeleteFailed,
			true,
		},
		{
			"do not delete stack when source status is update in progress",
			cloudformation.StackStatusUpdateInProgress,
			true,
		},
		{
			"do not delete stack when source status is udpdate complete cleanup in progress",
			cloudformation.StackStatusUpdateCompleteCleanupInProgress,
			true,
		},
		{
			"do not delete stack when source status is update rollback in progress",
			cloudformation.StackStatusUpdateRollbackInProgress,
			true,
		},
		{
			"do not delete stack when source status is update rollback failed",
			cloudformation.StackStatusUpdateRollbackFailed,
			true,
		},
		{
			"do not delete stack when source status is update rollback complete cleanup in progress",
			cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
			true,
		},
		{
			"do not delete stack when source status is review in progress",
			cloudformation.StackStatusReviewInProgress,
			true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			logger, err := micrologger.New(micrologger.Config{IOWriter: ioutil.Discard})
			if err != nil {
				t.Fatalf("micrologger.New: %v", err)
			}

			targetStacks := []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(tc.status),
					Tags: []*cloudformation.Tag{
						&cloudformation.Tag{
							Key:   aws.String(installationTag),
							Value: aws.String(installation),
						},
					},
				},
			}

			sourceClient := newSourceWithStacks(nil)
			targetClient := newTargetWithStacks(targetStacks)

			c := &Config{
				Logger:               logger,
				Installation:         installation,
				SourceClient:         sourceClient,
				TargetClient:         targetClient,
				TargetHostedZoneID:   zoneID,
				TargetHostedZoneName: zoneName,
			}
			m, err := NewManager(c)
			if err != nil {
				t.Fatalf("NewManager: %v", err)
			}

			err = m.deleteOrphanTargetStacks(nil, targetStacks)
			if err != nil {
				t.Fatalf("m.Sync: %v", err)
			}

			if tc.expectDelete && len(targetClient.deletedStacks) <= 0 {
				t.Errorf("delete expected, got nothing")
			} else if !tc.expectDelete && len(targetClient.deletedStacks) > 0 {
				t.Errorf("no delete expected, got %v", targetClient.deletedStacks)
			}
		})
	}
}
