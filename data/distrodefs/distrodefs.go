//go:build !rpm_data

package distrodefs

import "embed"

//go:embed *.yaml */*.yaml
var Data embed.FS

// Source describes where the default definitions are loaded from.
func Source() string { return "embedded" }
