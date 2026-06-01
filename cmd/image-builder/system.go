package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

type systemStatus struct {
	System struct {
		Cache struct {
			Path string `yaml:"path" json:"path"`
		} `yaml:"cache" json:"cache"`
	} `yaml:"system" json:"system"`
}

func readSystemStatus() *systemStatus {
	ss := &systemStatus{}
	ss.System.Cache.Path = defaultCacheDir()
	return ss
}

func prettySystemStatus() string {
	var b strings.Builder

	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)

	enc.Encode(readSystemStatus())

	return b.String()
}

func jsonSystemStatus() string {
	b, err := json.MarshalIndent(readSystemStatus(), "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err)
	}
	return string(b) + "\n"
}
