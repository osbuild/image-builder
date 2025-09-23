package progress

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/osbuild"
)

type OSBuildOptions struct {
	StoreDir  string
	OutputDir string
	ExtraEnv  []string

	// BuildLog writes the osbuild output to the given writer
	BuildLog io.Writer

	CacheMaxSize int64

	Metrics bool
}

// XXX: merge variant back into images/pkg/osbuild/osbuild-exec.go
func RunOSBuild(pb ProgressBar, manifest []byte, exports []string, opts *OSBuildOptions) error {
	if opts == nil {
		opts = &OSBuildOptions{}
	}

	// To keep maximum compatibility keep the old behavior to run osbuild
	// directly and show all messages unless we have a "real" progress bar.
	//
	// This should ensure that e.g. "podman bootc" keeps working as it
	// is currently expecting the raw osbuild output. Once we double
	// checked with them we can remove the runOSBuildNoProgress() and
	// just run with the new runOSBuildWithProgress() helper.
	switch pb.(type) {
	case *terminalProgressBar, *debugProgressBar:
		return runOSBuildWithProgress(pb, manifest, exports, opts)
	default:
		return runOSBuildNoProgress(pb, manifest, exports, opts)
	}
}

func newOsbuildCmd(manifest []byte, exports []string, opts *OSBuildOptions) *exec.Cmd {
	cacheMaxSize := int64(20 * datasizes.GiB)
	if opts.CacheMaxSize != 0 {
		cacheMaxSize = opts.CacheMaxSize
	}
	cmd := exec.Command(
		osbuildCmd,
		"--store", opts.StoreDir,
		"--output-directory", opts.OutputDir,
		fmt.Sprintf("--cache-max-size=%v", cacheMaxSize),
		"-",
	)
	for _, export := range exports {
		cmd.Args = append(cmd.Args, "--export", export)
	}
	cmd.Env = append(os.Environ(), opts.ExtraEnv...)
	cmd.Stdin = bytes.NewBuffer(manifest)
	return cmd
}

func runOSBuildNoProgress(pb ProgressBar, manifest []byte, exports []string, opts *OSBuildOptions) error {
	var stdout, stderr io.Writer

	var writeMu sync.Mutex
	if opts.BuildLog == nil {
		// No external build log requested and we won't need an
		// internal one because all output goes directly to
		// stdout/stderr. This is for maximum compatibility with
		// the existing bootc-image-builder in "verbose" mode
		// where stdout, stderr come directly from osbuild.
		stdout = osStdout()
		stderr = osStderr()
	} else {
		// There is a slight wrinkle here: when requesting a
		// buildlog we can no longer write to separate
		// stdout/stderr streams without being racy and give
		// potential out-of-order output (which is very bad
		// and confusing in a log). The reason is that if
		// cmd.Std{out,err} are different "go" will start two
		// go-routine to monitor/copy those are racy when both
		// stdout,stderr output happens close together
		// (TestRunOSBuildWithBuildlog demos that). We cannot
		// have our cake and eat it so here we need to combine
		// osbuilds stderr into our stdout.
		mw := newSyncedWriter(&writeMu, io.MultiWriter(osStdout(), opts.BuildLog))
		stdout = mw
		stderr = mw
	}

	cmd := newOsbuildCmd(manifest, exports, opts)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running osbuild: %w", err)
	}
	return nil
}

var osbuildCmd = "osbuild"

func runOSBuildWithProgress(pb ProgressBar, manifest []byte, exports []string, opts *OSBuildOptions) (err error) {
	rp, wp, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("cannot create pipe for osbuild: %w", err)
	}
	defer rp.Close()
	defer wp.Close()

	cmd := newOsbuildCmd(manifest, exports, opts)
	cmd.Args = append(cmd.Args, "--monitor=JSONSeqMonitor")
	cmd.Args = append(cmd.Args, "--monitor-fd=3")

	var stdio bytes.Buffer
	var mw, buildLog io.Writer
	var writeMu sync.Mutex
	if opts.BuildLog != nil {
		mw = newSyncedWriter(&writeMu, io.MultiWriter(&stdio, opts.BuildLog))
		buildLog = newSyncedWriter(&writeMu, opts.BuildLog)
	} else {
		mw = &stdio
		buildLog = io.Discard
	}

	cmd.Stdout = mw
	cmd.Stderr = mw
	cmd.ExtraFiles = []*os.File{wp}

	osbuildStatus := osbuild.NewStatusScanner(rp)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting osbuild: %v", err)
	}
	wp.Close()
	defer func() {
		// Try to stop osbuild if we exit early, we are gentle
		// here to give osbuild the chance to release its
		// resources (like mounts in the buildroot). This is
		// best effort only (but also a pretty uncommon error
		// condition). If ProcessState is set the process has
		// already exited and we have nothing to do.
		if err != nil && cmd.Process != nil && cmd.ProcessState == nil {
			sigErr := cmd.Process.Signal(syscall.SIGINT)
			err = errors.Join(err, sigErr)
		}
	}()

	var tracesMsgs []string
	oss := osbuildStageMetrics{}
	for {
		st, err := osbuildStatus.Status()
		if err != nil {
			// This should never happen but if it does we try
			// to be helpful. We need to exit here (and kill
			// osbuild in the defer) or we would appear to be
			// handing as cmd.Wait() would wait to finish but
			// no progress or other message is reported. We
			// can also not (in the general case) recover as
			// the underlying osbuildStatus.scanner maybe in
			// an unrecoverable state (like ErrTooBig).
			return fmt.Errorf(`error parsing osbuild status, please report a bug and try with "--progress=verbose": %w`, err)
		}
		if st == nil {
			break
		}
		i := 0
		for p := st.Progress; p != nil; p = p.SubProgress {
			if err := pb.SetProgress(i, p.Message, p.Done, p.Total); err != nil {
				log.Printf("WARNING: cannot set progress: %v", err)
			}
			i++
		}
		// forward to user
		if st.Message != "" {
			pb.SetMessagef(st.Message)
		}

		// keep internal log for error reporting, forward to
		// external build log
		if st.Message != "" {
			tracesMsgs = append(tracesMsgs, st.Message)
			fmt.Fprintln(buildLog, st.Message)
		}
		if st.Trace != "" {
			tracesMsgs = append(tracesMsgs, st.Trace)
			fmt.Fprintln(buildLog, st.Trace)
		}

		// store metrics if requested
		if opts.Metrics {
			oss.Add(st)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error running osbuild: %w\nBuildLog:\n%s\nOutput:\n%s", err, strings.Join(tracesMsgs, "\n"), stdio.String())
	}

	// append metrics to the end message
	if opts.Metrics {
		pb.Write(oss.Bytes())
	}

	return nil
}
