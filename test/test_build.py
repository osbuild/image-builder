import os
import platform
import subprocess

import pytest

# put common podman run args in once place
podman_run = ["podman", "run", "--rm", "--privileged"]


@pytest.mark.parametrize("use_librepo", [False, True])
@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_build_builds_image(tmp_path, build_container, use_librepo):
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call(podman_run + [
        "-v", f"{output_dir}:/output",
        build_container,
        "build",
        "minimal-raw",
        "--distro", "centos-9",
        f"--use-librepo={use_librepo}",
    ])
    arch = "x86_64"
    basename = f"centos-9-minimal-raw-{arch}"
    assert (output_dir / basename / f"{basename}.raw.xz").exists()
    # XXX: ensure no other leftover dirs
    dents = os.listdir(output_dir)
    assert len(dents) == 1, f"too many dentries in output dir: {dents}"


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_build_build_generates_manifest(tmp_path, build_container):
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call(podman_run + [
        "-v", f"{output_dir}:/output",
        build_container,
        "build",
        "minimal-raw",
        "--distro", "centos-9",
        "--with-manifest",
    ], stdout=subprocess.DEVNULL)
    arch = platform.machine()
    fn = f"centos-9-minimal-raw-{arch}/centos-9-minimal-raw-{arch}.osbuild-manifest.json"
    image_manifest_path = output_dir / fn
    assert image_manifest_path.exists()


@pytest.mark.parametrize("progress,needle,forbidden", [
    ("verbose", "osbuild-stdout-output", "[|]"),
    ("term", "[|]", "osbuild-stdout-output"),
])
@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_build_with_progress(tmp_path, build_fake_container, progress, needle, forbidden):
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    output = subprocess.check_output(podman_run + [
        "-t",
        "-v", f"{output_dir}:/output",
        build_fake_container,
        "build",
        "qcow2",
        "--distro", "centos-9",
        "--output-dir=.",
        f"--progress={progress}",
    ], text=True)
    assert needle in output
    assert forbidden not in output


def test_build_builds_bootc(tmp_path, build_container):
    bootc_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    subprocess.check_call(["podman", "pull", bootc_ref])

    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call(podman_run + [
        "-v", f"{output_dir}:/output",
        build_container,
        "build",
        "qcow2",
        "--bootc-ref", bootc_ref,
    ])
    arch = "x86_64"
    basename = f"bootc-centos-9-qcow2-{arch}"
    assert (output_dir / basename / f"{basename}.qcow2").exists()
    # XXX: ensure no other leftover dirs
    dents = os.listdir(output_dir)
    assert len(dents) == 1, f"too many dentries in output dir: {dents}"
