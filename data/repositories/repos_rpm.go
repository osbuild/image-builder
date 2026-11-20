//go:build rpm_data

package repos

import "os"

// DataDir can be set at build time to match the installation prefix.
var DataDir = "/usr/share/image-builder/repositories"

var FS = os.DirFS(DataDir)
