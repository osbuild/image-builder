import os
import subprocess as sp
import tempfile
from unittest.mock import patch

import pytest

import imgtestlib as testlib

TEST_ARCHES = ["amd64", "arm64", "ppc64le", "s390x"]


def can_sudo_nopw() -> bool:
    """
    Check if we can run sudo without a password.
    """
    job = sp.run(["sudo", "-n", "true"], capture_output=True, check=False)
    return job.returncode == 0


def test_runcmd():
    stdout, stderr = testlib.runcmd(["/bin/echo", "hello"])
    assert stdout == b"hello\n"
    assert stderr == b""


def test_runcmd_env():
    os.environ["RUNCMD_GLOBAL_TEST_VAR"] = "global test value"
    stdout, stderr = testlib.runcmd(["env"], extra_env={"RUNCMD_TEST_VAR": "the test value"})
    assert b"RUNCMD_TEST_VAR=the test value\n" in stdout, "extra env var not set"
    assert b"RUNCMD_GLOBAL_TEST_VAR=global test value\n" in stdout, "global env vars not preserved"
    assert stderr == b""


def test_read_seed():
    # check that it's read without error - no need to test the value itself
    seed_env = testlib.rng_seed_env()
    assert "OSBUILD_TESTING_RNG_SEED" in seed_env


@pytest.mark.parametrize("kwargs,expected", (
    (
        {
            "osbuild_ref": "abc123",
            "runner_distro": "fedora-41",
        },
        "osbuild-ref-abc123/runner-fedora-41/"
    ),
    (
        {
            "osbuild_ref": "abc123",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
        },
        "osbuild-ref-abc123/runner-fedora-41/fedora-41/"
    ),
    (
        {
            "osbuild_ref": "abc123",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
            "arch": "x86_64",
        },
        "osbuild-ref-abc123/runner-fedora-41/fedora-41/x86_64/"
    ),
    (
        {
            "osbuild_ref": "abc123",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
            "arch": "x86_64",
            "manifest_id": "abc123123",
        },
        "osbuild-ref-abc123/runner-fedora-41/fedora-41/x86_64/manifest-id-abc123123/"
    ),
    # Optional arg 'distro' not specified, thus following optional args 'arch' and 'manifest_id' are ignored
    (
        {
            "osbuild_ref": "abc123",
            "runner_distro": "fedora-41",
            "arch": "x86_64",
            "manifest_id": "abc123123"
        },
        "osbuild-ref-abc123/runner-fedora-41/"
    ),
    # Optional arg 'arch' not specified, thus following optional arg 'manifest_id' is ignored
    (
        {
            "osbuild_ref": "abc123",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
            "manifest_id": "abc123123"
        },
        "osbuild-ref-abc123/runner-fedora-41/fedora-41/"
    ),
    # default osbuild_ref
    (
        {
            "runner_distro": "fedora-41",
        },
        "osbuild-ref-abcdef123456/runner-fedora-41/"
    ),
    # default runner_distro
    (
        {
            "osbuild_ref": "abc123",
        },
        "osbuild-ref-abc123/runner-fedora-999/"
    ),
    # default osbuild_ref and runner_distro
    (
        {},
        "osbuild-ref-abcdef123456/runner-fedora-999/"
    ),
))
def test_gen_build_info_dir_path_prefix(kwargs, expected):
    # we need to patch the functions that were imported into the cache namespace, not the originals in .testenv
    with patch("imgtestlib.cache.get_host_distro", return_value="fedora-999"), \
         patch("imgtestlib.cache.get_osbuild_commit", return_value="abcdef123456"):
        assert testlib.gen_build_info_dir_path_prefix(**kwargs) == expected


