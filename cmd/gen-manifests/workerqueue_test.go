package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWorkerQueueRaceConditions tests the workerQueue for potential race conditions
// when running with -race flag
func TestWorkerQueueRaceConditions(t *testing.T) {
	const (
		nworkers = 10
		njobs    = 100
	)

	wq := newWorkerQueue(nworkers, njobs)
	wq.start()

	// Shared state that jobs will access
	var (
		counter    int32
		sharedMap  = make(map[string]int)
		mapMutex   sync.Mutex
		jobResults []string
		resultMu   sync.Mutex
	)

	// Submit jobs that access shared state
	for i := 0; i < njobs; i++ {
		jobID := i
		job := func(msgq chan string) error {
			// Simulate some work
			time.Sleep(time.Millisecond)

			// Atomic operation - should be safe
			atomic.AddInt32(&counter, 1)

			// Map access with mutex - should be safe
			mapMutex.Lock()
			sharedMap[fmt.Sprintf("job-%d", jobID)] = jobID
			mapMutex.Unlock()

			// Append to slice with mutex - should be safe
			resultMu.Lock()
			jobResults = append(jobResults, fmt.Sprintf("result-%d", jobID))
			resultMu.Unlock()

			// Send message
			msgq <- fmt.Sprintf("Job %d completed", jobID)
			return nil
		}
		wq.submitJob(job)
	}

	// Wait for all jobs to complete
	errors := wq.wait()

	// Verify no errors occurred
	if len(errors) > 0 {
		t.Errorf("Expected no errors, got %d errors: %v", len(errors), errors)
	}

	// Verify results
	if atomic.LoadInt32(&counter) != njobs {
		t.Errorf("Expected counter to be %d, got %d", njobs, counter)
	}

	mapMutex.Lock()
	if len(sharedMap) != njobs {
		t.Errorf("Expected shared map to have %d entries, got %d", njobs, len(sharedMap))
	}
	mapMutex.Unlock()

	resultMu.Lock()
	if len(jobResults) != njobs {
		t.Errorf("Expected job results to have %d entries, got %d", njobs, len(jobResults))
	}
	resultMu.Unlock()
}

// TestWorkerQueueWithErrors tests error collection in the workerQueue
func TestWorkerQueueWithErrors(t *testing.T) {
	const (
		nworkers    = 5
		njobs       = 20
		failingJobs = 5
	)

	wq := newWorkerQueue(nworkers, njobs)
	wq.start()

	// Submit jobs, some of which will fail
	for i := 0; i < njobs; i++ {
		jobID := i
		job := func(msgq chan string) error {
			if jobID%4 == 0 {
				return fmt.Errorf("job %d failed", jobID)
			}
			msgq <- fmt.Sprintf("Job %d completed", jobID)
			return nil
		}
		wq.submitJob(job)
	}

	errors := wq.wait()

	// Verify we got the expected number of errors
	if len(errors) != failingJobs {
		t.Errorf("Expected %d errors, got %d", failingJobs, len(errors))
	}
}

// TestWorkerQueueMessageOrdering tests that messages are properly queued
// This test doesn't guarantee order but ensures no messages are lost
func TestWorkerQueueMessageOrdering(t *testing.T) {
	const (
		nworkers = 3
		njobs    = 30
	)

	wq := newWorkerQueue(nworkers, njobs)
	wq.start()

	messageCount := int32(0)

	// Submit jobs that send multiple messages
	for i := 0; i < njobs; i++ {
		jobID := i
		job := func(msgq chan string) error {
			msgq <- fmt.Sprintf("Job %d started", jobID)
			atomic.AddInt32(&messageCount, 1)
			time.Sleep(time.Millisecond)
			msgq <- fmt.Sprintf("Job %d finished", jobID)
			atomic.AddInt32(&messageCount, 1)
			return nil
		}
		wq.submitJob(job)
	}

	errors := wq.wait()

	if len(errors) > 0 {
		t.Errorf("Expected no errors, got %d errors", len(errors))
	}

	// Each job sends 2 messages
	expectedMessages := njobs * 2
	if int(atomic.LoadInt32(&messageCount)) != expectedMessages {
		t.Errorf("Expected %d messages, got %d", expectedMessages, messageCount)
	}
}

// TestWorkerQueueConcurrentAccess tests concurrent access patterns
// that might occur in manifest generation
func TestWorkerQueueConcurrentAccess(t *testing.T) {
	const (
		nworkers = 8
		njobs    = 50
	)

	wq := newWorkerQueue(nworkers, njobs)
	wq.start()

	// Simulate the pattern used in gen-manifests where multiple jobs
	// might access shared data structures
	type manifestData struct {
		mu      sync.Mutex
		results map[string]string
	}

	data := &manifestData{
		results: make(map[string]string),
	}

	for i := 0; i < njobs; i++ {
		jobID := i
		job := func(msgq chan string) error {
			// Simulate manifest generation
			manifestName := fmt.Sprintf("manifest-%d", jobID)
			manifestContent := fmt.Sprintf("content-%d", jobID)

			// Concurrent write to shared map
			data.mu.Lock()
			data.results[manifestName] = manifestContent
			data.mu.Unlock()

			msgq <- fmt.Sprintf("Generated %s", manifestName)
			return nil
		}
		wq.submitJob(job)
	}

	errors := wq.wait()

	if len(errors) > 0 {
		t.Errorf("Expected no errors, got %d errors", len(errors))
	}

	// Verify all manifests were generated
	data.mu.Lock()
	if len(data.results) != njobs {
		t.Errorf("Expected %d manifests, got %d", njobs, len(data.results))
	}
	data.mu.Unlock()
}
