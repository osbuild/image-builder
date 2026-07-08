import contextlib
import json
import os
import shutil
import subprocess as sp
import tempfile
import textwrap
from unittest.mock import call, patch

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
    stdout, stderr = testlib.run.runcmd(["/bin/echo", "hello"])
    assert stdout == b"hello\n"
    assert stderr == b""


def test_runcmd_env():
    os.environ["RUNCMD_GLOBAL_TEST_VAR"] = "global test value"
    stdout, stderr = testlib.run.runcmd(["env"], extra_env={"RUNCMD_TEST_VAR": "the test value"})
    assert b"RUNCMD_TEST_VAR=the test value\n" in stdout, "extra env var not set"
    assert b"RUNCMD_GLOBAL_TEST_VAR=global test value\n" in stdout, "global env vars not preserved"
    assert stderr == b""


def test_read_seed():
    # check that it's read without error - no need to test the value itself
    seed_env = testlib.testenv.rng_seed_env()
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
        assert testlib.cache.gen_build_info_dir_path_prefix(**kwargs) == expected


@pytest.mark.parametrize("kwargs,expected", (
    (
        {
            "osbuild_ref": "abcdef123456",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
            "arch": "aarch64",
            "manifest_id": "abc123"
        },
        testlib.cache.S3_BUCKET + "/" + testlib.cache.S3_PREFIX +
        "/osbuild-ref-abcdef123456/runner-fedora-41/fedora-41/aarch64/manifest-id-abc123/",
    ),
    (
        {
            "osbuild_ref": "abcdef123456",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
            "arch": "aarch64",
        },
        testlib.cache.S3_BUCKET + "/" + testlib.cache.S3_PREFIX +
        "/osbuild-ref-abcdef123456/runner-fedora-41/fedora-41/aarch64/",
    ),
    (
        {
            "osbuild_ref": "abcdef123456",
            "runner_distro": "fedora-41",
            "distro": "fedora-41",
        },
        testlib.cache.S3_BUCKET + "/" + testlib.cache.S3_PREFIX +
        "/osbuild-ref-abcdef123456/runner-fedora-41/fedora-41/",
    ),
    (
        {
            "osbuild_ref": "abcdef123456",
            "runner_distro": "fedora-41",
        },
        testlib.cache.S3_BUCKET + "/" + testlib.cache.S3_PREFIX +
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
        testlib.cache.S3_BUCKET + "/" + testlib.cache.S3_PREFIX +
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
        testlib.cache.S3_BUCKET + "/" + testlib.cache.S3_PREFIX +
        "/osbuild-ref-abcdef123456/runner-fedora-41/fedora-41/",
    ),
    # default osbuild_ref
    (
        {
            "runner_distro": "fedora-41",
        },
        testlib.cache.S3_BUCKET + "/" + testlib.cache.S3_PREFIX + "/osbuild-ref-abcdef123456/runner-fedora-41/"
    ),
    # default runner_distro
    (
        {
            "osbuild_ref": "abc123",
        },
        testlib.cache.S3_BUCKET + "/" + testlib.cache.S3_PREFIX + "/osbuild-ref-abc123/runner-fedora-999/"
    ),
    # default osbuild_ref and runner_distro
    (
        {},
        testlib.cache.S3_BUCKET + "/" + testlib.cache.S3_PREFIX + "/osbuild-ref-abcdef123456/runner-fedora-999/"
    ),
))
def test_gen_build_info_s3_dir_path(kwargs, expected):
    # we need to patch the functions that were imported into the cache namespace, not the originals in .testenv
    with patch("imgtestlib.cache.get_host_distro", return_value="fedora-999"), \
         patch("imgtestlib.cache.get_osbuild_commit", return_value="abcdef123456"):
        assert testlib.cache.gen_build_info_s3_dir_path(**kwargs) == expected


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
    assert testlib.core.skopeo_inspect_id(f"{transport}{test_container}:latest", arch) == image_id
    assert testlib.core.skopeo_inspect_id(f"{transport}{test_container}@{manifest_list_digest}", arch) == image_id


@pytest.mark.parametrize("arch", TEST_ARCHES)
def test_skopeo_inspect_image_manifest(arch):
    transport = "docker://"
    manifest_id = manifest_ids[arch]
    image_id = image_ids[arch]
    # arch arg to skopeo_inspect_id doesn't matter here
    assert testlib.core.skopeo_inspect_id(f"{transport}{test_container}@{manifest_id}", arch) == image_id


@pytest.mark.skipif(not can_sudo_nopw(), reason="requires passwordless sudo")
@pytest.mark.parametrize("arch", TEST_ARCHES)
@pytest.mark.skip("disabled")  # disabled: fails in github action - needs work
def test_skopeo_inspect_localstore(arch):
    transport = "containers-storage:"
    image = "registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/manifest-list-test:latest"
    with tempfile.TemporaryDirectory() as tmpdir:
        testlib.run.runcmd(["sudo", "podman", "pull",
                            f"--arch={arch}", "--storage-driver=vfs", f"--root={tmpdir}", image])

        # arch arg to skopeo_inspect_id doesn't matter here
        assert testlib.core.skopeo_inspect_id(f"{transport}[vfs@{tmpdir}]{image}", arch) == image_ids[arch]


