package check

import (
	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:          "bootc-status",
		RequiresBootc: true,
	}, bootcStatusCheck)
}

func bootcStatusCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	_, _, _, err := ExecString("bootc", "status")
	if err != nil {
		return Fail("bootc status failed:", err)
	}
	return Pass()
}
