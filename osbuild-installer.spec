%global goipath github.com/osbuild/osbuild-installer

Version:	1

%gometa

%global common_description %{expand:
A web service which relays requests to osbuild-composer.
}

Name:		osbuild-installer
Release:	1%{?dist}
Summary:	A web service which relays requests to osbuild-composer

# Upstream license specification: Apache-2.0
License:	ASL 2.0
URL:		%{gourl}
Source0:        %{gosource}

BuildRequires:	%{?go_compiler:compiler(go-compiler)}%{!?go_compiler:golang}
BuildRequires:	systemd

Requires: systemd

%description
%{common_description}

%prep
%forgeautosetup -p1

%build
export GOFLAGS=-mod=vendor

%gobuild -o _bin/osbuild-installer                   %{goipath}/cmd/osbuild-installer

%install
install -m 0755 -vd                                  %{buildroot}%{_libexecdir}/osbuild-installer
install -m 0755 -vp _bin/osbuild-installer           %{buildroot}%{_libexecdir}/osbuild-installer/

install -m 0755 -vd				     %{buildroot}%{_unitdir}
install -m 0644 -vp distribution/*.{service,socket}  %{buildroot}%{_unitdir}/

install -m 0755 -vd                                  %{buildroot}%{_datadir}/osbuild-installer/repositories
install -m 0644 -vp repositories/*                   %{buildroot}%{_datadir}/osbuild-installer/repositories/

%files
%{_libexecdir}/osbuild-installer/osbuild-installer
%{_unitdir}/osbuild-installer.service
%{_unitdir}/osbuild-installer.socket

%post
%systemd_post osbuild-installer.service osbuild-remote-installer.service

%preun
%systemd_preun osbuild-installer.service osbuild-remote-installer.service

%postun
%systemd_postun_with_restart osbuild-installer.service osbuild-remote-installer.service

%changelog
# None
