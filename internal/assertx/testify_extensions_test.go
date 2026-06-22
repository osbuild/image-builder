package assertx_test

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/internal/assertx"
)

type mockTestingT struct {
	errs []string
}

func (mt *mockTestingT) Errorf(format string, args ...interface{}) {
	mt.errs = append(mt.errs, fmt.Sprintf(format, args...))
}

func TestAssertPanicWithErrorRegexpWrongType(t *testing.T) {
	mockT := &mockTestingT{}

	assertx.PanicsWithErrorRegexp(mockT, regexp.MustCompile("ignore"), func() {
		panic("panic-with-string")
	})
	assert.Equal(t, len(mockT.errs), 1)
	assert.Contains(t, mockT.errs[0], "should return an error but got: panic-with-string (type string)")
}

func TestAssertPanicWithErrorRegexpNoError(t *testing.T) {
	mockT := &mockTestingT{}

	assertx.PanicsWithErrorRegexp(mockT, regexp.MustCompile("ignore"), func() {
		// no error
	})
	assert.Equal(t, len(mockT.errs), 1)
	assert.Contains(t, mockT.errs[0], " should panic but did not")
}

func TestAssertPanicWithErrorRegexpHappy(t *testing.T) {
	assertx.PanicsWithErrorRegexp(t, regexp.MustCompile(`(some|other) error`), func() {
		panic(errors.New("some error"))
	})
}
