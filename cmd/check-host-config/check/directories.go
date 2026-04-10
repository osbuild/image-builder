package check

import (
	"strconv"
	"syscall"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "directories",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, directoriesCheck)
}

func directoriesCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	expected := config.Blueprint.Customizations.Directories

	if len(expected) == 0 {
		return Skip("no directories to check")
	}

	for _, dir := range expected {
		if !ExistsDir(dir.Path) {
			return Fail("directory does not exist:", dir.Path)
		}

		info, err := Stat(dir.Path)
		if err != nil {
			return Fail("failed to get directory info:", dir.Path)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return Fail("check only works on UNIX-like")
		}
		mode, uid, gid := info.Mode(), stat.Uid, stat.Gid
		if !info.IsDir() {
			return Fail("path is not a directory:", dir.Path)
		}

		if dir.Mode != "" {
			userMode, err := strconv.ParseUint(dir.Mode, 8, 32)
			if err != nil {
				return Fail("failed to parse directory mode:", dir.Path)
			}

			if int64(mode.Perm()) != int64(userMode) {
				return Fail("directory mode does not match:", dir.Path)
			}
		}

		if dir.User != nil {
			expectedUid, err := resolveUser(dir.User)
			if err != nil {
				return Fail("failed to resolve user:", dir.Path, err)
			}
			if uid != expectedUid {
				return Fail("directory user does not match:", dir.Path)
			}
		}

		if dir.Group != nil {
			expectedGid, err := resolveGroup(dir.Group)
			if err != nil {
				return Fail("failed to resolve group:", dir.Path, err)
			}
			if gid != expectedGid {
				return Fail("directory group does not match:", dir.Path)
			}
		}
	}

	return Pass()
}
