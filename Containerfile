FROM registry.fedoraproject.org/fedora:42 AS builder
RUN dnf install -y git-core golang gpgme-devel libassuan-devel && mkdir -p /build/
ARG GOPROXY=https://proxy.golang.org,direct
RUN go env -w GOPROXY=$GOPROXY
COPY . /build
WORKDIR /build
# keep in sync with:
# https://github.com/containers/podman/blob/2981262215f563461d449b9841741339f4d9a894/Makefile#L51
# disable cgo as
# a) gcc crashes on fedora41/arm64 regularly
# b) we don't really need it
RUN CGO_ENABLED=0 go build -tags "containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper" ./cmd/image-builder

FROM registry.fedoraproject.org/fedora:41

# podman mount needs this
RUN mkdir -p /etc/containers/networks
# Fast-track osbuild so we don't depend on the "slow" Fedora release process to implement new features in bib
RUN dnf install -y dnf-plugins-core \
    && dnf copr enable -y @osbuild/osbuild \
    && dnf install -y libxcrypt-compat wget osbuild osbuild-ostree osbuild-depsolve-dnf osbuild-lvm2 \
    && dnf clean all

COPY --from=builder /build/image-builder /usr/bin/

COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
VOLUME /output
WORKDIR /output
# XXX: add "store" flag like bib
VOLUME /var/cache/image-builder/store
VOLUME /var/lib/containers/storage

LABEL description="This tools allows to build and deploy disk-images."
LABEL io.k8s.description="This tools allows to build and deploy disk-images."
LABEL io.k8s.display-name="Image Builder"
LABEL io.openshift.tags="base fedora40"
LABEL summary="A container to create disk-images."
