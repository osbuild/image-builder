package main

import (
	"fmt"
	"io"
	"os"

	"github.com/osbuild/image-builder/pkg/bootc"
	"github.com/osbuild/image-builder/pkg/cloud"
	"github.com/osbuild/image-builder/pkg/cloud/awscloud"
	"github.com/osbuild/image-builder/pkg/manifestgen"
	"github.com/osbuild/image-builder/pkg/reporegistry"
)

var (
	GetOneImage     = getOneImage
	GetAllImages    = getAllImages
	Run             = run
	FindDistro      = findDistro
	DescribeImage   = describeImage
	ProgressFromCmd = progressFromCmd
	BasenameFor     = basenameFor
	CacheDirForUid  = cacheDirForUid
)

type DescribeImgYAML describeImgYAML

func MockOsArgs(args []string) (restore func()) {
	saved := os.Args
	os.Args = append([]string{"argv0"}, args...)
	return func() {
		os.Args = saved
	}
}

func MockOsStdout(w io.Writer) (restore func()) {
	saved := osStdout
	osStdout = w
	return func() {
		osStdout = saved
	}
}

func MockOsStderr(w io.Writer) (restore func()) {
	saved := osStderr
	osStderr = w
	return func() {
		osStderr = saved
	}
}

func MockNewRepoRegistry(f func() (*reporegistry.RepoRegistry, error)) (restore func()) {
	saved := newRepoRegistry
	newRepoRegistry = func(repoDir string, extraRepos []string) (*reporegistry.RepoRegistry, error) {
		if repoDir != "" {
			panic(fmt.Sprintf("cannot use custom repoDir %v in mock", repoDir))
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

func MockBootcResolveInfo(f func(string) (*bootc.Info, error)) (restore func()) {
	saved := bootcResolveInfo
	bootcResolveInfo = f
	return func() {
		bootcResolveInfo = saved
	}
}

func MockGetCacheDir(f func() string) (restore func()) {
	saved := getCacheDir
	getCacheDir = f
	return func() {
		getCacheDir = saved
	}
}

func MockDirSize(f func(string) (int64, error)) (restore func()) {
	saved := dirSize
	dirSize = f
	return func() {
		dirSize = saved
	}
}

func MockManifestgenDepsolver(fn manifestgen.DepsolveFunc) (restore func()) {
	saved := manifestgenDepsolver
	manifestgenDepsolver = fn
	return func() {
		manifestgenDepsolver = saved
	}
}

func MockManifestgenContainerResolver(fn manifestgen.ContainerResolverFunc) (restore func()) {
	saved := manifestgenContainerResolver
	manifestgenContainerResolver = fn
	return func() {
		manifestgenContainerResolver = saved
	}
}
