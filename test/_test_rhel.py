import contextlib
import os
import shutil
import subprocess

import pytest

podman_run = [
    "podman", "run", "--rm", "--privileged",
    "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
]


@contextlib.contextmanager
def subscribed_host():
    org = os.getenv("SUBSCRIPTION_ORG")
    key = os.getenv("SUBSCRIPTION_ACTIVATION_KEY")
    try:
        subprocess.check_call([
            "subscription-manager", "register",
            f"--org={org}", f"--activationkey={key}",
        ])
        yield
    finally:
        subprocess.check_call(["subscription-manager", "unregister"])


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
@pytest.mark.skipif(not shutil.which("subscription-manager"), reason="needs subscription-manager")
@pytest.mark.skipif(not os.getenv("SUBSCRIPTION_ORG"), reason="needs a subscription secret")
def test_build_rhel(build_container):
    with subscribed_host():
        subprocess.check_call(podman_run + [
            build_container,
            "build",
            "tar",
            "--distro", "rhel-10.0",
        ])


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_build_can_build_tar(build_container):
    subprocess.check_call(podman_run + [
        build_container,
        "build",
        "tar",
        "--distro", "centos-10",
    ])


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
@pytest.mark.skipif(not os.getenv("RHEL_REGISTRY_USER"), reason="needs registry secrets")
def test_rhel_bootc_anaconda_iso(tmp_path):
    rhel_registry_user = os.getenv("RHEL_REGISTRY_USER")
    rhel_registry_secret = os.getenv("RHEL_REGISTRY_TOKEN")
    bootc_ref = "registry.redhat.io/rhel10/rhel-bootc:latest"
    subprocess.run([
        "podman",
        "login", "registry.redhat.io",
        "--username", rhel_registry_user,
        "--password-stdin",
    ], input=rhel_registry_secret, text=True, check=True)
    subprocess.check_call(["podman", "pull", bootc_ref])
    bib = tmp_path / "bootc-image-builder"
    subprocess.check_call(["go", "build", "-o", f"{bib}", "./cmd/image-builder"])
    subprocess.check_call([
        os.fspath(bib),
        "manifest",
        "--type", "anaconda-iso",
        bootc_ref,
    ])
