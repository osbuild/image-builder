# Do not build with tests by default
# Pass --with tests to rpmbuild to override
%bcond_with tests

%global goipath github.com/osbuild/image-builder

Version:	1

%gometa

%global common_description %{expand:
A web service which relays requests to osbuild-composer.
}

Name:		image-builder
Release:	1%{?dist}
Summary:	A web service which relays requests to osbuild-composer

# Upstream license specification: Apache-2.0
License:	ASL 2.0
URL:		%{gourl}
Source0:	%{gosource}

BuildRequires:	%{?go_compiler:compiler(go-compiler)}%{!?go_compiler:golang}
BuildRequires:	systemd

Requires: systemd

%description
%{common_description}

%prep
%forgeautosetup -p1

%build
GO_BUILD_PATH=$PWD/_build
install -m 0755 -vd $(dirname $GO_BUILD_PATH/src/%{goipath})
ln -fs $PWD $GO_BUILD_PATH/src/%{goipath}
cd $GO_BUILD_PATH/src/%{goipath}
install -m 0755 -vd _bin
export PATH=$PWD/_bin${PATH:+:$PATH}
export GOPATH=$GO_BUILD_PATH:%{gopath}
export GOFLAGS=-mod=vendor

%gobuild -o _bin/image-builder %{goipath}/cmd/image-builder
%gobuild -o _bin/image-builder-migrate-db %{goipath}/cmd/image-builder-migrate-db

%install
install -m 0755 -vd					%{buildroot}%{_libexecdir}/image-builder
install -m 0755 -vp _bin/image-builder			%{buildroot}%{_libexecdir}/image-builder/
install -m 0755 -vp _bin/image-builder-migrate-db       %{buildroot}%{_libexecdir}/image-builder/

install -m 0755 -vd					%{buildroot}%{_unitdir}
install -m 0644 -vp distribution/*.{service,socket}	%{buildroot}%{_unitdir}/

install -m 0755 -vd					%{buildroot}%{_datadir}/image-builder/distributions
install -m 0644 -vp distributions/*			%{buildroot}%{_datadir}/image-builder/distributions/

install -m 0755 -vd                                   %{buildroot}%{_datadir}/image-builder/migrations
install -m 0644 -vp internal/db/migrations/*          %{buildroot}%{_datadir}/image-builder/migrations/

%if %{with tests} || 0%{?rhel}
install -m 0755 -vd					%{buildroot}%{_libexecdir}/tests/image-builder
install -m 0755 -vp test/cases/*                        %{buildroot}%{_libexecdir}/tests/image-builder/

install -m 0755 -vd                                             %{buildroot}%{_datadir}/tests/image-builder/edge
install -m 0644 -vp test/data/edge/*                            %{buildroot}%{_datadir}/tests/image-builder/edge/

install -m 0755 -vd                                             %{buildroot}%{_datadir}/tests/image-builder/keyring
install -m 0644 -vp test/data/keyring/id_rsa.pub                %{buildroot}%{_datadir}/tests/image-builder/keyring/
install -m 0600 -vp test/data/keyring/id_rsa                    %{buildroot}%{_datadir}/tests/image-builder/keyring/

install -m 0755 -vd                                             %{buildroot}%{_datadir}/tests/image-builder/repositories
install -m 0644 -vp test/data/repositories/*                    %{buildroot}%{_datadir}/tests/image-builder/repositories/

%endif

%post
%systemd_post image-builder.service

%preun
%systemd_preun image-builder.service

%postun
%systemd_postun_with_restart image-builder.service

%files
%{_libexecdir}/image-builder/image-builder
%{_libexecdir}/image-builder/image-builder-migrate-db
%{_unitdir}/image-builder.service
%{_unitdir}/image-builder.socket
%{_datadir}/image-builder/distributions/
%{_datadir}/image-builder/migrations/

%if %{with tests} || 0%{?rhel}
%package tests
Summary:    Integration tests
Requires:   %{name} = %{version}-%{release}
Requires:   openssl
Requires:   jq
Requires:   curl
Requires:   httpd
Requires:   expect
Requires:   qemu-img
Requires:   qemu-kvm
Requires:   libvirt-client
Requires:   libvirt-daemon-kvm
Requires:   virt-install

%description tests
Integration tests to be run on a system with image-builder installed.

%files tests
%{_libexecdir}/tests/image-builder
%{_datadir}/tests/image-builder
%endif

%changelog
# the changelog is distribution-specific, therefore there's just one entry
# to make rpmlint happy.

* Fri Mar 27 2020 Image Builder team <osbuilders@redhat.com> - 0-1
- On this day, this project was born.
