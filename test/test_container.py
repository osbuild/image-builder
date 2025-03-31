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
    basename = f"centos-9-minimal-raw-{arch}"
    assert (output_dir / basename / f"{basename}.raw.xz").exists()
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
        "qcow2",
        "--distro", "centos-9",
        "--output-dir=.",
        f"--progress={progress}",
    ], text=True)
    assert needle in output
    assert forbidden not in output


# only test a subset here to avoid overly long runtimes
@pytest.mark.parametrize("arch", ["aarch64", "ppc64le", "riscv64", "s390x"])
def test_container_cross_build(tmp_path, build_container, arch):
    # this is only here to speed up builds by sharing downloaded stuff
    # when this is run locally (we could cache via GH action though)
    os.makedirs("/var/cache/image-builder/store", exist_ok=True)
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call([
        "podman", "run",
        "--privileged",
        "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
        "-v", "/var/cache/image-builder/store:/var/cache/image-builder/store",
        "-v", f"{output_dir}:/output",
        build_container,
        "build",
        "--progress=verbose",
        "--output-dir=/output",
        "container",
        "--distro", "fedora-41",
        # selecting a foreign arch here automatically triggers a cross-build
        f"--arch={arch}",
    ], text=True)
    assert os.path.exists(output_dir / f"fedora-41-container-{arch}.tar")


@pytest.mark.parametrize("use_seed_arg", [False, True])
@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_container_manifest_seeded_is_the_same(build_container, use_seed_arg):
    manifests = set()

    cmd = [
        "podman", "run",
        "--privileged",
        build_container,
        "manifest",
        "--distro", "centos-9",
        "minimal-raw",
    ]

    if use_seed_arg:
        cmd.extend(["--seed", "0"])

    for _ in range(3):
        p = subprocess.run(
            cmd,
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE)

        manifests.add(p.stdout)

    # verify all calls with the same seed generated the same manifest
    if use_seed_arg:
        assert len(manifests) == 1
    else:
        print(cmd)
        assert len(manifests) == 3
