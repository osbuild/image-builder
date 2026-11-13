package main

import (
	"errors"
	"testing"
	"time"
)

func TestWaitForSSHAuthenticationSucceeds(t *testing.T) {
	attempts := 0
	sleeps := 0
	err := waitForSSHAuthentication(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("not ready")
		}
		return nil
	}, func(delay time.Duration) {
		sleeps++
		if delay != sshAuthenticationRetryInterval {
			t.Errorf("unexpected retry delay: %s", delay)
		}
	})
	if err != nil {
		t.Fatalf("waitForSSHAuthentication() error = %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
	if sleeps != 2 {
		t.Errorf("sleeps = %d, want 2", sleeps)
	}
}

func TestWaitForSSHAuthenticationTimesOut(t *testing.T) {
	attempts := 0
	sleeps := 0
	err := waitForSSHAuthentication(func() error {
		attempts++
		return errors.New("not ready")
	}, func(time.Duration) {
		sleeps++
	})
	if err == nil {
		t.Fatal("waitForSSHAuthentication() error = nil, want timeout")
	}
	if attempts != sshAuthenticationRetries+1 {
		t.Errorf("attempts = %d, want %d", attempts, sshAuthenticationRetries+1)
	}
	if sleeps != sshAuthenticationRetries {
		t.Errorf("sleeps = %d, want %d", sleeps, sshAuthenticationRetries)
	}
}
