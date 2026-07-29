package main

import (
	"io"
	"os"
	"path/filepath"

	"github.com/osbuild/image-builder/pkg/customizations/subscription"
	"github.com/osbuild/image-builder/pkg/manifestgen"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/ostree"
)

type manifestOptions struct {
	ManifestgenOptions manifestgen.Options

	OutputDir                  string
	OutputFilename             string
	BlueprintPath              string
	Ostree                     *ostree.ImageOptions
	BootcRef                   string
	BootcInstallerPayloadRef   string
	BootcOmitDefaultKernelArgs bool
	BootcRemote                bool
	ImageSize                  uint64
	Subscription               *subscription.ImageOptions
	RpmDownloader              osbuild.RpmDownloader
	WithSBOM                   bool
	WithRPMList                bool
	IgnoreWarnings             bool
	Preview                    *bool

	ForceRepos []string
}

func fileWriter(outputDir, filename string, content io.Reader) error {
	p := filepath.Join(outputDir, filename)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, content); err != nil {
		return err
	}

	return f.Sync()
}