def test_find_image_file_single_export():
    """Test find_image_file with a single exported pipeline (excluding build)."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create manifest with build and one export pipeline
        manifest = {
            "pipelines": [
                {"name": "build"},
                {"name": "qcow2"}
            ]
        }
        manifest_path = os.path.join(tmpdir, "manifest.json")
        with open(manifest_path, "w", encoding="utf-8") as f:
            json.dump(manifest, f)

        # Create the export directory and file
        export_dir = os.path.join(tmpdir, "qcow2")
        os.makedirs(export_dir)
        image_file = os.path.join(export_dir, "disk.qcow2")
        with open(image_file, "w", encoding="utf-8") as f:
            f.write("fake image")

        # Test that it finds the correct file
        result = testlib.core.find_image_file(tmpdir)
        assert result == image_file


def test_find_image_file_multiple_pipelines_one_export():
    """Test find_image_file when manifest has multiple pipelines but only one is exported."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create manifest with build and multiple pipelines, but only one exported
        manifest = {
            "pipelines": [
                {"name": "build"},
                {"name": "image"},
                {"name": "qcow2"},
                {"name": "archive"}
            ]
        }
        manifest_path = os.path.join(tmpdir, "manifest.json")
        with open(manifest_path, "w", encoding="utf-8") as f:
            json.dump(manifest, f)

        # Create only the archive directory (the actual export)
        export_dir = os.path.join(tmpdir, "archive")
        os.makedirs(export_dir)
        image_file = os.path.join(export_dir, "image.tar")
        with open(image_file, "w", encoding="utf-8") as f:
            f.write("fake archive")

        # Test that it finds the correct file from the exported pipeline
        result = testlib.core.find_image_file(tmpdir)
        assert result == image_file


def test_find_image_file_no_export_directory():
    """Test find_image_file raises error when no export directory exists."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create manifest but no export directories
        manifest = {
            "pipelines": [
                {"name": "build"},
                {"name": "qcow2"}
            ]
        }
        manifest_path = os.path.join(tmpdir, "manifest.json")
        with open(manifest_path, "w", encoding="utf-8") as f:
            json.dump(manifest, f)

        # Don't create any export directories
        with pytest.raises(RuntimeError, match="Expected exactly one exported pipeline directory"):
            testlib.core.find_image_file(tmpdir)


def test_find_image_file_multiple_export_directories():
    """Test find_image_file raises error when multiple export directories exist."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create manifest with multiple pipelines
        manifest = {
            "pipelines": [
                {"name": "build"},
                {"name": "qcow2"},
                {"name": "vmdk"}
            ]
        }
        manifest_path = os.path.join(tmpdir, "manifest.json")
        with open(manifest_path, "w", encoding="utf-8") as f:
            json.dump(manifest, f)

        # Create multiple export directories
        for pipeline in ["qcow2", "vmdk"]:
            export_dir = os.path.join(tmpdir, pipeline)
            os.makedirs(export_dir)
            with open(os.path.join(export_dir, f"disk.{pipeline}"), "w", encoding="utf-8") as f:
                f.write("fake image")

        # Should raise error about multiple export directories
        with pytest.raises(RuntimeError, match="Expected exactly one exported pipeline directory"):
            testlib.core.find_image_file(tmpdir)


def test_find_image_file_no_files_in_export():
    """Test find_image_file raises error when export directory is empty."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create manifest
        manifest = {
            "pipelines": [
                {"name": "build"},
                {"name": "qcow2"}
            ]
        }
        manifest_path = os.path.join(tmpdir, "manifest.json")
        with open(manifest_path, "w", encoding="utf-8") as f:
            json.dump(manifest, f)

        # Create export directory but no files
        export_dir = os.path.join(tmpdir, "qcow2")
        os.makedirs(export_dir)

        # Should raise error about no files
        with pytest.raises(RuntimeError, match="Expected exactly one file in export directory"):
            testlib.core.find_image_file(tmpdir)


def test_find_image_file_multiple_files_in_export():
    """Test find_image_file raises error when export directory has multiple files."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create manifest
        manifest = {
            "pipelines": [
                {"name": "build"},
                {"name": "qcow2"}
            ]
        }
        manifest_path = os.path.join(tmpdir, "manifest.json")
        with open(manifest_path, "w", encoding="utf-8") as f:
            json.dump(manifest, f)

        # Create export directory with multiple files
        export_dir = os.path.join(tmpdir, "qcow2")
        os.makedirs(export_dir)
        for filename in ["disk1.qcow2", "disk2.qcow2"]:
            with open(os.path.join(export_dir, filename), "w", encoding="utf-8") as f:
                f.write("fake image")

        # Should raise error about multiple files
        with pytest.raises(RuntimeError, match="Expected exactly one file in export directory"):
            testlib.core.find_image_file(tmpdir)


def test_get_free_port():
    port_nr = testlib.vm.get_free_port()
    assert 1024 < port_nr < 65535


