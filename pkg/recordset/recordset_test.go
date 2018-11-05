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
		name                  string
		sourceStacks          []cloudformation.Stack
		targetStacks          []cloudformation.Stack
		expectedCreatedStacks []string
	}{
		{
			name: "case 0: create 1 stack",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			targetStacks:          nil,
			expectedCreatedStacks: []string{"cluster-foo-guest-recordsets"},
		},
		{
			name: "case 1: create 2 stacks",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-bar-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			targetStacks:          nil,
			expectedCreatedStacks: []string{"cluster-foo-guest-recordsets", "cluster-bar-guest-recordsets"},
		},
		{
			name: "case 2: create 2 stacks out of 3",
			sourceStacks: []cloudformation.Stack{
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
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-bar-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			expectedCreatedStacks: []string{"cluster-foo-guest-recordsets", "cluster-baz-guest-recordsets"},
		},
		{
			name: "case 3: do not create already existing stack",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			expectedCreatedStacks: nil,
		},
		{
			name:         "case 4: do not create stack when there is no source",
			sourceStacks: nil,
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			expectedCreatedStacks: nil,
		},
		{
			name:                  "case 5: do not create stack when there is no source and target",
			sourceStacks:          nil,
			targetStacks:          nil,
			expectedCreatedStacks: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
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

// TestCreateMissingStacks_Statuses tests Manager.createMissingTargetStacks.
//
// Creation is only allowed when source stack has status *_COMPLETE except DELETE_COMPLETE.
func TestCreateMissingStacks_Statuses(t *testing.T) {
	var (
		installation = "installation"
		zoneID       = "zoneID"
		zoneName     = "zoneName"
	)

	tcs := []struct {
		name         string
		status       string
		expectCreate bool
	}{
		{
			name:         "case 0: create stack when source status is create complete",
			status:       cloudformation.StackStatusCreateComplete,
			expectCreate: true,
		},
		{
			name:         "case 1: create stack when source status is update complete",
			status:       cloudformation.StackStatusUpdateComplete,
			expectCreate: true,
		},
		{
			name:         "case 2: do not create stack when source status is rollback complete",
			status:       cloudformation.StackStatusRollbackComplete,
			expectCreate: false,
		},
		{
			name:         "case 3: do not create stack when source status is update rollback complete",
			status:       cloudformation.StackStatusUpdateRollbackComplete,
			expectCreate: false,
		},
		{
			name:         "case 4: do not create stack when source status is create in progress",
			status:       cloudformation.StackStatusCreateInProgress,
			expectCreate: false,
		},
		{
			name:         "case 5: do not create stack when source status is create failed",
			status:       cloudformation.StackStatusCreateFailed,
			expectCreate: false,
		},
		{
			name:         "case 6: do not create stack when source status is rollback in progress",
			status:       cloudformation.StackStatusRollbackInProgress,
			expectCreate: false,
		},
		{
			name:         "case 7: do not create stack when source status is rollback failed",
			status:       cloudformation.StackStatusRollbackFailed,
			expectCreate: false,
		},
		{
			name:         "case 8: do not create stack when source status is delete in progress",
			status:       cloudformation.StackStatusDeleteInProgress,
			expectCreate: false,
		},
		{
			name:         "case 9: do not create stack when source status is delete failed",
			status:       cloudformation.StackStatusDeleteFailed,
			expectCreate: false,
		},
		{
			name:         "case 10: do not create stack when source status is delete complete",
			status:       cloudformation.StackStatusDeleteComplete,
			expectCreate: false,
		},
		{
			name:         "case 11: do not create stack when source status is update in progress",
			status:       cloudformation.StackStatusUpdateInProgress,
			expectCreate: false,
		},
		{
			name:         "case 12: do not create stack when source status is udpdate complete cleanup in progress",
			status:       cloudformation.StackStatusUpdateCompleteCleanupInProgress,
			expectCreate: false,
		},
		{
			name:         "case 13: do not create stack when source status is update rollback in progress",
			status:       cloudformation.StackStatusUpdateRollbackInProgress,
			expectCreate: false,
		},
		{
			name:         "case 14: do not create stack when source status is update rollback failed",
			status:       cloudformation.StackStatusUpdateRollbackFailed,
			expectCreate: false,
		},
		{
			name:         "case 15: do not create stack when source status is update rollback complete cleanup in progress",
			status:       cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
			expectCreate: false,
		},
		{
			name:         "case 16: do not create stack when source status is review in progress",
			status:       cloudformation.StackStatusReviewInProgress,
			expectCreate: false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
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
		name                  string
		sourceStacks          []cloudformation.Stack
		targetStacks          []cloudformation.Stack
		expectedUpdatedStacks []string
	}{
		{
			name: "case 0: update 1 stack",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			expectedUpdatedStacks: []string{"cluster-foo-guest-recordsets"},
		},
		{
			name: "case 1: update 2 stacks",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-bar-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-bar-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			expectedUpdatedStacks: []string{"cluster-foo-guest-recordsets", "cluster-bar-guest-recordsets"},
		},
		{
			name: "case 2: update 2 stacks out of 3",
			sourceStacks: []cloudformation.Stack{
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
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackName:   aws.String("cluster-baz-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			expectedUpdatedStacks: []string{"cluster-foo-guest-recordsets", "cluster-baz-guest-recordsets"},
		},

		{
			name: "case 3: do not update missing target stack",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-main"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			targetStacks:          nil,
			expectedUpdatedStacks: nil,
		},
		{
			name:         "case 4: do not update missing source stack",
			sourceStacks: nil,
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{
					StackName:   aws.String("cluster-foo-guest-recordsets"),
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			expectedUpdatedStacks: nil,
		},
		{
			name:                  "case 5: do not update missing source and target stacks",
			sourceStacks:          nil,
			targetStacks:          nil,
			expectedUpdatedStacks: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
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
//
// Update is only allowed when source stack has status *_COMPLETE except DELETE_COMPLETE.
func TestUpdateCurrentTargetStacks_SourceStatuses(t *testing.T) {
	var (
		installation = "installation"
		zoneID       = "zoneID"
		zoneName     = "zoneName"
	)

	tcs := []struct {
		name         string
		status       string
		expectUpdate bool
	}{
		{
			name:         "case 0: update stack when source status is create complete",
			status:       cloudformation.StackStatusCreateComplete,
			expectUpdate: true,
		},
		{
			name:         "case 1: update stack when source status is update complete",
			status:       cloudformation.StackStatusUpdateComplete,
			expectUpdate: true,
		},
		{
			name:         "case 2: do not update stack when source status is rollback complete",
			status:       cloudformation.StackStatusRollbackComplete,
			expectUpdate: false,
		},
		{
			name:         "case 3: do not update stack when source status is update rollback complete",
			status:       cloudformation.StackStatusUpdateRollbackComplete,
			expectUpdate: false,
		},
		{
			name:         "case 4: do not update stack when source status is create in progress",
			status:       cloudformation.StackStatusCreateInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 5: do not update stack when source status is create failed",
			status:       cloudformation.StackStatusCreateFailed,
			expectUpdate: false,
		},
		{
			name:         "case 6: do not update stack when source status is rollback in progress",
			status:       cloudformation.StackStatusRollbackInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 7: do not update stack when source status is rollback failed",
			status:       cloudformation.StackStatusRollbackFailed,
			expectUpdate: false,
		},
		{
			name:         "case 8: do not update stack when source status is delete in progress",
			status:       cloudformation.StackStatusDeleteInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 9: do not update stack when source status is delete failed",
			status:       cloudformation.StackStatusDeleteFailed,
			expectUpdate: false,
		},
		{
			name:         "case 10: do not update stack when source status is delete complete",
			status:       cloudformation.StackStatusDeleteComplete,
			expectUpdate: false,
		},
		{
			name:         "case 11: do not update stack when source status is update in progress",
			status:       cloudformation.StackStatusUpdateInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 12: do not update stack when source status is udpdate complete cleanup in progress",
			status:       cloudformation.StackStatusUpdateCompleteCleanupInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 13: do not update stack when source status is update rollback in progress",
			status:       cloudformation.StackStatusUpdateRollbackInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 14: do not update stack when source status is update rollback failed",
			status:       cloudformation.StackStatusUpdateRollbackFailed,
			expectUpdate: false,
		},
		{
			name:         "case 15: do not update stack when source status is update rollback complete cleanup in progress",
			status:       cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 16: do not update stack when source status is review in progress",
			status:       cloudformation.StackStatusReviewInProgress,
			expectUpdate: false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
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
//
// Update is only allowed when target stack has status *_COMPLETE except DELETE_COMPLETE or *_FAILED.
func TestUpdateCurrentTargetStacks_TargetStatuses(t *testing.T) {
	var (
		installation = "installation"
		zoneID       = "zoneID"
		zoneName     = "zoneName"
	)

	tcs := []struct {
		name         string
		status       string
		expectUpdate bool
	}{
		{
			name:         "case 0: update stack when target status is create complete",
			status:       cloudformation.StackStatusCreateComplete,
			expectUpdate: true,
		},
		{
			name:         "case 1: update stack when target status is update complete",
			status:       cloudformation.StackStatusUpdateComplete,
			expectUpdate: true,
		},
		{
			name:         "case 2: update stack when target status is rollback complete",
			status:       cloudformation.StackStatusRollbackComplete,
			expectUpdate: true,
		},
		{
			name:         "case 3: update stack when target status is update rollback complete",
			status:       cloudformation.StackStatusUpdateRollbackComplete,
			expectUpdate: true,
		},
		{
			name:         "case 4: update stack when target status is create failed",
			status:       cloudformation.StackStatusCreateFailed,
			expectUpdate: true,
		},
		{
			name:         "case 5: update stack when target status is rollback failed",
			status:       cloudformation.StackStatusRollbackFailed,
			expectUpdate: true,
		},
		{
			name:         "case 6: update stack when target status is delete failed",
			status:       cloudformation.StackStatusDeleteFailed,
			expectUpdate: true,
		},
		{
			name:         "case 7: update stack when target status is update rollback failed",
			status:       cloudformation.StackStatusUpdateRollbackFailed,
			expectUpdate: true,
		},
		{
			name:         "case 8: do not update stack when target status is create in progress",
			status:       cloudformation.StackStatusCreateInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 9: do not update stack when target status is rollback in progress",
			status:       cloudformation.StackStatusRollbackInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 10: do not update stack when target status is delete in progress",
			status:       cloudformation.StackStatusDeleteInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 11: do not update stack when target status is delete complete",
			status:       cloudformation.StackStatusDeleteComplete,
			expectUpdate: false,
		},
		{
			name:         "case 12: do not update stack when target status is update in progress",
			status:       cloudformation.StackStatusUpdateInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 13: do not update stack when target status is udpdate complete cleanup in progress",
			status:       cloudformation.StackStatusUpdateCompleteCleanupInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 14: do not update stack when target status is update rollback in progress",
			status:       cloudformation.StackStatusUpdateRollbackInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 15: do not update stack when target status is update rollback complete cleanup in progress",
			status:       cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
			expectUpdate: false,
		},
		{
			name:         "case 16: do not update stack when target status is review in progress",
			status:       cloudformation.StackStatusReviewInProgress,
			expectUpdate: false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
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
		name                  string
		sourceStacks          []cloudformation.Stack
		targetStacks          []cloudformation.Stack
		expectedDeletedStacks []string
	}{
		{
			name:                  "empty target and source stacks, nothing should be deleted",
			sourceStacks:          []cloudformation.Stack{},
			targetStacks:          []cloudformation.Stack{},
			expectedDeletedStacks: []string{},
		},
		{
			name:         "empty source stack, all should be deleted",
			sourceStacks: []cloudformation.Stack{},
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-bbbbb-guest-recordset")},
			},
			expectedDeletedStacks: []string{
				"cluster-bbbbb-guest-recordset",
			},
		},
		{
			name: "empty target stacks, nothing should be deleted",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-aaaaa-guest-main")},
			},
			targetStacks:          []cloudformation.Stack{},
			expectedDeletedStacks: []string{},
		},
		{
			name: "no orphaned stacks, no need to delete",
			sourceStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-aaaaa-guest-main")},
			},
			targetStacks: []cloudformation.Stack{
				cloudformation.Stack{StackName: aws.String("cluster-aaaaa-guest-recordset")},
			},
			expectedDeletedStacks: []string{},
		},
		{
			name: "one orphaned stack, needs to be deleted",
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
			name: "multiple orphaned stack, need to be deleted",
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
			name: "mixed orphaned and not-orphaned stacks",
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
		t.Run(tc.name, func(t *testing.T) {
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
//
// Deletion is only allowed when source stack has status DELETE_COMPLETE.
func TestDeleteOrphanTargetStacks_Statuses(t *testing.T) {
	var (
		installation = "installation"
		zoneID       = "zoneID"
		zoneName     = "zoneName"
	)

	tcs := []struct {
		name         string
		status       string
		expectDelete bool
	}{
		{
			name:         "case 0: delete stack when source status is delete complete",
			status:       cloudformation.StackStatusDeleteComplete,
			expectDelete: true,
		},
		{
			name:         "case 1: do not delete stack when source status is delete in progress",
			status:       cloudformation.StackStatusDeleteInProgress,
			expectDelete: false,
		},
		{
			name:         "case 2: do not delete stack when source status is create complete",
			status:       cloudformation.StackStatusCreateComplete,
			expectDelete: false,
		},
		{
			name:         "case 3: do not delete stack when source status is update complete",
			status:       cloudformation.StackStatusUpdateComplete,
			expectDelete: false,
		},
		{
			name:         "case 4: do not delete stack when source status is rollback complete",
			status:       cloudformation.StackStatusRollbackComplete,
			expectDelete: false,
		},
		{
			name:         "case 5: do not delete stack when source status is update rollback complete",
			status:       cloudformation.StackStatusUpdateRollbackComplete,
			expectDelete: false,
		},
		{
			name:         "case 6: do not delete stack when source status is create in progress",
			status:       cloudformation.StackStatusCreateInProgress,
			expectDelete: false,
		},
		{
			name:         "case 7: do not delete stack when source status is create failed",
			status:       cloudformation.StackStatusCreateFailed,
			expectDelete: false,
		},
		{
			name:         "case 8: do not delete stack when source status is rollback in progress",
			status:       cloudformation.StackStatusRollbackInProgress,
			expectDelete: false,
		},
		{
			name:         "case 9: do not delete stack when source status is rollback failed",
			status:       cloudformation.StackStatusRollbackFailed,
			expectDelete: false,
		},
		{
			name:         "case 10: do not delete stack when source status is delete failed",
			status:       cloudformation.StackStatusDeleteFailed,
			expectDelete: false,
		},
		{
			name:         "case 11: do not delete stack when source status is update in progress",
			status:       cloudformation.StackStatusUpdateInProgress,
			expectDelete: false,
		},
		{
			name:         "case 12: do not delete stack when source status is udpdate complete cleanup in progress",
			status:       cloudformation.StackStatusUpdateCompleteCleanupInProgress,
			expectDelete: false,
		},
		{
			name:         "case 13: do not delete stack when source status is update rollback in progress",
			status:       cloudformation.StackStatusUpdateRollbackInProgress,
			expectDelete: false,
		},
		{
			name:         "case 14: do not delete stack when source status is update rollback failed",
			status:       cloudformation.StackStatusUpdateRollbackFailed,
			expectDelete: false,
		},
		{
			name:         "case 15: do not delete stack when source status is update rollback complete cleanup in progress",
			status:       cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
			expectDelete: false,
		},
		{
			name:         "case 16: do not delete stack when source status is review in progress",
			status:       cloudformation.StackStatusReviewInProgress,
			expectDelete: false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
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

			err = m.deleteOrphanTargetStacks(sourceStacks, targetStacks)
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

func TestFilterStacksByStatus(t *testing.T) {
	tcs := []struct {
		description string
		input       []cloudformation.Stack
		output      []cloudformation.Stack
		statuses    []string
	}{
		{
			"nil inputs",
			nil,
			[]cloudformation.Stack{},
			nil,
		},
		{
			"zero value inputs",
			[]cloudformation.Stack{},
			[]cloudformation.Stack{},
			[]string{},
		},
		{
			"zero filter gives zero output",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]cloudformation.Stack{},
			[]string{},
		},
		{
			"non matching filter gives zero output",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]cloudformation.Stack{},
			[]string{
				cloudformation.StackStatusDeleteComplete,
			},
		},
		{
			"one matching filter",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
			[]string{
				cloudformation.StackStatusCreateComplete,
			},
		},
		{
			"two matching filters",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]string{
				cloudformation.StackStatusCreateComplete,
				cloudformation.StackStatusUpdateComplete,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			output := filterStacksByStatus(tc.input, tc.statuses)
			if !reflect.DeepEqual(tc.output, output) {
				t.Errorf("expected %#v, got %#v", tc.output, output)
			}
		})
	}
}

func TestDropStacksByStatus(t *testing.T) {
	tcs := []struct {
		description string
		input       []cloudformation.Stack
		output      []cloudformation.Stack
		statuses    []string
	}{
		{
			"nil inputs",
			nil,
			[]cloudformation.Stack{},
			nil,
		},
		{
			"zero value inputs",
			[]cloudformation.Stack{},
			[]cloudformation.Stack{},
			[]string{},
		},
		{
			"no filter gives input as output",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]string{},
		},
		{
			"non matching filter gives input as output",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]string{
				cloudformation.StackStatusDeleteComplete,
			},
		},

		{
			"one matching filter",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]string{
				cloudformation.StackStatusCreateComplete,
			},
		},
		{
			"two matching filters",
			[]cloudformation.Stack{
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
				cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusUpdateComplete),
				},
			},
			[]cloudformation.Stack{},
			[]string{
				cloudformation.StackStatusCreateComplete,
				cloudformation.StackStatusUpdateComplete,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			output := dropStacksByStatus(tc.input, tc.statuses)
			if !reflect.DeepEqual(tc.output, output) {
				t.Errorf("expected %#v, got %#v", tc.output, output)
			}
		})
	}
}
