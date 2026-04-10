import abc
import os
import pathlib
import platform
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
import uuid
from io import StringIO

import boto3
from botocore.exceptions import ClientError
from vmtest.util import get_free_port, wait_ssh_ready

AWS_REGION = "us-east-1"


_non_interactive_ssh = [
    "-o", "UserKnownHostsFile=/dev/null",
    "-o", "StrictHostKeyChecking=no",
    "-o", "LogLevel=ERROR",
    # ensure ssh-agent is skipped, we only ever provide keys via "-i"
    "-o", "IdentitiesOnly=yes",
]

# constants when waiting for ssh ready, this needs to be big as
# some image types (like oscap) reboot in between and relabel
# which can take some time
ssh_ready_n_retries = 30
ssh_ready_wait_sec = 10


class VM(abc.ABC):

    def __init__(self):
        self._ssh_port = None
        self._address = None

    def __del__(self):
        self.force_stop()

    @abc.abstractmethod
    def start(self):
        """
        Start the VM. This method will be called automatically if it is not called explicitly before calling run().
        """

    def _log(self, msg):
        # XXX: use a proper logger
        sys.stdout.write(msg.rstrip("\n") + "\n")

    def wait_ssh_ready(self, timeout_sec=600):
        wait_ssh_ready(self._address, self._ssh_port, sleep=1, max_wait_sec=timeout_sec)

    @abc.abstractmethod
    def force_stop(self):
        """
        Stop the VM and clean up any resources that were created when setting up and starting the machine.
        """

    def _sshpass(self, password):
        if not password:
            return []
        return ["sshpass", "-p", password]

    def _ensure_ssh(self, user, password="", keyfile=None):
        if not self.running():
            self.start()
        for i in range(ssh_ready_n_retries):
            try:
                self._run("true", user=user, password=password, keyfile=keyfile)
                return
            except Exception as e:
                print(f"ssh not ready {i+1}/{ssh_ready_n_retries}: {e}", file=sys.stderr)
            time.sleep(ssh_ready_wait_sec)
        raise RuntimeError(f"no ssh after {ssh_ready_n_retries} retries of {ssh_ready_wait_sec}s")

    def run(self, args, user, password="", keyfile=None):
        self._ensure_ssh(user, password, keyfile)
        return self._run(args, user=user, password=password, keyfile=keyfile)

    def _run(self, args, user, password="", keyfile=None):
        """
        Run a command on the VM via SSH using the provided credentials.
        """
        if isinstance(args, str):
            args = [args]
        run_cmd = shlex.join(args)
        ssh_cmd = self._sshpass(password) + [
            "ssh", "-p", str(self._ssh_port),
        ] + _non_interactive_ssh
        if keyfile:
            ssh_cmd.extend(["-i", keyfile])
        ssh_cmd.append(f"{user}@{self._address}")
        ssh_cmd.append(run_cmd)
        output = StringIO()
        with subprocess.Popen(
                ssh_cmd,
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                text=True, bufsize=1,
        ) as p:
            for out in p.stdout:
                self._log(out)
                output.write(out)
        ret = subprocess.CompletedProcess(run_cmd, p.returncode)
        ret.stdout = output.getvalue()
        # this will raise an CalledProcessError on error
        ret.check_returncode()
        return ret

    def scp(self, src, dst, user, password="", keyfile=None):
        self._ensure_ssh(user, password, keyfile)
        scp_cmd = self._sshpass(password) + [
            "scp", "-P", str(self._ssh_port),
        ] + _non_interactive_ssh
        if keyfile:
            scp_cmd.extend(["-i", keyfile])
        scp_cmd.append(src)
        scp_cmd.append(f"{user}@{self._address}:{dst}")
        subprocess.check_call(scp_cmd)

    @property
    def ssh_port(self):
        return self._ssh_port

    @abc.abstractmethod
    def running(self):
        """
        True if the VM is running.
        """

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.force_stop()


# needed as each distro puts the OVMF.fd in a different location
def find_ovmf():
    for p in [
            "/usr/share/ovmf/OVMF.fd",       # Debian
            "/usr/share/OVMF/OVMF_CODE.fd",  # Fedora
    ]:
        if os.path.exists(p):
            return p
    raise ValueError("cannot find a OVMF bios")


