import json
import os
import platform
import subprocess

import pytest


@pytest.mark.parametrize("use_librepo", [False, True])
@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_container_builds_image(tmp_path, build_container, use_librepo):
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call([
        "podman", "run",
        "--privileged",
        "-v", f"{output_dir}:/output",
        build_container,
        "build",
        "minimal-raw",
        "--distro", "centos-9",
        f"--use-librepo={use_librepo}",
    ])
    arch = "x86_64"
    assert (output_dir / f"centos-9-minimal-raw-{arch}/xz/disk.raw.xz").exists()
    # XXX: ensure no other leftover dirs
    dents = os.listdir(output_dir)
    assert len(dents) == 1, f"too many dentries in output dir: {dents}"


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_container_manifest_generates_sbom(tmp_path, build_container):
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call([
        "podman", "run",
        "--privileged",
        "-v", f"{output_dir}:/output",
        build_container,
        "manifest",
        "minimal-raw",
        "--distro", "centos-9",
        "--with-sbom",
    ], stdout=subprocess.DEVNULL)
    arch = platform.machine()
    fn = f"centos-9-minimal-raw-{arch}/centos-9-minimal-raw-{arch}.image-os.spdx.json"
    image_sbom_json_path = output_dir / fn
    assert image_sbom_json_path.exists()
    fn = f"centos-9-minimal-raw-{arch}/centos-9-minimal-raw-{arch}.buildroot-build.spdx.json"
    buildroot_sbom_json_path = output_dir / fn
    assert buildroot_sbom_json_path.exists()
    sbom_json = json.loads(image_sbom_json_path.read_text())
    # smoke test that we have glibc in the json doc
    assert "glibc" in [s["name"] for s in sbom_json["packages"]], f"missing glibc in {sbom_json}"


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_container_build_generates_manifest(tmp_path, build_container):
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call([
        "podman", "run",
        "--privileged",
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
def test_container_with_progress(tmp_path, build_fake_container, progress, needle, forbidden):
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    output = subprocess.check_output([
        "podman", "run", "-t",
        "--privileged",
        "-v", f"{output_dir}:/output",
        build_fake_container,
        "build",
        "minimal-raw",
        "--distro", "centos-9",
        f"--progress={progress}",
    ], text=True)
    assert needle in output
    assert forbidden not in output
