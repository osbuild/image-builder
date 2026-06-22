package shutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/pkg/shutil"
)

func TestShlexQuote(t *testing.T) {
	assert.Equal(t, `''`, shutil.Quote(""))
	assert.Equal(t, `'test file name'`, shutil.Quote(`test file name`))
	unsafe := `Robert'); DROP TABLE`
	assert.Equal(t, `'Robert'"'"'); DROP TABLE'`, shutil.Quote(unsafe))
}
