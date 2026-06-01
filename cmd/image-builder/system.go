package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

type cacheMaxSize struct {
	set   bool
	value int64
}

func (c cacheMaxSize) MarshalJSON() ([]byte, error) {
	if !c.set {
		return json.Marshal("unknown")
	}
	if c.value == 0 {
		return json.Marshal("unlimited")
	}
	return json.Marshal(c.value)
}

func (c cacheMaxSize) MarshalYAML() (interface{}, error) {
	if !c.set {
		return "unknown", nil
	}
	if c.value == 0 {
		return "unlimited", nil
	}
	return c.value, nil
}

type systemStatus struct {
	System struct {
		Cache struct {
			Path    string       `yaml:"path" json:"path"`
			Size    int64        `yaml:"size" json:"size"`
			MaxSize cacheMaxSize `yaml:"max-size" json:"max-size"`
		} `yaml:"cache" json:"cache"`
	} `yaml:"system" json:"system"`
}

var dirSize = calcDirSize
var getCacheDir = defaultCacheDir

func calcDirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func readCacheMaxSize(cacheDir string) cacheMaxSize {
	data, err := os.ReadFile(filepath.Join(cacheDir, "cache.size"))
	if err != nil {
		return cacheMaxSize{}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return cacheMaxSize{}
	}
	return cacheMaxSize{set: true, value: n}
}

func readSystemStatus() *systemStatus {
	ss := &systemStatus{}
	ss.System.Cache.Path = getCacheDir()
	ss.System.Cache.MaxSize = readCacheMaxSize(ss.System.Cache.Path)
	size, err := dirSize(ss.System.Cache.Path)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "failed to calculate cache size: %v\n", err)
	}
	ss.System.Cache.Size = size
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
