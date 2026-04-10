package remotefile

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSingleInputResolver(t *testing.T) {
	server := makeTestServer()
	url := server.URL + "/key1"

	resolver := NewResolver(context.Background())

	expectedOutput := Spec{
		URL:     url,
		Content: []byte("key1\n"),
	}

	resolver.Add(url)

	resultItems, err := resolver.Finish()
	assert.NoError(t, err)
	assert.Contains(t, resultItems, expectedOutput)
}

func TestMultiInputResolver(t *testing.T) {
	server := makeTestServer()

	urlOne := server.URL + "/key1"
	urlTwo := server.URL + "/key2"

	expectedOutputOne := Spec{
		URL:     urlOne,
		Content: []byte("key1\n"),
	}

	expectedOutputTwo := Spec{
		URL:     urlTwo,
		Content: []byte("key2\n"),
	}

	resolver := NewResolver(context.Background())

	resolver.Add(urlOne)
	resolver.Add(urlTwo)

	resultItems, err := resolver.Finish()
	assert.NoError(t, err)

	assert.Contains(t, resultItems, expectedOutputOne)
	assert.Contains(t, resultItems, expectedOutputTwo)
}

func TestInvalidInputResolver(t *testing.T) {
	url := ""

	resolver := NewResolver(context.Background())

	resolver.Add(url)

	expectedErr := fmt.Errorf("File resolver: url is required")

	resultItems, err := resolver.Finish()
	assert.Error(t, err)
	assert.Equal(t, fmt.Errorf("failed to resolve remote files: %s", expectedErr.Error()), err)
	assert.Len(t, resultItems, 0)
}

func TestMultiInvalidInputResolver(t *testing.T) {
	urlOne := ""
	urlTwo := "hello"

	resolver := NewResolver(context.Background())

	resolver.Add(urlOne)
	resolver.Add(urlTwo)

	expectedErrMessageOne := "File resolver: url is required"
	expectedErrMessageTwo := fmt.Sprintf("File resolver: invalid url %s", urlTwo)

	resultItems, err := resolver.Finish()
	assert.Error(t, err)
	assert.Equal(t, fmt.Errorf("failed to resolve remote files: %s; %s", expectedErrMessageTwo, expectedErrMessageOne), err)

	assert.Len(t, resultItems, 0)
}
