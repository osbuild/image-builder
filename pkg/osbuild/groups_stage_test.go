package osbuild

import (
	"testing"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/stretchr/testify/assert"
)

func TestNewGroupsStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.groups",
		Options: &GroupsStageOptions{},
	}
	actualStage := NewGroupsStage(&GroupsStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestNewGroupsStageOptions(t *testing.T) {
	type testCase struct {
		groups     []users.Group
		expOptions *GroupsStageOptions
	}

	testCases := map[string]testCase{
		"empty": {
			groups:     nil,
			expOptions: nil,
		},
		"single": {
			groups: []users.Group{
				{
					Name: "osbuild",
					GID:  common.ToPtr(42),
				},
			},
			expOptions: &GroupsStageOptions{
				Groups: map[string]GroupsStageOptionsGroup{
					"osbuild": {
						GID: common.ToPtr(42),
					},
				},
				Force: true,
			},
		},
		"multi": {
			groups: []users.Group{
				{
					Name: "osbuild",
					GID:  common.ToPtr(42),
				},
				{
					Name: "auser",
					GID:  common.ToPtr(1004),
				},
				{
					Name: "mysql",
					GID:  common.ToPtr(959),
				},
			},
			expOptions: &GroupsStageOptions{
				Groups: map[string]GroupsStageOptionsGroup{
					"osbuild": {
						GID: common.ToPtr(42),
					},
					"auser": {
						GID: common.ToPtr(1004),
					},
					"mysql": {
						GID: common.ToPtr(959),
					},
				},
				Force: true,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			options := NewGroupsStageOptions(tc.groups)
			assert.Equal(t, tc.expOptions, options)
		})
	}
}
