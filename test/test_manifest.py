import json
import os
import platform
import re
import subprocess

import pytest

# put common podman run args in once place
podman_run = [
    "podman", "run", "--rm", "--privileged",
    "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
]


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_manifest_generates_sbom(tmp_path, build_container):
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


@pytest.mark.parametrize("use_seed_arg", [False, True])
@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_manifest_seeded_is_the_same(build_container, use_seed_arg):
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


def test_manifest_bootc_build_container(build_container):
    bootc_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    bootc_build_container_ref = "quay.io/centos-bootc/centos-bootc:stream10"
    subprocess.check_call(["podman", "pull", bootc_ref])
    subprocess.check_call(["podman", "pull", bootc_build_container_ref])

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


def test_container_manifest_bootc_iso_smoke(build_container):
    # Note that this is not a realistic ref, a generic bootc
    # image does not contain anaconda so this won't produce a
    # working installer. For the purpose of the test to validate
    # that we get a manifest with the right refs its good enough.
    bootc_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    bootc_payload_ref = "quay.io/centos-bootc/centos-bootc:stream10"
    subprocess.check_call(["podman", "pull", bootc_ref])
    subprocess.check_call(["podman", "pull", bootc_payload_ref])
    output = subprocess.check_output(podman_run + [
        build_container,
        "manifest",
        "bootc-installer",
        "--bootc-ref", bootc_ref,
        "--bootc-installer-payload-ref", bootc_payload_ref
    ], text=True)
    manifest = json.loads(output)
    assert len(manifest["sources"]["org.osbuild.containers-storage"]["items"]) == 2
    assert bootc_ref in output
    assert bootc_payload_ref in output
    # build pipeline uses bootc-ref
    pipeline = [p for p in manifest["pipelines"]
                if p["name"] == "build"][0]
    cnt_deploy = [st for st in pipeline["stages"]
                  if st["type"] == "org.osbuild.container-deploy"][0]
    refs = cnt_deploy["inputs"]["images"]["references"]
    assert refs.popitem()[1]["name"] == bootc_ref
    # anaconda-tree uses bootc-ref too
    pipeline = [p for p in manifest["pipelines"]
                if p["name"] == "anaconda-tree"][0]
    cnt_deploy = [st for st in pipeline["stages"]
                  if st["type"] == "org.osbuild.container-deploy"][0]
    refs = cnt_deploy["inputs"]["images"]["references"]
    assert refs.popitem()[1]["name"] == bootc_ref
    # the payload container is centos10
    pipeline = [p for p in manifest["pipelines"]
                if p["name"] == "bootiso-tree"][0]
    skopeo_stage = [st for st in pipeline["stages"]
                    if st["type"] == "org.osbuild.skopeo"][0]
    refs = skopeo_stage["inputs"]["images"]["references"]
    assert refs.popitem()[1]["name"] == bootc_payload_ref
    kickstart_stage = [st for st in pipeline["stages"]
                       if st["type"] == "org.osbuild.kickstart"][0]
    assert re.match(
        f'bootc switch .* registry {bootc_payload_ref}',
        kickstart_stage["options"]["%post"][0]["commands"][1])


def test_manifest_honors_rpmmd_cache(tmp_path, build_container):
    rpmmd_cache = tmp_path / "rpmmd"
    rpmmd_cache.mkdir()
    subprocess.check_call(podman_run + [
        "-v", f"{rpmmd_cache}:/rpmmd_cache",
        build_container,
        "manifest",
        "--distro", "centos-9",
        "minimal-raw",
        "--rpmmd-cache", "/rpmmd_cache",
    ], text=True)
    assert len(list(rpmmd_cache.rglob("repomd.xml"))) > 0


@pytest.mark.parametrize("bootc_ref,defaultfs", [
    ("quay.io/fedora/fedora-bootc:42", "ext4"),
    ("quay.io/centos-bootc/centos-bootc:stream10", ""),
    ("quay.io/centos-bootc/centos-bootc:stream9", ""),
])
def test_container_manifest_bootc_smoke(build_container, bootc_ref, defaultfs):
    subprocess.check_call(["podman", "pull", bootc_ref])
    bootc_default_fs = []
    if defaultfs:
        bootc_default_fs = ["--bootc-default-fs", defaultfs]
    output = subprocess.check_output(podman_run + [
        build_container,
        "manifest",
        "qcow2",
        "--bootc-ref", bootc_ref,
    ] + bootc_default_fs, text=True)
    manifest = json.loads(output)
    assert manifest["version"] == "2"
    if defaultfs:
        pipeline = [p for p in manifest["pipelines"] if p["name"] == "image"][0]
        mkfs_stages = [st for st in pipeline["stages"]
                       if st["type"] == f"org.osbuild.mkfs.{defaultfs}"]
        assert mkfs_stages
