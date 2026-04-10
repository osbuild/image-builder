package osbuild

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRHSMFactsStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.rhsm.facts",
		Options: &RHSMFactsStageOptions{},
	}
	actualStage := NewRHSMFactsStage(&RHSMFactsStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestRHSMFactsStageJson(t *testing.T) {
	tests := []struct {
		Options    RHSMFactsStageOptions
		JsonString string
	}{
		{
			Options: RHSMFactsStageOptions{
				Facts: RHSMFacts{
					ApiType: "test-api",
				},
			},
			JsonString: fmt.Sprintf(`{"facts":{"image-builder.osbuild-composer.api-type":"%s"}}`, "test-api"),
		},
		{
			Options: RHSMFactsStageOptions{
				Facts: RHSMFacts{
					ApiType:           "test-api",
					OpenSCAPProfileID: "test-profile-id",
				},
			},
			JsonString: fmt.Sprintf(`{"facts":{"image-builder.osbuild-composer.api-type":"%s","image-builder.insights.compliance-profile-id":"%s"}}`, "test-api", "test-profile-id"),
		},
		{
			Options: RHSMFactsStageOptions{
				Facts: RHSMFacts{
					ApiType:            "test-api",
					OpenSCAPProfileID:  "test-profile-id",
					CompliancePolicyID: "test-compliance-policy-id",
				},
			},
			JsonString: fmt.Sprintf(`{"facts":{"image-builder.osbuild-composer.api-type":"%s","image-builder.insights.compliance-profile-id":"%s","image-builder.insights.compliance-policy-id":"%s"}}`, "test-api", "test-profile-id", "test-compliance-policy-id"),
		},
		{
			Options: RHSMFactsStageOptions{
				Facts: RHSMFacts{
					ApiType:     "test-api",
					BlueprintID: "123e4567-e89b-12d3-a456-426655440000",
				},
			},
			JsonString: fmt.Sprintf(`{"facts":{"image-builder.osbuild-composer.api-type":"%s","image-builder.blueprint-id":"%s"}}`, "test-api", "123e4567-e89b-12d3-a456-426655440000"),
		},
	}
	for _, test := range tests {
		marshaledJson, err := json.Marshal(test.Options)
		require.NoError(t, err, "failed to marshal JSON")
		require.Equal(t, test.JsonString, string(marshaledJson))

		var jsonOptions RHSMFactsStageOptions
		err = json.Unmarshal([]byte(test.JsonString), &jsonOptions)
		require.NoError(t, err, "failed to parse JSON")
		require.True(t, reflect.DeepEqual(test.Options, jsonOptions))
	}
}
