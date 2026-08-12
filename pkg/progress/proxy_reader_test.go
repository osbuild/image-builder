package progress_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/pkg/progress"
)

func TestProxyReader(t *testing.T) {
	var progressBuf bytes.Buffer
	restore := progress.MockOsStderr(&progressBuf)
	defer restore()

	pbar, err := progress.NewDebugProgressBar()
	assert.NoError(t, err)

	var readBuf bytes.Buffer
	size, err := readBuf.Write([]byte("duck"))
	assert.NoError(t, err)
	assert.Equal(t, 4, size)
	proxyReader, err := progress.NewProxyReader(bytes.NewReader(readBuf.Bytes()), size, pbar)
	assert.NoError(t, err)

	out := make([]byte, 256)
	nRead, err := proxyReader.Read(out)
	assert.NoError(t, err)
	assert.Equal(t, 4, nRead)
	assert.Equal(t, "duck", string(out[:nRead]))

	lines := strings.Split(progressBuf.String(), "\n")
	assert.Equal(t, "[0 / 4] Uploading", lines[0])
	assert.Equal(t, "[4 / 4] ", lines[1])
}
