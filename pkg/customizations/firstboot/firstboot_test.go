package firstboot_test

import (
	"encoding/json"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/pkg/customizations/firstboot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstbootOptionsFromBP(t *testing.T) {
	tests := []struct {
		name string
		json string
		want firstboot.FirstbootOptions
		err  string
	}{
		{
			name: "custom-named",
			json: `{"scripts": [{"type":"custom","name":"hello","contents":"echo hello"}]}`,
			want: firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{{
					Filename: "osbuild-first-boot-hello",
					Contents: "echo hello",
				}},
			},
		},
		{
			name: "custom-unnamed",
			json: `{"scripts": [{"type":"custom","contents":"echo one"}]}`,
			want: firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{{
					Filename: "osbuild-first-boot-custom-1",
					Contents: "echo one",
				}},
			},
		},
		{
			name: "custom-two-unnamed",
			json: `{"scripts": [
				{"type":"custom","contents":"echo one"},
				{"type":"custom","contents":"echo two"}
			]}`,
			want: firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{
					{Filename: "osbuild-first-boot-custom-1", Contents: "echo one"},
					{Filename: "osbuild-first-boot-custom-2", Contents: "echo two"},
				},
			},
		},
		{
			name: "satellite",
			json: `{"scripts": [{"type":"satellite","command":"echo hello"}]}`,
			want: firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{{
					Filename: "osbuild-first-boot-satellite",
					Contents: "echo hello",
				}},
			},
		},
		{
			name: "aap",
			json: `{"scripts": [{"type":"aap","host_config_key":"key","job_template_url":"https://aap.example.com/api/v2/job_templates/9/callback/"}]}`,
			want: firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{{
					Filename: "osbuild-first-boot-aap",
					Contents: "#!/usr/bin/bash\ncurl -i --data 'host_config_key=key' 'https://aap.example.com/api/v2/job_templates/9/callback/'\n",
				}},
			},
		},
		{
			name: "mixed",
			json: `{"scripts": [
				{"type":"satellite","command":"echo sat"},
				{"type":"custom","name":"setup","contents":"echo setup"},
				{"type":"custom","contents":"echo unnamed"},
				{"type":"aap","host_config_key":"key","job_template_url":"https://aap.example.com/api/v2/job_templates/9/callback/"}
			]}`,
			want: firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{
					{Filename: "osbuild-first-boot-satellite", Contents: "echo sat"},
					{Filename: "osbuild-first-boot-setup", Contents: "echo setup"},
					{Filename: "osbuild-first-boot-custom-1", Contents: "echo unnamed"},
					{Filename: "osbuild-first-boot-aap", Contents: "#!/usr/bin/bash\ncurl -i --data 'host_config_key=key' 'https://aap.example.com/api/v2/job_templates/9/callback/'\n"},
				},
			},
		},
		{
			name: "after-before",
			json: `{"scripts": [
				{"type":"custom","name":"first","after":["sshd.service"],"before":["postgresql.service"],"contents":"echo first"},
				{"type":"custom","name":"second","contents":"echo second"}
			]}`,
			want: firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{
					{
						Filename: "osbuild-first-boot-first",
						Contents: "echo first",
						After:    []string{"sshd.service"},
						Before:   []string{"postgresql.service"},
					},
					{Filename: "osbuild-first-boot-second", Contents: "echo second"},
				},
			},
		},
		{
			name: "duplicate-satellite",
			json: `{"scripts": [
				{"type":"satellite","command":"one"},
				{"type":"satellite","command":"two"}
			]}`,
			err: "firstboot customization already set: satellite",
		},
		{
			name: "duplicate-aap",
			json: `{"scripts": [
				{"type":"aap","host_config_key":"k","job_template_url":"https://example.com/callback/"},
				{"type":"aap","host_config_key":"k","job_template_url":"https://example.com/callback/"}
			]}`,
			err: "firstboot customization already set: aap",
		},
		{
			name: "duplicate-custom-name",
			json: `{"scripts": [
				{"type":"custom","name":"setup","contents":"one"},
				{"type":"custom","name":"setup","contents":"two"}
			]}`,
			err: "firstboot name already taken: setup",
		},
		{
			name: "invalid-path-parent",
			json: `{"scripts": [{"type":"custom","name":"../bad","contents":"bad"}]}`,
			err:  `firstboot name "../bad" is invalid`,
		},
		{
			name: "invalid-path-traversal",
			json: `{"scripts": [{"type":"custom","name":"../../etc/passwd","contents":"bad"}]}`,
			err:  `firstboot name "../../etc/passwd" is invalid`,
		},
		{
			name: "invalid-path-with-slash",
			json: `{"scripts": [{"type":"custom","name":"invalid/filename","contents":"bad"}]}`,
			err:  `firstboot name "invalid/filename" is invalid`,
		},
		{
			name: "invalid-absolute-path",
			json: `{"scripts": [{"type":"custom","name":"/etc/shadow","contents":"bad"}]}`,
			err:  `firstboot name "/etc/shadow" is invalid`,
		},
		{
			name: "reserved-satellite",
			json: `{"scripts": [{"type":"custom","name":"satellite","contents":"bad"}]}`,
			err:  `firstboot name "satellite" is reserved`,
		},
		{
			name: "reserved-aap",
			json: `{"scripts": [{"type":"custom","name":"aap","contents":"bad"}]}`,
			err:  `firstboot name "aap" is reserved`,
		},
		{
			name: "reserved-custom-prefix",
			json: `{"scripts": [{"type":"custom","name":"custom-42","contents":"bad"}]}`,
			err:  `firstboot name "custom-42" is reserved`,
		},
		{
			name: "reserved-custom-exact",
			json: `{"scripts": [{"type":"custom","name":"custom","contents":"bad"}]}`,
			err:  `firstboot name "custom" is reserved`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input blueprint.FirstbootCustomization
			require.NoError(t, json.Unmarshal([]byte(tt.json), &input))

			got, err := firstboot.FirstbootOptionsFromBP(input)
			if tt.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want, *got)
		})
	}
}
