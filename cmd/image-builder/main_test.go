package main_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	testrepos "github.com/osbuild/images/test/data/repositories"

	"github.com/osbuild/image-builder-cli/cmd/image-builder"
)

func init() {
	// silence logrus by default, it is quite verbose
	logrus.SetLevel(logrus.WarnLevel)
}

func TestListImagesNoArguments(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	for _, args := range [][]string{nil, []string{"--output=text"}} {
		restore = main.MockOsArgs(append([]string{"list-images"}, args...))
		defer restore()

		var fakeStdout bytes.Buffer
		restore = main.MockOsStdout(&fakeStdout)
		defer restore()

		err := main.Run()
		assert.NoError(t, err)
		// we expect at least this canary
		assert.Contains(t, fakeStdout.String(), "rhel-10.0 type:qcow2 arch:x86_64\n")
		// output is sorted, i.e. 8.9 comes before 8.10
		assert.Regexp(t, `(?ms)rhel-8.9.*rhel-8.10`, fakeStdout.String())
	}
}

func TestListImagesNoArgsOutputJSON(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs([]string{"list-images", "--output=json"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.NoError(t, err)

	// smoke test only, we expect valid json and at least the
	// distro/arch/image_type keys in the json
	var jo []map[string]interface{}
	err = json.Unmarshal(fakeStdout.Bytes(), &jo)
	assert.NoError(t, err)
	res := jo[0]
	for _, key := range []string{"distro", "arch", "image_type"} {
		assert.NotNil(t, res[key])
	}
}

func TestListImagesFilteringSmoke(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs([]string{"list-images", "--filter=centos*"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.NoError(t, err)
	// we have centos
	assert.Contains(t, fakeStdout.String(), "centos-9 type:qcow2 arch:x86_64\n")
	// but not rhel
	assert.NotContains(t, fakeStdout.String(), "rhel")
}

func TestBadCmdErrorsNoExtraCobraNoise(t *testing.T) {
	var fakeStderr bytes.Buffer
	restore := main.MockOsStderr(&fakeStderr)
	defer restore()

	restore = main.MockOsArgs([]string{"bad-command"})
	defer restore()

	err := main.Run()
	assert.EqualError(t, err, `unknown command "bad-command" for "image-builder"`)
	// no extra output from cobra
	assert.Equal(t, "", fakeStderr.String())
}

func TestListImagesOverrideDatadir(t *testing.T) {
	restore := main.MockOsArgs([]string{"--datadir=/this/path/does/not/exist", "list-images"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.EqualError(t, err, `no repositories found in the given paths: [/this/path/does/not/exist]`)
}

func TestListImagesErrorsOnExtraArgs(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs(append([]string{"list-images"}, "extra-arg"))
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.EqualError(t, err, `unknown command "extra-arg" for "image-builder list-images"`)
}
