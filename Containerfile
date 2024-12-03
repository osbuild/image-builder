FROM registry.fedoraproject.org/fedora:41 AS builder
RUN dnf install -y git-core golang gpgme-devel libassuan-devel && mkdir -p /build/
ARG GOPROXY=https://proxy.golang.org,direct
RUN go env -w GOPROXY=$GOPROXY
COPY . /build
WORKDIR /build
# keep in sync with:
# https://github.com/containers/podman/blob/2981262215f563461d449b9841741339f4d9a894/Makefile#L51
RUN go build -tags "containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper" ./cmd/image-builder

FROM registry.fedoraproject.org/fedora:41

# Fast-track osbuild so we don't depend on the "slow" Fedora release process to implement new features in bib
RUN dnf install -y dnf-plugins-core \
    && dnf copr enable -y @osbuild/osbuild \
    && dnf install -y libxcrypt-compat wget osbuild osbuild-ostree osbuild-depsolve-dnf osbuild-lvm2 \
    && dnf clean all

COPY --from=builder /build/image-builder /usr/bin/

# install repo data from osbuild-composer in an ugly way
# XXX: find a better way
RUN <<EOR
 mkdir -p /usr/share/osbuild-composer/repositories
 cd /usr/share/osbuild-composer/repositories
 # XXX: find a better way to organize the upstream supported repos
 # we cannot just checkout the osbuild-composer repo here as it contains a bunch
 # of "*-no-aux-key.json" files too, so a naive "cp *.json" will not work.
 # Ideally we split the supported repos out of osbuild-composer into
 # either "images" (as it generates the code for supported distro it could
 # be the place that also defines what is supported) or into its own repo.
 # Bonus points if we could make them go:embedable :)
 for fname in centos-stream-9 centos-stream-10 \
              fedora-40 fedora-41 \
              rhel-8 rhel-8.1 rhel-8.2 rhel-8.3 rhel-8.4 rhel-8.5 rhel-8.6 \
	      rhel-8.7 rhel-8.8 rhel-8.9 \
	      rhel-9.0 rhel-9.1 rhel-9.2 rhel-9.3 rhel-9.4 rhel-9.5 rhel-9.6 \
	      rhel-10.0; do
   # XXX: if only we had 'go:embed'able repos :(
   wget https://raw.githubusercontent.com/osbuild/osbuild-composer/refs/heads/main/repositories/${fname}.json
 done
 # XXX: find an even better way here, those are symlinks in the upstream repo
 cp -a centos-stream-9.json centos-9.json
 cp -a centos-stream-10.json centos-10.json
EOR

COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
VOLUME /output
WORKDIR /output
VOLUME /store
VOLUME /rpmmd
VOLUME /var/lib/containers/storage

LABEL description="This tools allows to build and deploy disk-images."
LABEL io.k8s.description="This tools allows to build and deploy disk-images."
LABEL io.k8s.display-name="Image Builder"
LABEL io.openshift.tags="base fedora40"
LABEL summary="A container to create disk-images."
