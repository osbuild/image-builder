FROM fedora:latest

# Install:
#  - a few packages for convenient usage
#  - the go compiler
#  - osbuild
RUN dnf install -y \
  fish \
  fd-find \
  ripgrep \
  jq \
  go \
  osbuild \
  osbuild-lvm2 \
  osbuild-luks2 \
  osbuild-ostree \
  osbuild-depsolve-dnf \
  osbuild-tools
# install the language server
RUN go install -v golang.org/x/tools/gopls@latest
RUN go install -v github.com/uudashr/gopkgs/v2/cmd/gopkgs@latest
RUN go install -v github.com/ramya-rao-a/go-outline@latest
RUN go install -v github.com/go-delve/delve/cmd/dlv@latest
RUN go install -v honnef.co/go/tools/cmd/staticcheck@latest
COPY ./test/scripts/install-dependencies /install-dependencies
RUN /install-dependencies
