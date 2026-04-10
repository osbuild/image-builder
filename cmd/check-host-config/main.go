package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/osbuild/images/internal/buildconfig"
)

// waitForSystem waits until the system is reported by systemd as "running" or the timeout is reached.
func waitForSystem(timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}

	if err := runningWait(timeout, 15*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Error while waiting for system to be running: %v\n", err)
		if errors.Is(err, ErrTimeout) {
			if activatingUnits := listBadUnits(); len(activatingUnits) > 0 {
				fmt.Fprintf(os.Stderr, "Units still activating: %s\n", strings.Join(activatingUnits, " "))
				for _, unit := range activatingUnits {
					fmt.Fprintf(os.Stderr, "Unit %s journal:\n", unit)
					printUnitJournal(unit)
				}
			}
		}
		return err
	}
	return nil
}

// shouldRunOn returns whether a check should run on the current system.
func shouldRunOn(osRelease *check.OSRelease, runOn []string) bool {
	if len(runOn) == 0 || osRelease == nil {
		return true
	}

	currentID := strings.ToLower(strings.TrimSpace(osRelease.ID + "-" + osRelease.VersionID))
	var inclusions []string
	for _, entry := range runOn {
		entry = strings.TrimSpace(entry)
		if after, ok := strings.CutPrefix(entry, "!"); ok {
			if strings.ToLower(after) == currentID {
				return false
			}
		} else {
			inclusions = append(inclusions, strings.ToLower(entry))
		}
	}

	return len(inclusions) == 0 || slices.Contains(inclusions, currentID)
}

// runChecks runs all checks sequentially and processes their results.
func runChecks(checks []check.RegisteredCheck, config *buildconfig.BuildConfig, osRelease *check.OSRelease, quiet bool) bool {
	defer log.SetPrefix("")
	if quiet {
		log.SetOutput(io.Discard)
		defer log.SetOutput(os.Stdout)
	}

	var results check.SortedResults
	for _, chk := range checks {
		var err error
		meta := chk.Meta
		log.SetPrefix(meta.Name + ": ")

		switch {
		case !shouldRunOn(osRelease, meta.RunOn):
			err = check.Skip(osRelease.ID + "-" + osRelease.VersionID + " excluded via RunOn: " + strings.Join(meta.RunOn, ", "))
		case meta.TempDisabled != "":
			err = check.Skip("temporarily disabled: " + meta.TempDisabled)
		case meta.RequiresBlueprint && (config == nil || config.Blueprint == nil):
			err = check.Skip("no blueprint")
		case meta.RequiresCustomizations && (config == nil || config.Blueprint == nil || config.Blueprint.Customizations == nil):
			err = check.Skip("no customizations")
		default:
			err = chk.Func(meta, config)
		}

		results = append(results, check.Result{Meta: meta, Error: err})

		if err != nil {
			log.Println(err)
		}
	}

	log.SetOutput(os.Stdout)
	sort.Sort(results)
	var seenError bool
	for _, res := range results {
		err := res.Error
		icon := check.IconFor(err)

		switch err {
		case nil:
			fmt.Printf("%s %s: passed\n", icon, res.Meta.Name)
		default:
			if !check.IsSkip(err) && !check.IsWarning(err) {
				seenError = true
			}
			fmt.Printf("%s %s: %s\n", icon, res.Meta.Name, err)
		}
	}

	return !seenError
}

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	waitTimeout := flag.Duration("wait-timeout", 15*time.Minute, "timeout for waiting for system to be running (0 to skip)")
	quiet := flag.Bool("quiet", false, "less logging output")
	flag.Parse()
	configFile := flag.Arg(0)
	if configFile == "" {
		log.Fatalf("Missing build config file, usage: %s <config.json>", os.Args[0])
	}

	var config *buildconfig.BuildConfig
	var err error
	config, err = buildconfig.New(configFile, nil)
	if err != nil {
		log.Fatalf("Failed to load build config: %v\n", err)
	}

	if err := waitForSystem(*waitTimeout); err != nil {
		log.Fatalf("Problem during waiting for system to be running: %v\n", err)
	}

	osRelease, _ := check.ParseOSRelease("")
	if osRelease == nil {
		log.Println("Could not parse /etc/os-release, RunOn filtering disabled")
	}

	if !runChecks(checks, config, osRelease, *quiet) {
		log.Fatalf("Host check with config %q failed, return code 1\n", configFile)
	}
}
