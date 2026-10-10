# Generating a Fedora 45 Live ISO with Anaconda for bootc Image Installation

The `bootc-image-builder` project has been deprecated in favor of the unified `image-builder` tool. This document details the process of creating a Fedora Live ISO with the Anaconda installer to install `bootc` images using the new `bootc-generic-iso` method in `image-builder`.

Below is a `Containerfile` that generates a local container image used to build a Fedora 45 ISO with your custom `bootc` image. This documentation was tested specifically on Fedora 45. For other versions, the legacy `bootc-image-builder` utility still works, and this workflow may also work on Fedora 44.

The `Containerfile` was obtained from [https://osbuild.org/docs/developer-guide/projects/image-builder/advanced/bootc/isos/](https://osbuild.org/docs/developer-guide/projects/image-builder/advanced/bootc/isos/?utm_source=gemini). Reading that documentation prior to following this tutorial is strongly recommended.

## Containerfile

```
FROM quay.io/fedora/fedora-bootc:45

# Installation of installer tools and boot media
RUN dnf5 install -qy \
    anaconda \
    anaconda-install-img-deps \
    anaconda-dracut \
    dracut-config-generic \
    dracut-network \
    net-tools \
    grub2-efi-x64-cdboot \
    plymouth \
    default-fonts-core-sans \
    default-fonts-other-sans \
    google-noto-sans-cjk-fonts \
    xorrisofs \
    squashfs-tools \
    jq && \
    dnf5 clean all

RUN mkdir -p /boot/efi && cp -ra /usr/lib/efi/*/*/EFI /boot/efi

# ISO GRUB Boot Menu Configuration
RUN mkdir -p /usr/lib/image-builder/bootc
COPY <<EOT /usr/lib/image-builder/bootc/iso.yaml
label: "Custom-Fedora-Installer"
grub2:
  entries:
    - name: "Install Fedora (bootc)"
      linux: "/images/pxeboot/vmlinuz inst.stage2=hd:LABEL=Custom-Fedora-Installer console=tty0 inst.graphical selinux=0 rhgb quiet"
      initrd: "/images/pxeboot/initrd.img"
EOT

# Enables bootc installation mode in ISO's Anaconda (this is where kickstart gets fetched)
COPY <<EOT /usr/share/anaconda/interactive-defaults.ks
bootc --source-imgref registry:IMAGE-NAME --target-imgref IMAGE-NAME
EOT

# Anaconda environment adjustments (with DNF5 path)
RUN echo "install:x:0:0:root:/root:/usr/libexec/anaconda/run-anaconda" >> /etc/passwd && \
    echo "install::14438:0:99999:7:::" >> /etc/shadow && \
    passwd -d root

RUN mv /usr/share/anaconda/list-harddrives-stub /usr/bin/list-harddrives && \
    mv /usr/share/dnf5/repos.d /etc/anaconda.repos.d && \
    ln -s /lib/systemd/system/anaconda.target /etc/systemd/system/default.target && \
    rm -v /usr/lib/systemd/system-generators/systemd-gpt-auto-generator

RUN ln -s /usr/lib/systemd/system/anaconda-shell@.service /usr/lib/systemd/system/autovt@.service

RUN mkdir -p /usr/lib/systemd/logind.conf.d
COPY <<EOT /usr/lib/systemd/logind.conf.d/anaconda-shell.conf
[Login]
ReserveVT=2
EOT

RUN mkdir -p "$(realpath /root)" && \
    kernel=$(kernel-install list --json pretty | jq -r '.[] | select(.has_kernel == true) | .version') && \
    DRACUT_NO_XATTR=1 dracut --force -v --zstd --reproducible --no-hostonly \
        --add "anaconda" \
        "/usr/lib/modules/${kernel}/initramfs.img" "${kernel}"

RUN mkdir -p /etc/systemd/user/pipewire.service.d/
COPY <<EOT /etc/systemd/user/pipewire.service.d/allowroot.conf
[Unit]
ConditionUser=
EOT

RUN mkdir -p /etc/systemd/user/pipewire.socket.d/
COPY <<EOT /etc/systemd/user/pipewire.socket.d/allowroot.conf
[Unit]
ConditionUser=
EOT
```

## Build Instructions

### Step 1: Update the Containerfile with Your bootc Image

In the `Containerfile` above, locate the following line:

```
bootc --source-imgref registry:IMAGE-NAME --target-imgref "IMAGE-NAME"
```

Replace `IMAGE-NAME` with your custom `bootc` image in both places. Then, build the container image:

```
sudo podman build -t localhost/bootc-generic-iso-f45:latest -f Containerfile
```

### Step 2: Generate the Installer ISO

After building the container image, use it to generate the Anaconda installer ISO for your Fedora 45 `bootc` image (requires `image-builder` to be installed):

```
sudo image-builder build \
  --bootc-ref localhost/bootc-generic-iso-f45:latest \
  --bootc-default-fs ext4 \
  bootc-generic-iso
```

Once the process completes, a new directory will be created in the current working directory containing the generated ISO for bare-metal installation.