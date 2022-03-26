FROM fedora:35

# Install:
#  - a few packages for convenient usage
#  - RPM tooling
#  - the go compiler
RUN dnf install -y \
    fish \
    fd-find \
    ripgrep \
    jq \
    go
# install the language server
RUN go install -v golang.org/x/tools/gopls@latest
RUN go install -v github.com/uudashr/gopkgs/v2/cmd/gopkgs@latest
RUN go install -v github.com/ramya-rao-a/go-outline@latest
RUN go install -v github.com/go-delve/delve/cmd/dlv@latest
RUN go install -v honnef.co/go/tools/cmd/staticcheck@latest