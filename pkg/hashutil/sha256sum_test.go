package hashutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/pkg/hashutil"
)

func TestSha256sum(t *testing.T) {
	for _, tc := range []struct {
		inp      string
		expected string
	}{
		// test vectors from
		// https://www.di-mgt.com.au/sha_testvectors.html
		{
			"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}, {
			"abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		}, {
			"abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq",
			"248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1",
		},
	} {
		t.Run(tc.inp, func(t *testing.T) {
			tmpf := filepath.Join(t.TempDir(), "inp.txt")
			err := os.WriteFile(tmpf, []byte(tc.inp), 0644)
			assert.NoError(t, err)
			hash, err := hashutil.Sha256sum(tmpf)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, hash)
		})
	}
}

func TestSha256sumError(t *testing.T) {
	_, err := hashutil.Sha256sum("non-existing-file")
	assert.EqualError(t, err, `open non-existing-file: no such file or directory`)
}
