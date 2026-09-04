package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRPMMacrosStage(t *testing.T) {
	expectedStage := &Stage{
		Type: "org.osbuild.rpm.macros",
		Options: &RPMMacrosStageOptions{
			Filename: "/etc/rpm/macros.image-builder",
			Macros: RPMMacros{
				InstallLangs: []string{"C.UTF-8"},
			},
		},
	}
	actualStage := NewRPMMacrosStage(&RPMMacrosStageOptions{
		Filename: "/etc/rpm/macros.image-builder",
		Macros: RPMMacros{
			InstallLangs: []string{"C.UTF-8"},
		},
	})
	assert.Equal(t, expectedStage, actualStage)
}
