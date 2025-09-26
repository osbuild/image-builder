import json
import os
import platform
import subprocess

import pytest
import yaml

# put common podman run args in once place
podman_run = ["podman", "run", "--rm", "--privileged"]


@pytest.mark.parametrize("use_librepo", [False, True])
@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_container_builds_image(tmp_path, build_container, use_librepo):
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
def test_container_manifest_generates_sbom(tmp_path, build_container):
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call(podman_run + [
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
def test_container_with_progress(tmp_path, build_fake_container, progress, needle, forbidden):
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


# only test a subset here to avoid overly long runtimes
@pytest.mark.parametrize("arch", ["aarch64", "ppc64le", "riscv64", "s390x"])
def test_container_cross_build(tmp_path, build_container, arch):
    # this is only here to speed up builds by sharing downloaded stuff
    # when this is run locally (we could cache via GH action though)
    os.makedirs("/var/cache/image-builder/store", exist_ok=True)
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call(podman_run + [
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

    cmd = podman_run + [
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


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_container_version_smoke(build_container):
    output = subprocess.check_output(podman_run + [
        build_container,
        "--version",
    ])

    ver_yaml = yaml.load(output, yaml.loader.SafeLoader)

    assert ver_yaml["image-builder"]["version"] != ""
    assert ver_yaml["image-builder"]["commit"] != ""
    assert ver_yaml["image-builder"]["dependencies"]["images"] != ""
    assert ver_yaml["image-builder"]["dependencies"]["osbuild"] != ""


def test_container_builds_bootc(tmp_path, build_container):
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


def test_container_manifest_bootc_build_container(build_container):
    bootc_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    bootc_build_container_ref = "quay.io/centos-bootc/centos-bootc:stream10"
    subprocess.check_call(["podman", "pull", bootc_ref])

    output = subprocess.check_output(podman_run + [
        build_container,
        "manifest",
        "qcow2",
        "--bootc-ref", bootc_ref,
        "--bootc-build-ref", bootc_build_container_ref
    ], text=True)
    manifest = json.loads(output)
    assert len(manifest["sources"]["org.osbuild.containers-storage"]["items"]) == 2
    assert bootc_ref in output
    assert bootc_build_container_ref in output
    # build container is set correctly
    build_pipeline = [p for p in manifest["pipelines"]
                      if p["name"] == "build"][0]
    cnt_deploy = [st for st in build_pipeline["stages"]
                  if st["type"] == "org.osbuild.container-deploy"][0]
    refs = cnt_deploy["inputs"]["images"]["references"]
    assert refs.popitem()[1]["name"] == "quay.io/centos-bootc/centos-bootc:stream10"
    # target is correct
    img_pipeline = [p for p in manifest["pipelines"]
                    if p["name"] == "image"][0]
    cnt_deploy = [st for st in img_pipeline["stages"]
                  if st["type"] == "org.osbuild.bootc.install-to-filesystem"][0]
    assert cnt_deploy["options"]["target-imgref"] == "quay.io/centos-bootc/centos-bootc:stream9"


def test_container_has_expected_images_centos(build_container):
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


def test_container_has_expected_images_fedora(build_container):
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
