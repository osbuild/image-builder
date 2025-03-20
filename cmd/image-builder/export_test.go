package main

import (
	"fmt"
	"io"
	"os"

	"github.com/osbuild/images/pkg/cloud"
	"github.com/osbuild/images/pkg/cloud/awscloud"
	"github.com/osbuild/images/pkg/reporegistry"
)

var (
	GetOneImage     = getOneImage
	Run             = run
	FindDistro      = findDistro
	DescribeImage   = describeImage
	ProgressFromCmd = progressFromCmd
	BasenameFor     = basenameFor
)

func MockOsArgs(new []string) (restore func()) {
	saved := os.Args
	os.Args = append([]string{"argv0"}, new...)
	return func() {
		os.Args = saved
	}
}

func MockOsStdout(new io.Writer) (restore func()) {
	saved := osStdout
	osStdout = new
	return func() {
		osStdout = saved
	}
}

func MockOsStderr(new io.Writer) (restore func()) {
	saved := osStderr
	osStderr = new
	return func() {
		osStderr = saved
	}
}

func MockNewRepoRegistry(f func() (*reporegistry.RepoRegistry, error)) (restore func()) {
	saved := newRepoRegistry
	newRepoRegistry = func(dataDir string, extraRepos []string) (*reporegistry.RepoRegistry, error) {
		if dataDir != "" {
			panic(fmt.Sprintf("cannot use custom dataDir %v in mock", dataDir))
		}
		return f()
	}
	return func() {
		newRepoRegistry = saved
	}
}

func MockDistroGetHostDistroName(f func() (string, error)) (restore func()) {
	saved := distroGetHostDistroName
	distroGetHostDistroName = f
	return func() {
		distroGetHostDistroName = saved
	}
}

func MockAwscloudNewUploader(f func(string, string, string, *awscloud.UploaderOptions) (cloud.Uploader, error)) (restore func()) {
	saved := awscloudNewUploader
	awscloudNewUploader = f
	return func() {
		awscloudNewUploader = saved
	}
}
