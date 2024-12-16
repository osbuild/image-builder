# Do not build with tests by default
# Pass --with tests to rpmbuild to override
%bcond_with tests
%bcond_with relax_requires

# The minimum required osbuild version
%global min_osbuild_version 129

%global goipath         github.com/osbuild/image-builder-cli

Version:        0

%gometa

%global common_description %{expand:
A service for building customized OS artifacts, such as VM images and OSTree
commits, that uses osbuild under the hood. Besides building images for local
usage, it can also upload images directly to cloud.

It is compatible with composer-cli and cockpit-composer clients.
}

Name:           image-builder-cli
Release:        1%{?dist}
Summary:        An image building service based on osbuild
ExcludeArch:    i686 armv7hl

# Upstream license specification: Apache-2.0
License:        Apache-2.0
URL:            %{gourl}
Source0:        %{gosource}


BuildRequires:  %{?go_compiler:compiler(go-compiler)}%{!?go_compiler:golang}
BuildRequires:  systemd
BuildRequires:  krb5-devel
BuildRequires:  python3-docutils
BuildRequires:  make
# Build requirements of 'theproglottis/gpgme' package
BuildRequires:  gpgme-devel
BuildRequires:  libassuan-devel
# Build requirements of 'github.com/containers/storage' package
BuildRequires:  device-mapper-devel
%if 0%{?fedora}
BuildRequires:  systemd-rpm-macros
BuildRequires:  git
# Build requirements of 'github.com/containers/storage' package
BuildRequires:  btrfs-progs-devel
# DO NOT REMOVE the BUNDLE_START and BUNDLE_END markers as they are used by 'tools/rpm_spec_add_provides_bundle.sh' to generate the Provides: bundled list
# BUNDLE_START
# BUNDLE_END
%endif

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
%if 0%{?fedora}
# Fedora disables Go modules by default, but we want to use them.
# Undefine the macro which disables it to use the default behavior.
%undefine gomodulesmode
%endif

# btrfs-progs-devel is not available on RHEL
%if 0%{?rhel}
GOTAGS="exclude_graphdriver_btrfs"
%endif

%gobuild ${GOTAGS:+-tags=$GOTAGS} -o %{gobuilddir}/bin/image-builder %{goipath}/cmd/image-builder

%install
install -m 0755 -vd                                 %{buildroot}%{_bindir}
install -m 0755 -vp %{gobuilddir}/bin/image-builder %{buildroot}%{_bindir}/

%check
export GOFLAGS="-buildmode=pie"
%gocheck

%files
%license LICENSE
%doc README.md
%{_bindir}/image-builder

%changelog
# the changelog is distribution-specific, therefore there's just one entry
# to make rpmlint happy.

* Wed Sep 11 2019 Image Builder team <osbuilders@redhat.com> - 0-1
- On this day, this project was born.
