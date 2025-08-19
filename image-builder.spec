# The minimum required osbuild version, note that this used to be 129
# but got bumped to 138 for librepo support which is not strictly
# required. So if this needs backport to places where there is no
# recent osbuild available we could simply make --use-librepo false
# and go back to 129.
%global min_osbuild_version 157

%global goipath         github.com/osbuild/image-builder-cli

Version:        31

%gometa

%global common_description %{expand:
A local binary for building customized OS artifacts such as VM images and
OSTree commits. Uses osbuild under the hood.
}

Name:           image-builder
Release:        1%{?dist}
Summary:        An image building executable using osbuild
ExcludeArch:    i686

# Upstream license specification: Apache-2.0
# Others generated with:
#   $ go_vendor_license -C <UNPACKED ARCHIVE> report expression
License:        Apache-2.0 AND BSD-2-Clause AND BSD-3-Clause AND CC-BY-SA-4.0 AND ISC AND MIT AND MPL-2.0 AND Unlicense

URL:            %{gourl}
Source0:        https://github.com/osbuild/image-builder-cli/releases/download/v%{version}/image-builder-cli-%{version}.tar.gz


BuildRequires:  %{?go_compiler:compiler(go-compiler)}%{!?go_compiler:golang}
# Build requirements of 'theproglottis/gpgme' package
BuildRequires:  gpgme-devel
BuildRequires:  libassuan-devel
# Build requirements of 'github.com/containers/storage' package
BuildRequires:  device-mapper-devel
BuildRequires:  libxcrypt-devel
%if 0%{?fedora}
# Build requirements of 'github.com/containers/storage' package
BuildRequires:  btrfs-progs-devel
# DO NOT REMOVE the BUNDLE_START and BUNDLE_END markers as they are used by 'tools/rpm_spec_add_provides_bundle.sh' to generate the Provides: bundled list
# BUNDLE_START
# BUNDLE_END
%endif

Requires:   osbuild >= %{min_osbuild_version}
Requires:   osbuild-ostree >= %{min_osbuild_version}
Requires:   osbuild-lvm2 >= %{min_osbuild_version}
Requires:   osbuild-luks2 >= %{min_osbuild_version}
Requires:   osbuild-depsolve-dnf >= %{min_osbuild_version}

%description
%{common_description}

%prep
%if 0%{?rhel}
%forgeautosetup -p1
%else
%goprep -k
%endif

%build
export GOFLAGS="-buildmode=pie"
%if 0%{?rhel}
GO_BUILD_PATH=$PWD/_build
install -m 0755 -vd $(dirname $GO_BUILD_PATH/src/%{goipath})
ln -fs $PWD $GO_BUILD_PATH/src/%{goipath}
cd $GO_BUILD_PATH/src/%{goipath}
install -m 0755 -vd _bin
export PATH=$PWD/_bin${PATH:+:$PATH}
export GOPATH=$GO_BUILD_PATH:%{gopath}
export GOFLAGS+=" -mod=vendor"
%endif

%if 0%{?fedora}
# Fedora disables Go modules by default, but we want to use them.
# Undefine the macro which disables it to use the default behavior.
%undefine gomodulesmode
%endif

# btrfs-progs-devel is not available on RHEL
%if 0%{?rhel}
GOTAGS="exclude_graphdriver_btrfs"
%endif

export LDFLAGS="${LDFLAGS} -X 'main.version=%{version}'"
%gobuild ${GOTAGS:+-tags=$GOTAGS} -o %{gobuilddir}/bin/image-builder %{goipath}/cmd/image-builder

%install
install -m 0755 -vd                                 %{buildroot}%{_bindir}
install -m 0755 -vp %{gobuilddir}/bin/image-builder %{buildroot}%{_bindir}/

%check
export GOFLAGS="-buildmode=pie"
%if 0%{?rhel}
export GOFLAGS+=" -mod=vendor -tags=exclude_graphdriver_btrfs"
export GOPATH=$PWD/_build:%{gopath}
# cd inside GOPATH, otherwise go with GO111MODULE=off ignores vendor directory
cd $PWD/_build/src/%{goipath}
%gotest ./...
%else
%gocheck
%endif

%files
%license LICENSE
%doc README.md
%{_bindir}/image-builder

%changelog
# the changelog is distribution-specific, therefore there's just one entry
# to make rpmlint happy.

* Fri Jan 24 2025 Image Builder team <osbuilders@redhat.com> - 0-1
- On this day, this project was born and the RPM created.
