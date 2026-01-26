package distro

import (
	"os"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetHostDistroName(t *testing.T) {
	backup := getHostDistroNameTree
	defer func() { getHostDistroNameTree = backup }()
	getHostDistroNameTree = t.TempDir()

	require.NoError(t, os.MkdirAll(path.Join(getHostDistroNameTree, "etc"), 0755))
	require.NoError(t,
		os.WriteFile(path.Join(getHostDistroNameTree, "etc/os-release"), []byte("ID=toucanOS\nVERSION_ID=42\n"), 0600),
	)

	name, err := GetHostDistroName()
	require.NoError(t, err)
	require.Equal(t, "toucanOS-42", name)
}

func TestGetHostDistroNameUnhappy(t *testing.T) {
	backup := getHostDistroNameTree
	defer func() { getHostDistroNameTree = backup }()
	getHostDistroNameTree = t.TempDir()

	require.NoError(t, os.MkdirAll(path.Join(getHostDistroNameTree, "etc"), 0755))

	// no file at all
	_, err := GetHostDistroName()
	require.ErrorContains(t, err, "cannot get the host distro name: failed to read os-release")

	// missing ID
	require.NoError(t,
		os.WriteFile(path.Join(getHostDistroNameTree, "etc/os-release"), []byte("VERSION_ID=toucanOS\n"), 0600),
	)
	_, err = GetHostDistroName()
	require.ErrorContains(t, err, "cannot get the host distro name: missing ID field")

	// missing VERSION_ID
	require.NoError(t,
		os.WriteFile(path.Join(getHostDistroNameTree, "etc/os-release"), []byte("ID=42\n"), 0600),
	)
	_, err = GetHostDistroName()
	require.ErrorContains(t, err, "cannot get the host distro name: missing VERSION_ID field")
}

func TestGetHostDistroNameKitten(t *testing.T) {
	backup := getHostDistroNameTree
	defer func() { getHostDistroNameTree = backup }()
	getHostDistroNameTree = t.TempDir()

	require.NoError(t, os.MkdirAll(path.Join(getHostDistroNameTree, "etc"), 0755))

	var cases = []struct {
		Input string
		ID    string
	}{
		{"ID=almalinux\nVERSION_ID=9.5\n", "almalinux-9.5"},
		{"ID=almalinux\nVERSION_ID=10.0\n", "almalinux-10.0"},
		{"ID=almalinux\nVERSION_ID=11\n", "almalinux-11"},
		{"ID=almalinux\nVERSION_ID=10\n", "almalinux_kitten-10"}, // note the replacement!
		{"ID=centos\nVERSION_ID=10\n", "centos-10"},
		{"ID=fedora\nVERSION_ID=42\n", "fedora-42"},
	}

	for _, c := range cases {
		require.NoError(t,
			os.WriteFile(path.Join(getHostDistroNameTree, "etc/os-release"), []byte(c.Input), 0600),
		)

		name, err := GetHostDistroName()
		require.NoError(t, err)
		require.Equal(t, c.ID, name)
	}
}

// Oracle Linux host distro names have the VERSION_ID minor number removed
func TestGetHostDistroNameOracleLinux(t *testing.T) {
	backup := getHostDistroNameTree
	defer func() { getHostDistroNameTree = backup }()
	getHostDistroNameTree = t.TempDir()

	require.NoError(t, os.MkdirAll(path.Join(getHostDistroNameTree, "etc"), 0755))

	var cases = []struct {
		Input string
		ID    string
	}{
		{"ID=ol\nVERSION_ID=9.7\n", "ol-9"},
		{"ID=ol\nVERSION_ID=10.1\n", "ol-10"},
	}

	for _, c := range cases {
		require.NoError(t,
			os.WriteFile(path.Join(getHostDistroNameTree, "etc/os-release"), []byte(c.Input), 0600),
		)

		name, err := GetHostDistroName()
		require.NoError(t, err)
		require.Equal(t, c.ID, name)
	}
}

func TestOSRelease(t *testing.T) {
	var cases = []struct {
		Input     string
		OSRelease map[string]string
	}{
		{
			``,
			map[string]string{},
		},
		{
			`NAME=Fedora
VERSION="30 (Workstation Edition)"
ID=fedora
VERSION_ID=30
VERSION_CODENAME=""
PLATFORM_ID="platform:f30"
PRETTY_NAME="Fedora 30 (Workstation Edition)"
VARIANT="Workstation Edition"
VARIANT_ID=workstation`,
			map[string]string{
				"NAME":             "Fedora",
				"VERSION":          "30 (Workstation Edition)",
				"ID":               "fedora",
				"VERSION_ID":       "30",
				"VERSION_CODENAME": "",
				"PLATFORM_ID":      "platform:f30",
				"PRETTY_NAME":      "Fedora 30 (Workstation Edition)",
				"VARIANT":          "Workstation Edition",
				"VARIANT_ID":       "workstation",
			},
		},
	}

	for i, c := range cases {
		r := strings.NewReader(c.Input)

		osrelease, err := readOSRelease(r)
		if err != nil {
			t.Fatalf("%d: readOSRelease: %v", i, err)
		}

		if !reflect.DeepEqual(osrelease, c.OSRelease) {
			t.Fatalf("%d: readOSRelease returned unexpected result: %#v", i, osrelease)
		}
	}
}

func TestReadOSReleaseFromTree(t *testing.T) {
	tree := t.TempDir()

	// initialize dirs
	require.NoError(t, os.MkdirAll(path.Join(tree, "usr/lib"), 0755))
	require.NoError(t, os.MkdirAll(path.Join(tree, "etc"), 0755))

	// firstly, let's write a simple /usr/lib/os-release
	require.NoError(t,
		os.WriteFile(path.Join(tree, "usr/lib/os-release"), []byte("ID=toucan\n"), 0600),
	)

	osRelease, err := ReadOSReleaseFromTree(tree)
	require.NoError(t, err)
	require.Equal(t, "toucan", osRelease["ID"])

	// secondly, let's override it with /etc/os-release
	require.NoError(t,
		os.WriteFile(path.Join(tree, "etc/os-release"), []byte("ID=kingfisher\n"), 0600),
	)

	osRelease, err = ReadOSReleaseFromTree(tree)
	require.NoError(t, err)
	require.Equal(t, "kingfisher", osRelease["ID"])
}

func TestReadOSReleaseFromTreeUnhappy(t *testing.T) {
	tree := t.TempDir()

	_, err := ReadOSReleaseFromTree(tree)
	require.ErrorContains(t, err, "failed to read os-release")
}
