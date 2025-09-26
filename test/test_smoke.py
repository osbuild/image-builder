import os
import subprocess

import pytest
import yaml

# put common podman run args in once place
podman_run = ["podman", "run", "--rm", "--privileged"]


def test_smoke_has_expected_images_centos(build_container):
    """
    Ensure that image types that are built in by CentOS are available
    and do not disappear from the list. See:
    https://gitlab.com/redhat/centos-stream/release-engineering/releng-tools/-/blob/master/scripts/images-build-gen2.py
    """

    output = subprocess.check_output(podman_run + [
        build_container,
        "list",
    ], text=True)

    type_arch = {
        "tar": ["aarch64", "x86_64", "ppc64le", "s390x"],
        "qcow2": ["aarch64", "x86_64", "ppc64le", "s390x"],
        "ec2": ["x86_64", "aarch64"],
        "azure": ["x86_64", "aarch64"],
        "wsl": ["x86_64", "aarch64"],
        "vagrant-libvirt": ["x86_64"],
        "vagrant-virtualbox": ["x86_64"],
        "image-installer": ["x86_64", "aarch64"],
    }

    for distro in ["centos-9", "centos-10"]:
        for type_, arches in type_arch.items():
            for arch in arches:
                assert f"{distro} type:{type_} arch:{arch}" in output


def test_smoke_has_expected_images_fedora(build_container):
    """
    Ensure that image types that are built in by Fedora are available
    and do not disappear from the list. See:
    https://pagure.io/pungi-fedora/blob/main/f/fedora.conf
    and
    https://pagure.io/fedora-iot/pungi-iot/blob/main/f/fedora-iot.conf
    """

    output = subprocess.check_output(podman_run + [
        build_container,
        "list",
    ], text=True)

    type_arch = {
        "minimal-raw-xz": ["aarch64"],
        "iot-raw-xz": ["x86_64", "aarch64"],
        "iot-installer": ["x86_64", "aarch64"],
        "iot-simplified-installer": ["x86_64", "aarch64"],
    }

    for distro in ["fedora-42", "fedora-43", "fedora-44"]:
        for type_, arches in type_arch.items():
            for arch in arches:
                assert f"{distro} type:{type_} arch:{arch}" in output


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_smoke_version_smoke(build_container):
    output = subprocess.check_output(podman_run + [
        build_container,
        "--version",
    ])

    ver_yaml = yaml.load(output, yaml.loader.SafeLoader)

    assert ver_yaml["image-builder"]["version"] != ""
    assert ver_yaml["image-builder"]["commit"] != ""
    assert ver_yaml["image-builder"]["dependencies"]["images"] != ""
    assert ver_yaml["image-builder"]["dependencies"]["osbuild"] != ""
