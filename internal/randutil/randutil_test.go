package randutil_test

import (
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/internal/randutil"
)

func TestChoiceSimple(t *testing.T) {
	n := 128

	r1 := randutil.String(n)
	r2 := randutil.String(n)
	assert.NotEqual(t, r1, r2)
	assert.Len(t, r1, n)
	assert.Len(t, r2, n)
	// by default we pick lower,upper,digit,symbols, ensure
	// we do not stray from this range
	defaultChars := randutil.AsciiLower + randutil.AsciiUpper + randutil.AsciiDigit + randutil.AsciiSymbol
	for _, char := range r1 {
		assert.Contains(t, []rune(defaultChars), char)
	}
}

func TestChoiceHonorsSeq(t *testing.T) {
	result := randutil.String(128, randutil.AsciiDigit)
	for _, char := range result {
		assert.True(t, unicode.IsDigit(char))
	}
}
