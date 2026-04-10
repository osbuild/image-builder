package test

import (
	"sync"
	"testing"
)

var (
	// registryMu protects access to the mutex map itself
	registryMu sync.Mutex
	// mutexRegistry stores a mutex for every unique global pointer
	mutexRegistry = make(map[any]*sync.Mutex)
)

// getMutexFor returns a dedicated mutex for a specific memory address
func getMutexFor(ptr any) *sync.Mutex {
	registryMu.Lock()
	defer registryMu.Unlock()

	if mu, ok := mutexRegistry[ptr]; ok {
		return mu
	}

	mu := &sync.Mutex{}
	mutexRegistry[ptr] = mu
	return mu
}

// MockGlobal provides a thread-safe way to swap a global variable.
// It only blocks other tests attempting to mock the SAME variable.
// It will block if a single test attempts to mock the same variable twice.
func MockGlobal[T any](t *testing.T, target *T, mock T) {
	t.Helper()

	// Get the mutex specific to this variable's memory address
	mu := getMutexFor(target)

	// Lock it—this test now "owns" this specific global
	mu.Lock()

	original := *target
	*target = mock

	t.Cleanup(func() {
		*target = original
		mu.Unlock() // Release only this specific lock
	})
}
