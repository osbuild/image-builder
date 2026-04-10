package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/osbuild/images/cmd/check-host-config/check"
)

var ErrTimeout = errors.New("timeout")

// runningWait emulates 'systemctl is-system-running --wait'
// It blocks until the system reaches "running" or fails on other states.
// It is required for older versions of systemd that don't support the option (EL8).
// It silences the global logger and only logs once per minute.
func runningWait(timeout time.Duration, ticks time.Duration) error {
	startTime := time.Now()
	ticker := time.NewTicker(ticks)
	defer ticker.Stop()

	for {
		// Check if timeout has been exceeded
		if time.Since(startTime) > timeout {
			return fmt.Errorf("%w: before waiting for running state: exceeded %v", ErrTimeout, timeout)
		}

		stdout, _, _, err := check.ExecString("systemctl", "is-system-running")
		if err != nil {
			// systemctl typically returns non-zero exit code for non-running states but on
			// older RHEL systems it returns zero exit code and outputs the state to stdout.
			// Wait for next tick before continuing
			<-ticker.C
			continue
		}

		switch stdout {
		case "initializing", "starting":
			// Wait for next tick before checking again
			<-ticker.C
			continue
		case "running":
			log.Println("System is running")
			return nil
		case "degraded":
			log.Println("System is degraded")
			return fmt.Errorf("systemctl returned degraded output")
		case "":
			log.Println("System is in unknown state")
			return fmt.Errorf("systemctl returned empty output")
		default:
			return fmt.Errorf("system is at non-running state: %q", stdout)
		}
	}
}

// listBadUnits returns systemd units that are still in the activating state.
// It calls systemctl list-units to get the list.
// This is only used in case of timeout to help with debugging.
func listBadUnits() []string {
	stdout, _, _, err := check.Exec("systemctl", "list-units", "--state=activating,failed", "--plain", "--no-legend", "--no-pager")
	if err != nil {
		return nil
	}
	out := stdout

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var units []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// First column is the unit name (everything before the first space)
		fields := strings.Fields(line)
		if len(fields) > 0 {
			units = append(units, fields[0])
		}
	}

	return units
}

func printUnitJournal(unit string) {
	stdout, _, _, _ := check.Exec("journalctl", "-u", unit, "-o", "cat")
	fmt.Fprintf(os.Stderr, "%s\n", stdout)
}