@pytest.mark.parametrize("kwargs,expected", (
    (
        {
            "osbuild_ref": "abcdef123456",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
            "arch": "aarch64",
            "manifest_id": "abc123"
        },
        testlib.S3_BUCKET + "/" + testlib.S3_PREFIX +
        "/osbuild-ref-abcdef123456/runner-fedora-41/fedora-41/aarch64/manifest-id-abc123/",
    ),
    (
        {
            "osbuild_ref": "abcdef123456",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
            "arch": "aarch64",
        },
        testlib.S3_BUCKET + "/" + testlib.S3_PREFIX +
        "/osbuild-ref-abcdef123456/runner-fedora-41/fedora-41/aarch64/",
    ),
    (
        {
            "osbuild_ref": "abcdef123456",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
        },
        testlib.S3_BUCKET + "/" + testlib.S3_PREFIX +
        "/osbuild-ref-abcdef123456/runner-fedora-41/fedora-41/",
    ),
    (
        {
            "osbuild_ref": "abcdef123456",
            "runner_distro": "fedora-41",
        },
        testlib.S3_BUCKET + "/" + testlib.S3_PREFIX +
        "/osbuild-ref-abcdef123456/runner-fedora-41/",
    ),
    # Optional arg 'distro' not specified, thus following optional args 'arch' and 'manifest_id' are ignored
    (
        {
            "osbuild_ref": "abcdef123456",
            "runner_distro": "fedora-41",
            "arch": "aarch64",
            "manifest_id": "abc123"
        },
        testlib.S3_BUCKET + "/" + testlib.S3_PREFIX +
        "/osbuild-ref-abcdef123456/runner-fedora-41/",
    ),
    # Optional arg 'arch' not specified, thus following optional arg 'manifest_id' is ignored
    (
        {
            "osbuild_ref": "abcdef123456",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
            "manifest_id": "abc123"
        },
        testlib.S3_BUCKET + "/" + testlib.S3_PREFIX +
        "/osbuild-ref-abcdef123456/runner-fedora-41/fedora-41/",
    ),
    # default osbuild_ref
    (
        {
            "runner_distro": "fedora-41",
        },
        testlib.S3_BUCKET + "/" + testlib.S3_PREFIX + "/osbuild-ref-abcdef123456/runner-fedora-41/"
    ),
    # default runner_distro
    (
        {
            "osbuild_ref": "abc123",
        },
        testlib.S3_BUCKET + "/" + testlib.S3_PREFIX + "/osbuild-ref-abc123/runner-fedora-999/"
    ),
    # default osbuild_ref and runner_distro
    (
        {},
        testlib.S3_BUCKET + "/" + testlib.S3_PREFIX + "/osbuild-ref-abcdef123456/runner-fedora-999/"
    ),
))
def test_gen_build_info_s3_dir_path(kwargs, expected):
    # we need to patch the functions that were imported into the cache namespace, not the originals in .testenv
    with patch("imgtestlib.cache.get_host_distro", return_value="fedora-999"), \
         patch("imgtestlib.cache.get_osbuild_commit", return_value="abcdef123456"):
        assert testlib.gen_build_info_s3_dir_path(**kwargs) == expected


test_container = "registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/manifest-list-test"

manifest_list_digest = "sha256:11b8172c893595bfcdea054e15457deb54f208afdaf16a602d05fad8e1f6adc1"

# manifest IDs for
#  registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/manifest-list-test:latest
manifest_ids = {
    "amd64": "sha256:78bd1e02fb7b18fbcec064ec10eba0dabfbe94ab6c4513f4edf9e8c8e13396a5",
    "arm64": "sha256:33d37d6b9a9ce7935494d053a0777ec7f764a75ad623a6f1e5285eb5ab785b6b",
    "ppc64le": "sha256:8583990f74a6909c4267ba4e75a2e16c52c41c2a23bf237e12d980846c41efc3",
    "s390x": "sha256:e34cb0973da8876807f945d9c0869c419b364cbcc5a1c069366cf370c429dbda",
}

# image IDs for
#  registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/manifest-list-test:latest
image_ids = {
    "amd64": "sha256:d0f28374e1a9b1ae28d543cadd0916b8ee4f8619a834900a964961a188bc16c4",
    "arm64": "sha256:d012c8612b9c700f7651aca3f6b0853077ea97c306b97fd614c846929cc28ac4",
    "ppc64le": "sha256:55da7d7b0a5a4585d40f4fe1922c6d42d99868d9f05aca6c879b7a9ea26693bc",
    "s390x": "sha256:5bf4c5066f83f30a20b934cee2b6e84e4eb8c44362409776fe7376289ae52acd",
}


@pytest.mark.parametrize("arch", TEST_ARCHES)
def test_skopeo_inspect_id_manifest_list(arch):
    transport = "docker://"
    image_id = image_ids[arch]
    assert testlib.skopeo_inspect_id(f"{transport}{test_container}:latest", arch) == image_id
    assert testlib.skopeo_inspect_id(f"{transport}{test_container}@{manifest_list_digest}", arch) == image_id


@pytest.mark.parametrize("arch", TEST_ARCHES)
def test_skopeo_inspect_image_manifest(arch):
    transport = "docker://"
    manifest_id = manifest_ids[arch]
    image_id = image_ids[arch]
    # arch arg to skopeo_inspect_id doesn't matter here
    assert testlib.skopeo_inspect_id(f"{transport}{test_container}@{manifest_id}", arch) == image_id


@pytest.mark.skipif(not can_sudo_nopw(), reason="requires passwordless sudo")
@pytest.mark.parametrize("arch", TEST_ARCHES)
@pytest.mark.skip("disabled")  # disabled: fails in github action - needs work
def test_skopeo_inspect_localstore(arch):
    transport = "containers-storage:"
    image = "registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/manifest-list-test:latest"
    with tempfile.TemporaryDirectory() as tmpdir:
        testlib.runcmd(["sudo", "podman", "pull", f"--arch={arch}", "--storage-driver=vfs", f"--root={tmpdir}", image])

        # arch arg to skopeo_inspect_id doesn't matter here
        assert testlib.skopeo_inspect_id(f"{transport}[vfs@{tmpdir}]{image}", arch) == image_ids[arch]
