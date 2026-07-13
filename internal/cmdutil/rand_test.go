package cmdutil_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/cmdutil"
)

func TestNewRNGSeed(t *testing.T) {
	// env is global - run all tests in one function so they don't run in
	// parallel
	t.Run("default", func(t *testing.T) {
		t.Setenv(cmdutil.RNG_SEED_ENV_KEY, "")
		seed1, err := cmdutil.NewRNGSeed()
		require.Nil(t, err)
		require.IsType(t, int64(0), seed1)

		seed2, err := cmdutil.NewRNGSeed()
		require.Nil(t, err)
		require.IsType(t, int64(0), seed1)
		require.NotEqual(t, seed1, seed2) // 1/2^64 chance this will fail randomly
	})

	t.Run("happy", func(t *testing.T) {
		t.Setenv(cmdutil.RNG_SEED_ENV_KEY, "1234")
		seed, err := cmdutil.NewRNGSeed()
		require.Nil(t, err)
		assert.Equal(t, seed, int64(1234))
	})

	t.Run("error", func(t *testing.T) {
		t.Setenv(cmdutil.RNG_SEED_ENV_KEY, "NaN")
		_, err := cmdutil.NewRNGSeed()
		require.EqualError(t, err, fmt.Sprintf(`failed to parse %s: strconv.ParseInt: parsing "NaN": invalid syntax`, cmdutil.RNG_SEED_ENV_KEY))
	})
}
