FROM registry.fedoraproject.org/fedora:43 AS builder
RUN dnf install -y git-core golang gpgme-devel libassuan-devel && mkdir -p /build/
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# disable cgo as
# a) gcc crashes on fedora41/arm64 regularly
# b) we don't really need it
# Note that this cannot move into the makefile as for bib we want to build *with* cgo to have
# the chmod/chown logic work as those need cgo/libnss (in the general case)
RUN CGO_ENABLED=0 make build

FROM registry.fedoraproject.org/fedora:43

# podman mount needs this
RUN mkdir -p /etc/containers/networks
# Fast-track osbuild so we don't depend on the "slow" Fedora release process to implement new features in bib
RUN dnf install -y dnf-plugins-core \
    && dnf copr enable -y @osbuild/osbuild \
    && dnf install -y libxcrypt-compat wget osbuild osbuild-ostree osbuild-depsolve-dnf osbuild-lvm2 openssl subscription-manager \
    && dnf clean all

COPY --from=builder /build/bin/image-builder /usr/bin/

ENTRYPOINT ["/usr/bin/image-builder"]
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