class QEMU(VM):
    def __init__(self, img, arch="", snapshot=True, cdrom=None, extra_args=None, memory="2048"):
        super().__init__()
        self._img = pathlib.Path(img)
        self._tmpdir = tempfile.mkdtemp(prefix="vmtest-", suffix=f"-{self._img.name}")
        self._qmp_socket = os.path.join(self._tmpdir, "qmp.socket")
        self._qemu_p = None
        self._snapshot = snapshot
        self._cdrom = cdrom
        self._extra_args = extra_args
        self._memory = memory
        self._ssh_port = None
        if not arch:
            arch = platform.machine()
        self._arch = arch

    def __del__(self):
        self.force_stop()
        shutil.rmtree(self._tmpdir)

    def _num_cores(self):
        """
        Return the number of CPU cores available on the system.
        """
        return os.cpu_count() or 1

    def _gen_qemu_cmdline(self, snapshot, use_ovmf):
        virtio_scsi_hd = [
            "-device", "virtio-scsi-pci,id=scsi",
            "-device", "scsi-hd,drive=disk0",
        ]
        virtio_net_device = "virtio-net-pci"
        if self._arch in ("arm64", "aarch64"):
            qemu_cmdline = [
                "qemu-system-aarch64",
                "-machine", "virt",
                "-cpu", "cortex-a57",
                "-accel", "tcg,thread=multi",
                "-smp", str(self._num_cores()),
                "-bios", "/usr/share/AAVMF/AAVMF_CODE.fd",
            ] + virtio_scsi_hd
        elif self._arch in ("amd64", "x86_64"):
            qemu_cmdline = [
                "qemu-system-x86_64",
                "-M", "q35,accel=kvm",
                # RHEL 10 requires x86_64-v3 pass it to avoid "illegal instruction", but
                # do not use "host" because some modern features are not supported by some
                # distributions when running locally on very recent laptops.
                "-cpu", "Haswell-v4",
            ] + virtio_scsi_hd
            if use_ovmf:
                qemu_cmdline.extend(["-bios", find_ovmf()])
        elif self._arch in ("ppc64le", "ppc64"):
            qemu_cmdline = [
                "qemu-system-ppc64",
                "-machine", "pseries",
                "-smp", str(self._num_cores()),
            ] + virtio_scsi_hd
        elif self._arch == "s390x":
            qemu_cmdline = [
                "qemu-system-s390x",
                "-machine", "s390-ccw-virtio",
                "-smp", str(self._num_cores()),
                # sepcial disk setup
                "-device", "virtio-blk,drive=disk0,bootindex=1",
            ]
            virtio_net_device = "virtio-net-ccw"
        else:
            raise ValueError(f"unsupported architecture {self._arch}")

        if self._img.suffix == ".qcow2":
            img_format = "qcow2"
        elif self._img.suffix in (".img", ".raw"):
            img_format = "raw"
        else:
            raise ValueError(f"Unsupported image extension: {self._img}. Must be .qcow2 or .img")

        # common part
        qemu_cmdline += [
            "-m", str(self._memory),
            "-serial", "stdio",
            "-monitor", "none",
            # make sure SSH key generation during boot does not block due to lack of entropy
            "-object", "rng-random,filename=/dev/urandom,id=rng0",
            "-device", "virtio-rng-pci,rng=rng0",
            # network
            "-device", f"{virtio_net_device},netdev=net.0,id=net.0",
            "-netdev", f"user,id=net.0,hostfwd=tcp::{self._ssh_port}-:22",
            "-qmp", f"unix:{self._qmp_socket},server,nowait",
            # boot
            "-drive", f"file={self._img},if=none,id=disk0,cache=unsafe,format={img_format}",
        ]
        if not os.environ.get("OSBUILD_TEST_QEMU_GUI"):
            qemu_cmdline.append("-nographic")
        if self._cdrom:
            qemu_cmdline.extend(["-cdrom", str(self._cdrom)])
        if snapshot:
            qemu_cmdline.append("-snapshot")
        if self._extra_args:
            qemu_cmdline.extend(str(arg) for arg in self._extra_args)

        print("QEMU: " + " ".join(qemu_cmdline))
        return qemu_cmdline

    # XXX: move args to init() so that __enter__ can use them?
    def start(self, wait_event="ssh", snapshot=True, use_ovmf=False, timeout_sec=120):
        if self.running():
            return
        self._ssh_port = get_free_port()
        self._address = "localhost"

        # XXX: use systemd-run to ensure cleanup?
        # pylint: disable=consider-using-with
        self._qemu_p = subprocess.Popen(
            self._gen_qemu_cmdline(snapshot, use_ovmf),
            stdout=sys.stdout,
            stderr=sys.stderr,
        )
        # XXX: also check that qemu is working and did not crash
        ev = wait_event.split(":")
        if ev == ["ssh"]:
            self.wait_ssh_ready(timeout_sec=timeout_sec)
            self._log(f"vm ready at port {self._ssh_port}")
        elif ev[0] == "qmp":
            qmp_event = ev[1]
            self.wait_qmp_event(qmp_event, timeout_sec=timeout_sec)
            self._log(f"qmp event {qmp_event}")
        else:
            raise ValueError(f"unsupported wait_event {wait_event}")

    def _wait_qmp_socket(self, timeout_sec):
        for _ in range(timeout_sec):
            if os.path.exists(self._qmp_socket):
                return True
            time.sleep(1)
        raise TimeoutError(f"no {self._qmp_socket} after {timeout_sec} seconds")

    def wait_qmp_event(self, qmp_event, timeout_sec=120):
        # import lazy to avoid requiring it for all operations
        import qmp  # pylint: disable=import-outside-toplevel
        self._wait_qmp_socket(30)
        mon = qmp.QEMUMonitorProtocol(os.fspath(self._qmp_socket))
        mon.connect()
        start = time.monotonic()
        while True:
            # NOTE: Using wait=1.0 with pull_event only works once.
            #       So we do the timeout ourselves.
            time.sleep(1)
            event = mon.pull_event(wait=False)
            if event is not None:
                self._log(f"DEBUG: got event {event}")
                if event["event"] == qmp_event:
                    return

            if time.monotonic() > start + timeout_sec:
                raise TimeoutError(f"no {qmp_event} event after {timeout_sec} seconds")

    def force_stop(self):
        if self._qemu_p:
            self._qemu_p.kill()
            self._qemu_p.wait()
            self._qemu_p = None
            self._address = None
            self._ssh_port = None

    def running(self):
        return self._qemu_p is not None