@patch("time.sleep")
def test_wait_ssh_ready_sleeps_no_connection(mocked_sleep):
    free_port = testlib.vm.get_free_port()
    with pytest.raises(ConnectionRefusedError):
        testlib.vm.wait_ssh_ready("localhost", free_port, sleep=0.1, max_wait_sec=0.35)
    assert mocked_sleep.call_args_list == [call(0.1), call(0.1), call(0.1)]


@pytest.mark.skipif(not shutil.which("nc"), reason="needs nc")
def test_wait_ssh_ready_sleeps_wrong_reply():
    free_port = testlib.vm.get_free_port()
    with contextlib.ExitStack() as cm:
        with sp.Popen(
            f"echo not-ssh | nc -vv -l {free_port}",
            shell=True,
            stdout=sp.PIPE,
            stderr=sp.STDOUT,
            encoding="utf-8",
        ) as p:
            cm.callback(p.kill)
            # wait for nc to be ready
            while True:
                # netcat tranditional uses "listening", others "Listening"
                # so just omit the first char
                if "istening " in p.stdout.readline():
                    break
            # now connect
            with patch("time.sleep") as mocked_sleep:
                with pytest.raises(ConnectionRefusedError):
                    testlib.vm.wait_ssh_ready("localhost", free_port, sleep=0.1, max_wait_sec=0.55)
                assert mocked_sleep.call_args_list == [
                    call(0.1), call(0.1), call(0.1), call(0.1), call(0.1)]


class MockVM(testlib.vm.VM):
    _address = None
    _ssh_port = None

    def start(self):
        pass

    def force_stop(self):
        pass

    def running(self):
        return True

    def set_ssh(self, address, port):
        self._address = address
        self._ssh_port = port


def make_fake_ssh(fake_bin_path, extra_script=""):
    mock_ssh = fake_bin_path / "ssh"
    mock_ssh.write_text(textwrap.dedent(f"""\
    #!/bin/bash -e
    echo "calling $0 with: $@"
    {extra_script}
    """))
    mock_ssh.chmod(0o755)
    return mock_ssh


def test_ssh_calls_cmd_happy(tmp_path, monkeypatch):
    monkeypatch.setenv("PATH", os.fspath(tmp_path), prepend=os.pathsep)
    make_fake_ssh(tmp_path)
    vm = MockVM()
    res = vm.run(["cmd1", "arg1", "arg2"], user="user1", keyfile="keyfile1")
    assert res.returncode == 0
    assert res.stdout.endswith("cmd1 arg1 arg2\n")


def test_ssh_calls_cmd_happy_single_cmd(tmp_path, monkeypatch):
    monkeypatch.setenv("PATH", os.fspath(tmp_path), prepend=os.pathsep)
    make_fake_ssh(tmp_path)
    vm = MockVM()
    res = vm.run("true", user="user1", keyfile="keyfile1")
    assert res.returncode == 0
    assert res.stdout.endswith("true\n")


def test_ssh_calls_cmd_happy_quoting_works(tmp_path, monkeypatch):
    monkeypatch.setenv("PATH", os.fspath(tmp_path), prepend=os.pathsep)
    make_fake_ssh(tmp_path)
    vm = MockVM()
    res = vm.run("this needs quoting", user="user1", keyfile="keyfile1")
    assert res.returncode == 0
    assert res.stdout.endswith("'this needs quoting'\n")


def test_ssh_calls_cmd_sad(tmp_path, monkeypatch):
    monkeypatch.setenv("PATH", os.fspath(tmp_path), prepend=os.pathsep)
    make_fake_ssh(tmp_path, """ echo bad-output ; if [ "${@: -1}" = "bad-cmd" ]; then exit 42; fi """)
    vm = MockVM()
    with pytest.raises(sp.CalledProcessError) as e:
        vm.run("bad-cmd", user="user1", keyfile="keyfile1")
    assert e.value.returncode == 42
    assert e.value.stdout.endswith("bad-output\n")


@patch("time.sleep")
def test_ssh_calls_retries(mocked_sleep, tmp_path, monkeypatch, capsys):
    monkeypatch.setenv("PATH", os.fspath(tmp_path), prepend=os.pathsep)
    make_fake_ssh(tmp_path, "echo ssh-very-sad; exit 21")
    vm = MockVM()
    with pytest.raises(RuntimeError) as e:
        vm.run(["cmd1", "arg1", "arg2"], user="user1", keyfile="keyfile1")
    assert str(e.value) == "no ssh after 30 retries of 10s"
    assert mocked_sleep.call_args_list == 30 * [call(10)]
    assert capsys.readouterr().err == "\n".join([
        f"ssh not ready {i+1}/30: Command 'true' returned non-zero exit status 21."
        for i in range(30)
    ]) + "\n"


def test_wait_ssh_ready_timeout():
    vm = MockVM()
    vm.set_ssh("localhost", testlib.vm.get_free_port())
    with pytest.raises(ConnectionRefusedError) as e:
        vm.wait_ssh_ready(timeout_sec=3)
    assert "after 3s" in str(e.value)
