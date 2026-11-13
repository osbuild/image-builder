package sbom

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRPMPURL(t *testing.T) {
	tests := []struct {
		name string
		pkg  Package
		want string
	}{
		{
			name: "red hat vendor maps to redhat",
			pkg: Package{
				Name:    "bash",
				Version: "5.1.8",
				Release: "9.el9",
				Arch:    "x86_64",
				Vendor:  "Red Hat, Inc.",
			},
			want: "pkg:rpm/redhat/bash@5.1.8-9.el9?arch=x86_64",
		},
		{
			name: "fedora vendor maps to fedora",
			pkg: Package{
				Name:    "bash",
				Version: "5.2.26",
				Release: "3.fc40",
				Arch:    "x86_64",
				Vendor:  "Fedora Project",
			},
			want: "pkg:rpm/fedora/bash@5.2.26-3.fc40?arch=x86_64",
		},
		{
			name: "centos vendor maps to centos",
			pkg: Package{
				Name:    "bash",
				Version: "5.1.8",
				Release: "9.el9",
				Arch:    "x86_64",
				Vendor:  "CentOS",
			},
			want: "pkg:rpm/centos/bash@5.1.8-9.el9?arch=x86_64",
		},
		{
			name: "almalinux vendor maps to almalinux",
			pkg: Package{
				Name:    "bash",
				Version: "5.1.8",
				Release: "9.el9",
				Arch:    "x86_64",
				Vendor:  "AlmaLinux",
			},
			want: "pkg:rpm/almalinux/bash@5.1.8-9.el9?arch=x86_64",
		},
		{
			name: "rocky vendor maps to rockylinux",
			pkg: Package{
				Name:    "bash",
				Version: "5.1.8",
				Release: "9.el9",
				Arch:    "x86_64",
				Vendor:  "Rocky Enterprise Software Foundation",
			},
			want: "pkg:rpm/rockylinux/bash@5.1.8-9.el9?arch=x86_64",
		},
		{
			name: "oracle vendor maps to oraclelinux",
			pkg: Package{
				Name:    "bash",
				Version: "5.1.8",
				Release: "9.el9",
				Arch:    "x86_64",
				Vendor:  "Oracle America",
			},
			want: "pkg:rpm/oraclelinux/bash@5.1.8-9.el9?arch=x86_64",
		},
		{
			name: "unknown vendor falls back to the lowercased vendor",
			pkg: Package{
				Name:    "ffmpeg",
				Version: "7.1.1",
				Release: "1.fc42",
				Arch:    "x86_64",
				Vendor:  "RPM Fusion",
			},
			want: "pkg:rpm/rpm%20fusion/ffmpeg@7.1.1-1.fc42?arch=x86_64",
		},
		{
			name: "no vendor means no namespace",
			pkg: Package{
				Name:    "widget",
				Version: "1.0",
				Release: "1",
				Arch:    "x86_64",
			},
			want: "pkg:rpm/widget@1.0-1?arch=x86_64",
		},
		{
			name: "plus in name is percent-encoded",
			pkg: Package{
				Name:    "libstdc++",
				Version: "11.5.0",
				Release: "2.el9",
				Arch:    "x86_64",
				Vendor:  "Red Hat, Inc.",
			},
			want: "pkg:rpm/redhat/libstdc%2B%2B@11.5.0-2.el9?arch=x86_64",
		},
		{
			name: "non-zero epoch becomes a qualifier",
			pkg: Package{
				Name:    "emacs",
				Epoch:   1,
				Version: "27.2",
				Release: "9.el9",
				Arch:    "x86_64",
				Vendor:  "Red Hat, Inc.",
			},
			want: "pkg:rpm/redhat/emacs@27.2-9.el9?arch=x86_64&epoch=1",
		},
		{
			name: "noarch",
			pkg: Package{
				Name:    "tzdata",
				Version: "2024a",
				Release: "1.el9",
				Arch:    "noarch",
				Vendor:  "Red Hat, Inc.",
			},
			want: "pkg:rpm/redhat/tzdata@2024a-1.el9?arch=noarch",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rpmPURL(tt.pkg))
		})
	}
}
