package distribution

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDistroRegistry_List(t *testing.T) {
	allDistros := []string{"rhel-8", "rhel-84", "rhel-85", "rhel-86", "rhel-9", "rhel-90", "centos-8", "centos-9"}
	notEntitledDistros := []string{"centos-8", "centos-9"}

	dr, err := LoadDistroRegistry("../../distributions")
	require.NoError(t, err)

	result := dr.Available(true).List()
	require.Len(t, result, len(allDistros))
	for _, distro := range result {
		require.Contains(t, allDistros, distro.Distribution.Name)
	}

	result = dr.Available(false).List()
	require.Len(t, result, len(notEntitledDistros))
	for _, distro := range result {
		require.Contains(t, notEntitledDistros, distro.Distribution.Name)
	}
}
