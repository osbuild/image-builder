package oci

import (
	"encoding/json"
	"fmt"

	"github.com/oracle/oci-go-sdk/v54/core"
)

// ParseShapeConfig decodes OCI flex shape JSON like the oci CLI --shape-config flag.
func ParseShapeConfig(raw string) (*core.LaunchInstanceShapeConfigDetails, error) {
	if raw == "" {
		return nil, nil
	}
	var parsed struct {
		Ocpus       *float32 `json:"ocpus"`
		MemoryInGBs *float32 `json:"memoryInGBs"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("ParseShapeConfig: %w", err)
	}
	return &core.LaunchInstanceShapeConfigDetails{
		Ocpus:       parsed.Ocpus,
		MemoryInGBs: parsed.MemoryInGBs,
	}, nil
}
