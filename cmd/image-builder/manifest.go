package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/image-builder/pkg/customizations/subscription"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/imagefilter"
	"github.com/osbuild/image-builder/pkg/manifestgen"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/ostree"
	"github.com/osbuild/image-builder/pkg/rhsm/facts"
	"github.com/osbuild/image-builder/pkg/sbom"

	"github.com/osbuild/image-builder/internal/blueprintload"
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

// XXX: just return []byte instead of using output writer
func generateManifest(repoDir string, extraRepos []string, img *imagefilter.Result, output io.Writer, opts *manifestOptions) error {
	repos, err := newRepoRegistry(repoDir, extraRepos)
	if err != nil {
		return err
	}
	manifestGenOpts := &opts.ManifestgenOptions
	if opts.WithSBOM {
		outputDir := basenameFor(img, opts.OutputDir)
		manifestGenOpts.SBOMWriter = func(filename string, content io.Reader, docType sbom.StandardType) error {
			filename = fmt.Sprintf("%s.%s", basenameFor(img, opts.OutputFilename), strings.SplitN(filename, ".", 2)[1])
			return fileWriter(outputDir, filename, content)
		}
	}
	if len(opts.ForceRepos) > 0 {
		forcedRepos, err := parseRepoURLs(opts.ForceRepos, "forced")
		if err != nil {
			return err
		}
		manifestGenOpts.OverrideRepos = forcedRepos
	}
	if opts.IgnoreWarnings {
		manifestGenOpts.WarningsOutput = os.Stderr
	}

	if opts.WithRPMList {
		outputDir := basenameFor(img, opts.OutputDir)
		manifestGenOpts.RPMListWriter = func(filename string, content io.Reader) error {
			filename = fmt.Sprintf("%s.%s", basenameFor(img, opts.OutputFilename), filename)
			return fileWriter(outputDir, filename, content)
		}
	}

	mg, err := manifestgen.New(repos, manifestGenOpts)
	if err != nil {
		return err
	}

	bp, err := blueprintload.Load(opts.BlueprintPath)
	if err != nil {
		return err
	}

	imgOpts := &distro.ImageOptions{
		Facts:        &facts.ImageOptions{APIType: facts.IBCLI_APITYPE},
		OSTree:       opts.Ostree,
		Subscription: opts.Subscription,
		Size:         opts.ImageSize,
		Bootc: &distro.BootcImageOptions{
			InstallerPayloadRef:      opts.BootcInstallerPayloadRef,
			OmitDefaultKernelArgs:    opts.BootcOmitDefaultKernelArgs,
			UseRemoteContainerSource: opts.BootcRemote,
		},
		Preview: opts.Preview,
	}

	mf, err := mg.Generate(bp, img.ImgType, imgOpts)
	if err != nil {
		return err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(mf), "", "    "); err != nil {
		return err
	}

	if _, err := pretty.WriteTo(output); err != nil {
		return err
	}
	if _, err := output.Write([]byte{'\n'}); err != nil {
		return err
	}
	return nil
}