class AWS(VM):

    _instance_type = "t3.medium"  # set based on architecture when we add arm tests

    def __init__(self, ami_id):
        super().__init__()
        self._ssh_port = 22
        self._ami_id = ami_id
        self._ec2_instance = None
        self._ec2_security_group = None
        self._ec2_resource = boto3.resource("ec2", region_name=AWS_REGION)

    def start(self):
        if self.running():
            return
        sec_group_ids = []
        if not self._ec2_security_group:
            self._set_ssh_security_group()
        sec_group_ids = [self._ec2_security_group.id]
        try:
            self._log(f"Creating ec2 instance from {self._ami_id}")
            instances = self._ec2_resource.create_instances(
                ImageId=self._ami_id,
                InstanceType=self._instance_type,
                SecurityGroupIds=sec_group_ids,
                MinCount=1, MaxCount=1
            )
            self._ec2_instance = instances[0]
            self._log(f"Waiting for instance {self._ec2_instance.id} to start")
            self._ec2_instance.wait_until_running()
            self._ec2_instance.reload()  # make sure the instance info is up to date
            self._address = self._ec2_instance.public_ip_address
            self._log(f"Instance is running at {self._address}")
            self.wait_ssh_ready()
            self._log("SSH is ready")
        except ClientError as err:
            err_code = err.response["Error"]["Code"]
            err_msg = err.response["Error"]["Message"]
            self._log(f"Couldn't create instance with image {self._ami_id} and type {self._instance_type}.")
            self._log(f"Error {err_code}: {err_msg}")
            raise

    def _set_ssh_security_group(self):
        group_name = f"bootc-image-builder-test-{str(uuid.uuid4())}"
        group_desc = "bootc-image-builder test security group: SSH rule"
        try:
            self._log(f"Creating security group {group_name}")
            self._ec2_security_group = self._ec2_resource.create_security_group(GroupName=group_name,
                                                                                Description=group_desc)
            ip_permissions = [
                {
                    "IpProtocol": "tcp",
                    "FromPort": self._ssh_port,
                    "ToPort": self._ssh_port,
                    "IpRanges": [{"CidrIp": "0.0.0.0/0"}],
                }
            ]
            self._log(f"Authorizing inbound rule for {group_name} ({self._ec2_security_group})")
            self._ec2_security_group.authorize_ingress(IpPermissions=ip_permissions)
            self._log("Security group created")
        except ClientError as err:
            err_code = err.response["Error"]["Code"]
            err_msg = err.response["Error"]["Message"]
            self._log(f"Couldn't create security group {group_name} or authorize inbound rule.")
            self._log(f"Error {err_code}: {err_msg}")
            raise

    def force_stop(self):
        if self._ec2_instance:
            self._log(f"Terminating instance {self._ec2_instance.id}")
            try:
                self._ec2_instance.terminate()
                self._ec2_instance.wait_until_terminated()
                self._ec2_instance = None
                self._address = None
            except ClientError as err:
                err_code = err.response["Error"]["Code"]
                err_msg = err.response["Error"]["Message"]
                self._log(f"Couldn't terminate instance {self._ec2_instance.id}.")
                self._log(f"Error {err_code}: {err_msg}")
        else:
            self._log("No EC2 instance defined. Skipping termination.")

        if self._ec2_security_group:
            self._log(f"Deleting security group {self._ec2_security_group.id}")
            try:
                self._ec2_security_group.delete()
                self._ec2_security_group = None
            except ClientError as err:
                err_code = err.response["Error"]["Code"]
                err_msg = err.response["Error"]["Message"]
                self._log(f"Couldn't delete security group {self._ec2_security_group.id}.")
                self._log(f"Error {err_code}: {err_msg}")
        else:
            self._log("No security group defined. Skipping deletion.")

    def running(self):
        return self._ec2_instance is not None
