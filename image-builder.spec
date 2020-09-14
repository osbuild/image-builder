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
Source0:        %{gosource}

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

%gobuild -o _bin/image-builder                        %{goipath}/cmd/image-builder

%install
install -m 0755 -vd                                   %{buildroot}%{_libexecdir}/image-builder
install -m 0755 -vp _bin/image-builder                %{buildroot}%{_libexecdir}/image-builder/

install -m 0755 -vd				      %{buildroot}%{_unitdir}
install -m 0644 -vp distribution/*.{service,socket}   %{buildroot}%{_unitdir}/

install -m 0755 -vd                                   %{buildroot}%{_datadir}/image-builder/distributions
install -m 0644 -vp distributions/*                   %{buildroot}%{_datadir}/image-builder/distributions/

%files
%{_libexecdir}/image-builder/image-builder
%{_unitdir}/image-builder.service
%{_unitdir}/image-builder.socket
%{_datadir}/image-builder/distributions/

%post
%systemd_post image-builder.service

%preun
%systemd_preun image-builder.service

%postun
%systemd_postun_with_restart image-builder.service

%changelog
# None
