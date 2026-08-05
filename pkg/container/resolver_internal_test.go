package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Smoke tests for the SetAuthFilePath method on the Resolver interface.
// These need to be in the container package because they access internal fields.

func TestAsyncResolverSetAuthFilePath(t *testing.T) {
	resolver := NewResolver("amd64")
	resolver.SetAuthFilePath("/path/to/auth.json")
	assert.Equal(t, "/path/to/auth.json", resolver.AuthFilePath)
}

func TestBlockingResolverSetAuthFilePath(t *testing.T) {
	resolver := NewBlockingResolver("amd64")
	resolver.SetAuthFilePath("/path/to/auth.json")
	assert.Equal(t, "/path/to/auth.json", resolver.(*blockingResolver).AuthFilePath)
}
