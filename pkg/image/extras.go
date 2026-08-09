package image

import "fmt"

// SysextPipelineName returns the osbuild pipeline name for a sysext with the
// given name and format.
func SysextPipelineName(name, format string) string {
	return fmt.Sprintf("sysext-%s-%s", name, format)
}
