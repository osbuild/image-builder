package manifesttest

import (
	"encoding/json"
	"fmt"
)

// PipelineNamesFrom will return all pipeline names from an osbuild
// json manifest. It will error on missing pipelines.
//
// TODO: move to images:pkg/manifesttest
func PipelineNamesFrom(osbuildManifest []byte) ([]string, error) {
	var manifest map[string]interface{}

	if err := json.Unmarshal(osbuildManifest, &manifest); err != nil {
		return nil, fmt.Errorf("cannot unmarshal manifest: %w", err)
	}
	if manifest["pipelines"] == nil {
		return nil, fmt.Errorf("cannot find any pipelines in %v", manifest)
	}
	pipelines := manifest["pipelines"].([]interface{})
	pipelineNames := make([]string, len(pipelines))
	for idx, pi := range pipelines {
		pipelineNames[idx] = pi.(map[string]interface{})["name"].(string)
	}
	return pipelineNames, nil
}
