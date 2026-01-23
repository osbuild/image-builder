import os
import random
import json
import pathlib
import platform
import string
import subprocess
import textwrap
import threading

from contextlib import ExitStack
from http.server import ThreadingHTTPServer, SimpleHTTPRequestHandler
from tempfile import TemporaryDirectory

import pytest
# local test utils
import testutil
from containerbuild import build_container_fixture, make_container    # pylint: disable=unused-import
from vmtest.vm import QEMU


def get_ostree_path(grub_path):
    """Get the ostree= boot path from a grub.cfg file"""
    with open(grub_path, encoding="utf-8") as f:
        for line in f.readlines():
            if "ostree=" not in line:
                continue
            args = line.split()
            for a in args:
                if a.startswith("ostree="):
                    return a
    return ""


class DirServer(ThreadingHTTPServer):
    def __init__(self, address, directory):
        super().__init__(address, SimpleHTTPRequestHandler)
        self.directory = directory

    def finish_request(self, request, client_address):
        SimpleHTTPRequestHandler(request, client_address, self, directory=self.directory)


# pylint: disable=too-many-arguments,too-many-locals,unused-argument
def boot_qemu_pxe(arch, pxe_tar_path, container_ref, username, password, ssh_key_path, keep=False):
    with ExitStack() as cm:
        # unpack the tar and create a combined image
        tmpdir = cm.enter_context(TemporaryDirectory(dir="/var/tmp", prefix="qemu-pxe-", delete=not keep))
        subprocess.check_call(
            ["tar", "-C", tmpdir, "-x", "-f", pxe_tar_path])
        subprocess.check_call(
            "echo rootfs.img | cpio -H newc --quiet -L -o > rootfs.cpio", shell=True, cwd=tmpdir)
        subprocess.check_call(
            "cat initrd.img rootfs.cpio > combined.img", shell=True, cwd=tmpdir)

        # Get the ostree= kernel cmdline argument from grub.cfg
        ostree_path = get_ostree_path(pathlib.Path(tmpdir) / "grub.cfg")
        assert ostree_path.startswith("ostree=")

        # Setup http server thread for the rootfs.img
        http_server = DirServer(('127.0.0.1', 0), tmpdir)
        http_port = http_server.server_port
        threading.Thread(target=http_server.serve_forever).start()
        try:
            # test disk is unused for live OS
            test_disk_path = pathlib.Path(tmpdir) / "disk.img"
            with open(test_disk_path, "w", encoding="utf-8") as fp:
                fp.truncate(0)

            # test both the combined and HTTP rootfs variants
            for use_ovmf in [False, True]:
                for root_arg, initrd_file in [
                    ("live:/rootfs.img", "combined.img"),
                    (f"live:http://10.0.2.2:{http_port}/rootfs.img", "initrd.img")
                ]:
                    append_arg = (
                        f"rd.live.image root={root_arg} rw console=ttyS0 "
                        f"systemd.debug-shell=ttyS0 "
                        f"{ostree_path}"
                    )
                    extra_args = [
                        "-kernel", str(pathlib.Path(tmpdir) / "vmlinuz"),
                        "-initrd", str(pathlib.Path(tmpdir) / initrd_file),
                        "-append", append_arg
                    ]

                    with QEMU(test_disk_path, memory="4096", arch=arch, extra_args=extra_args) as vm:
                        vm.start(use_ovmf=use_ovmf)
                        vm.run("true", user=username, password=password)
                        vm.run("mount", user="root", keyfile=ssh_key_path)
#                        ret = vm.run(["bootc", "status"], user="root", keyfile=ssh_key_path)
#                        assert f"image: {container_ref}" in ret.stdout
        finally:
            http_server.shutdown()

        return True


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("container_ref", [
    "quay.io/centos-bootc/centos-bootc:stream10",
    "quay.io/fedora/fedora-bootc:43",
    "quay.io/centos-bootc/centos-bootc:stream9",
])
# pylint: disable=too-many-locals,duplicate-code
def test_bootc_pxe_tar_xz(keep_tmpdir, tmp_path, build_container, container_ref):
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
        # Get details about the build_container
        cmd = [
            "podman", "image", "inspect", container_tag
        ]
        subprocess.check_call(cmd)

        cmd = [
            *testutil.podman_run_common,
            "-v", f"{config_json_path}:/config.json:ro",
            "-v", f"{output_path}:/output",
            "-v", "/var/tmp/osbuild-test-store:/store",  # share the cache between builds
            "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
            build_container,
            "--type", "pxe-tar-xz",
            "--rootfs", "ext4",
            f"localhost/{container_tag}",
        ]
        subprocess.check_call(cmd)
        assert os.path.exists(output_path / "xz" / "pxe.tar.xz")
        boot_qemu_pxe(platform.machine(), output_path / "xz" / "pxe.tar.xz",
                      container_ref,
                      username, password,
                      ssh_keyfile_private_path,
                      keep_tmpdir)
