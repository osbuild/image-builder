package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dnf module list --enabled output fixtures for different distro versions
var (
	dnfModuleListOutputRHEL7 = `Last metadata expiration check: 1:23:45 ago on Mon 01 Jan 2024 12:00:00 PM UTC.
Dependencies resolved.
Module Stream Profiles
nodejs           10        [e]       common [d], development, minimal
python36         3.6       [e]       build, common [d], devel
Hint: [d]efault, [e]nabled, [x]disabled, [i]nstalled, [a]ctive
`

	dnfModuleListOutputRHEL8 = `Last metadata expiration check: 0:00:00 ago on Mon 01 Jan 2024 12:00:00 PM UTC.
Dependencies resolved.
Module Stream Profiles
nodejs           12        [e]       common [d], development, minimal, s2i
python38         3.8       [e]       build, common [d], devel, minimal
postgresql       12        [e]       client, server [d]
Hint: [d]efault, [e]nabled, [x]disabled, [i]nstalled, [a]ctive
`

	dnfModuleListOutputRHEL9 = `Last metadata expiration check: 0:00:00 ago
Dependencies resolved.
Module Stream Profiles
nodejs           18        [d]       common [d], development, minimal, s2i
python39         3.9       [d]       build, common [d], devel, minimal
Hint: [d]efault, [e]nabled, [x]disabled, [i]nstalled, [a]ctive
`

	dnfModuleListOutputRHEL10 = `Last metadata expiration check: 0:00:00 ago
Dependencies resolved.
Module Stream Profiles
nodejs           20        [e]       common [d], development, minimal, s2i
python312        3.12      [e]       build, common [d], devel, minimal
postgresql       16        [e]       client, server [d], devel
Use "dnf module info <module:stream>" to get more information.
Hint: [d]efault, [e]nabled, [x]disabled, [i]nstalled, [a]ctive
`

	dnfModuleListOutputCentOS9 = `CentOS Stream 9 - AppStream
Name      Stream    Profiles                                Summary             
nodejs    18 [e]    common [d], development, minimal, s2i   Javascript runtime  

Hint: [d]efault, [e]nabled, [x]disabled, [i]nstalled
`

	dnfModuleListOutputMultiple = `Last metadata expiration check: 0:00:00 ago
Dependencies resolved.
Module Stream Profiles
nodejs           18        [e]       common [d], development, minimal, s2i
python39         3.9       [e]       build, common [d], devel, minimal
postgresql       13        [e]       client, server [d]
ruby              3.1       [e]       common [d], devel
Hint: [d]efault, [e]nabled, [x]disabled, [i]nstalled, [a]ctive
`
)

const dnfModuleListCmd = "dnf -y -q module list --enabled"

func TestModularityCheck(t *testing.T) {
	tests := []struct {
		name     string
		config   []blueprint.EnabledModule
		mockExec map[string]ExecResult
		wantErr  error
	}{
		{
			name:    "skip when no modules",
			config:  []blueprint.EnabledModule{},
			wantErr: check.ErrCheckSkipped,
		},
		{
			name: "pass with single module (RHEL 9 style)",
			config: []blueprint.EnabledModule{
				{Name: "nodejs", Stream: "18"},
			},
			mockExec: map[string]ExecResult{
				dnfModuleListCmd: {Stdout: []byte(dnfModuleListOutputRHEL9)},
			},
		},
		{
			name: "pass RHEL 7 format",
			config: []blueprint.EnabledModule{
				{Name: "nodejs", Stream: "10"},
				{Name: "python36", Stream: "3.6"},
			},
			mockExec: map[string]ExecResult{
				dnfModuleListCmd: {Stdout: []byte(dnfModuleListOutputRHEL7)},
			},
		},
		{
			name: "pass RHEL 8 format",
			config: []blueprint.EnabledModule{
				{Name: "nodejs", Stream: "12"},
				{Name: "python38", Stream: "3.8"},
				{Name: "postgresql", Stream: "12"},
			},
			mockExec: map[string]ExecResult{
				dnfModuleListCmd: {Stdout: []byte(dnfModuleListOutputRHEL8)},
			},
		},
		{
			name: "pass RHEL 9 format",
			config: []blueprint.EnabledModule{
				{Name: "nodejs", Stream: "18"},
				{Name: "python39", Stream: "3.9"},
			},
			mockExec: map[string]ExecResult{
				dnfModuleListCmd: {Stdout: []byte(dnfModuleListOutputRHEL9)},
			},
		},
		{
			name: "pass RHEL 10 format",
			config: []blueprint.EnabledModule{
				{Name: "nodejs", Stream: "20"},
				{Name: "python312", Stream: "3.12"},
				{Name: "postgresql", Stream: "16"},
			},
			mockExec: map[string]ExecResult{
				dnfModuleListCmd: {Stdout: []byte(dnfModuleListOutputRHEL10)},
			},
		},
		{
			name: "pass CentOS 9 format",
			config: []blueprint.EnabledModule{
				{Name: "nodejs", Stream: "18"},
			},
			mockExec: map[string]ExecResult{
				dnfModuleListCmd: {Stdout: []byte(dnfModuleListOutputCentOS9)},
			},
		},
		{
			name: "pass multiple modules",
			config: []blueprint.EnabledModule{
				{Name: "nodejs", Stream: "18"},
				{Name: "python39", Stream: "3.9"},
				{Name: "postgresql", Stream: "13"},
				{Name: "ruby", Stream: "3.1"},
			},
			mockExec: map[string]ExecResult{
				dnfModuleListCmd: {Stdout: []byte(dnfModuleListOutputMultiple)},
			},
		},
		{
			name: "fail when dnf errors",
			config: []blueprint.EnabledModule{
				{Name: "nodejs", Stream: "18"},
			},
			mockExec: map[string]ExecResult{
				dnfModuleListCmd: {Code: 1, Err: errors.New("dnf failed")},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)

			chk, found := check.FindCheckByName("modularity")
			require.True(t, found, "modularity check not found")
			config := buildConfigWithBlueprint(func(bp *blueprint.Blueprint) {
				bp.EnabledModules = tt.config
				if len(tt.config) == 0 {
					bp.Packages = []blueprint.Package{}
				}
			})

			err := chk.Func(chk.Meta, config)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
