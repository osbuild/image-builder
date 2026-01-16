import os
import random
import json
import pathlib
import platform
import string
import subprocess
import textwrap
from contextlib import ExitStack

import pytest
# local test utils
import testutil
from containerbuild import build_container_fixture, make_container    # pylint: disable=unused-import
from vmtest.vm import QEMU


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("container_ref", [
    "quay.io/centos-bootc/centos-bootc:stream10",
    "quay.io/fedora/fedora-bootc:43",
    "quay.io/centos-bootc/centos-bootc:stream9",
])
# pylint: disable=too-many-locals,duplicate-code
def test_bootc_pxe_tar_xz(tmp_path, build_container, container_ref):
    # XXX: duplicated from test_build_disk.py
    username = "test"
    password = "".join(
        random.choices(string.ascii_uppercase + string.digits, k=18))
    ssh_keyfile_private_path = tmp_path / "ssh-keyfile"
    ssh_keyfile_public_path = ssh_keyfile_private_path.with_suffix(".pub")
    if not ssh_keyfile_private_path.exists():
        subprocess.run([
            "ssh-keygen",
            "-N", "",
            # be very conservative with keys for paramiko
            "-b", "2048",
            "-t", "rsa",
            "-f", os.fspath(ssh_keyfile_private_path),
        ], check=True)
    ssh_pubkey = ssh_keyfile_public_path.read_text(encoding="utf8").strip()
    cfg = {
        "customizations": {
            "user": [
                {
                    "name": "root",
                    "key": ssh_pubkey,
                    # note that we have no "home" here for ISOs
                }, {
                    "name": username,
                    "password": password,
                    "groups": ["wheel"],
                },
            ],
            "kernel": {
                # Use console=ttyS0 so that we see output in our debug
                # logs. by default anaconda prints to the last console=
                # from the kernel commandline
                "append": "console=ttyS0",
            },
        },
    }
    config_json_path = tmp_path / "config.json"
    config_json_path.write_text(json.dumps(cfg), encoding="utf-8")
    # create anaconda iso from base
    cntf_path = tmp_path / "Containerfile"
    cntf_path.write_text(textwrap.dedent(f"""\n
    FROM {container_ref}
    RUN dnf install -y \
         dracut-live \
         squashfs-tools \
         && dnf clean all
    RUN bootc container lint
    """), encoding="utf8")
    output_path = tmp_path / "output"
    output_path.mkdir()
    pathlib.Path("/var/tmp/osbuild-test-store").mkdir(exist_ok=True, parents=True)
    with make_container(tmp_path) as container_tag:
        cmd = [
            *testutil.podman_run_common,
            "-v", f"{config_json_path}:/config.json:ro",
            "-v", f"{output_path}:/output",
            "-v", "/var/tmp/osbuild-test-store:/store",  # share the cache between builds
            "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
            build_container,
            "--type", "pxe-tar-xz",
            "--rootfs", "ext4",
            "--installer-payload-ref", container_ref,
            f"localhost/{container_tag}",
        ]
        subprocess.check_call(cmd)
        assert os.path.exists(output_path / "xz" / "pxe.tar.xz")
        # TODO use lzap's pxe boot code from images to boot the result
