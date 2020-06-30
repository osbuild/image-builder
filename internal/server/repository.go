package server

import (
	"encoding/json"
	"os"

	"github.com/osbuild/osbuild-installer/internal/cloudapi"
)

func RepositoriesForImage(distro string, arch string) ([]cloudapi.Repository, error) {
	confPaths := [2]string{"/usr/share/osbuild-installer", "."}
	path := "/repositories/" + distro + ".json"

	var f *os.File
	var err error
	for _, confPath := range confPaths {
		f, err = os.Open(confPath + path)
		if err == nil {
			break
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reposMap map[string][]cloudapi.Repository
	err = json.NewDecoder(f).Decode(&reposMap)
	if err != nil {
		return nil, err
	}

	return reposMap[arch], nil
}
