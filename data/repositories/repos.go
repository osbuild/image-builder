//go:build !rpm_data

package repos

import (
	"embed"
)

//go:embed *.json
var FS embed.FS
