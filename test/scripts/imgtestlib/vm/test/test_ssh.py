import os
import subprocess
import textwrap
import time
from unittest.mock import call, patch

import pytest

import vmtest.vm
from vmtest.vm import VM
from vmtest.util import get_free_port


class MockVM(VM):
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
    with pytest.raises(subprocess.CalledProcessError) as e:
        vm.run("bad-cmd", user="user1", keyfile="keyfile1")
    assert e.value.returncode == 42
    assert e.value.stdout.endswith("bad-output\n")


@patch("time.sleep")
def test_ssh_calls_retries(mocked_sleep, tmp_path, monkeypatch, capsys):
    monkeypatch.setenv("PATH", os.fspath(tmp_path), prepend=os.pathsep)
    make_fake_ssh(tmp_path, "echo ssh-very-sad; exit 21")
    vm = MockVM()
    with pytest.raises(RuntimeError) as e:
        res = vm.run(["cmd1", "arg1", "arg2"], user="user1", keyfile="keyfile1")
    assert str(e.value) == "no ssh after 30 retries of 10s"
    assert mocked_sleep.call_args_list == 30 * [ call(10) ]
    assert capsys.readouterr().err == "\n".join([
        f"ssh not ready {i+1}/30: Command 'true' returned non-zero exit status 21."
        for i in range(30)
    ]) + "\n"

def test_wait_ssh_ready_timeout():
    vm = MockVM()
    vm.set_ssh("localhost", get_free_port())
    with pytest.raises(ConnectionRefusedError) as e:
        vm.wait_ssh_ready(timeout_sec=3)
    assert "after 3s" in str(e.value)
