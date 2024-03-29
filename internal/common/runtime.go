package common

import "runtime/debug"

var (
	// Git SHA commit (first 4 characters)
	BuildCommit string

	// Build date and time
	BuildTime string

	// BuildGoVersion carries Go version the binary was built with
	BuildGoVersion string
)

func init() {
	bi, ok := debug.ReadBuildInfo()

	if !ok {
		BuildTime = "N/A"
		BuildCommit = "HEAD"
	}

	BuildGoVersion = bi.GoVersion

	for _, bs := range bi.Settings {
		switch bs.Key {
		case "vcs.revision":
			if len(bs.Value) > 6 {
				BuildCommit = bs.Value[0:6]
			}
		case "vcs.time":
			BuildTime = bs.Value
		}
	}
}
