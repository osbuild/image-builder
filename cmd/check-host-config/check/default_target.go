package check

import (
	"github.com/osbuild/image-builder/internal/buildconfig"
	"log"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "default-target",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, defaultTargetCheck)
}

func defaultTargetCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	services := config.Blueprint.Customizations.Services
	if services == nil || services.DefaultTarget == "" {
		return Skip("no default target specified")
	}
	expected := services.DefaultTarget

	log.Printf("Checking default target: %s\n", expected)
	actual, _, _, err := ExecString("systemctl", "get-default")
	if err != nil {
		return Fail("systemctl get-default failed:", err)
	}
	if actual != expected {
		return Fail("default target mismatch: expected", expected, "got", actual)
	}

	log.Printf("Default target is set to %s as expected\n", expected)
	return Pass()
}
