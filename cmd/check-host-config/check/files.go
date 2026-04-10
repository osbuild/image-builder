package check

import (
	"log"
	"os"
	"strconv"
	"syscall"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "files",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, filesCheck)
}

func filesCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	expected := config.Blueprint.Customizations.Files

	if len(expected) == 0 {
		return Skip("no files to check")
	}

	for _, file := range expected {
		if !Exists(file.Path) {
			return Fail("file does not exist:", file.Path)
		}

		info, err := Stat(file.Path)
		if err != nil {
			return Fail("failed to get file info:", file.Path)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return Fail("check only works on UNIX-like")
		}
		mode, uid, gid := info.Mode(), stat.Uid, stat.Gid

		if file.Mode != "" {
			userMode, err := strconv.ParseUint(file.Mode, 8, 32)
			if err != nil {
				return Fail("failed to parse file mode:", file.Path)
			}

			if mode.Perm() != os.FileMode(userMode) {
				return Fail("file mode does not match:", file.Path)
			}
		}

		if file.User != nil {
			expectedUid, err := resolveUser(file.User)
			if err != nil {
				return Fail("failed to resolve user:", file.Path, err)
			}
			if uid != expectedUid {
				return Fail("file user does not match:", file.Path)
			}
		}

		if file.Group != nil {
			expectedGid, err := resolveGroup(file.Group)
			if err != nil {
				return Fail("failed to resolve group:", file.Path, err)
			}
			if gid != expectedGid {
				return Fail("file group does not match:", file.Path)
			}
		}

		if len(file.Data) > 0 {
			content, err := ReadFile(file.Path)
			if err != nil {
				return Fail("failed to read file:", file.Path)
			}

			if string(content) != file.Data {
				return Fail("file content does not match:", file.Path)
			}
		}

		if len(file.URI) > 0 {
			log.Printf("Not checking file content specified by URI: %s", file.URI)
		}
	}

	return Pass()
}
