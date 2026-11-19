//go:build rpm_data

package distrodefs

import "os"

// DataDir can be set at build time to match the installation prefix.
var DataDir = "/usr/share/image-builder/definitions"

var Data = os.DirFS(DataDir)

// Source describes where the default definitions are loaded from.
func Source() string { return DataDir }
