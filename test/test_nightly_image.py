import os
from pathlib import Path
import subprocess
import textwrap

import pytest
import yaml


WORKFLOW = Path(__file__).resolve().parents[1] / ".gitlab/nightly-images.yml"


def run_build(tmp_path, **variables):
    """Run the job's build script with a fake sudo, recording commands in calls."""
    sudo = tmp_path / "sudo"
    sudo.write_text(textwrap.dedent("""\
        #!/bin/bash
        echo "$*" >> calls
        case "$1 $2" in
            "subscription-manager identity") exit "$IDENTITY_STATUS" ;;
            "subscription-manager register") exit "$REGISTER_STATUS" ;;
            "subscription-manager unregister") exit "$UNREGISTER_STATUS" ;;
            "./bin/image-builder build")
                if [ "$BUILD_STATUS" != 0 ]; then exit "$BUILD_STATUS"; fi
                if [ "$MISSING_OUTPUT" = true ]; then exit 0; fi
                name="$DISTRO-$IMAGE_TYPE-$ARCH"
                mkdir "$name"
                echo "test image" > "$name/$name.qcow2"
                ;;
            "test -s") exec test -s "$3" ;;
            *) exit 99 ;;
        esac
        """), encoding="utf-8")
    sudo.chmod(0o755)

    with WORKFLOW.open(encoding="utf-8") as workflow:
        job = yaml.safe_load(workflow)["nightly-image"]
    env = {
        **os.environ,
        **job["parallel"]["matrix"][0],
        "PATH": f"{tmp_path}:{os.environ['PATH']}",
        "SUBSCRIPTION_ORG": "test-org",
        "SUBSCRIPTION_ACTIVATION_KEY": "test-key",
        "IDENTITY_STATUS": "1",
        "REGISTER_STATUS": "0",
        "UNREGISTER_STATUS": "0",
        "BUILD_STATUS": "0",
        "MISSING_OUTPUT": "false",
        **variables,
    }
    result = subprocess.run(["bash", "-c", job["script"][-1]], cwd=tmp_path, env=env,
                            capture_output=True, text=True, check=False)
    return result, (tmp_path / "calls").read_text(encoding="utf-8").splitlines()


def test_success_uses_cli_defaults(tmp_path):
    result, calls = run_build(tmp_path)
    assert result.returncode == 0, result.stderr
    assert calls == [
        "subscription-manager identity",
        "subscription-manager register --org=test-org --activationkey=test-key",
        "./bin/image-builder build qcow2 --distro rhel-10.3 --arch x86_64",
        "test -s rhel-10.3-qcow2-x86_64/rhel-10.3-qcow2-x86_64.qcow2",
        "subscription-manager unregister",
    ]


@pytest.mark.parametrize("variables,status", [
    pytest.param({"BUILD_STATUS": "7"}, 7, id="build-fails"),
    pytest.param({"REGISTER_STATUS": "3"}, 3, id="registration-fails"),
    pytest.param({"MISSING_OUTPUT": "true"}, 1, id="image-missing"),
    pytest.param({"UNREGISTER_STATUS": "1"}, 1, id="cleanup-fails"),
    pytest.param({"BUILD_STATUS": "7", "UNREGISTER_STATUS": "1"}, 7, id="preserve-build-error"),
    pytest.param({"REGISTER_STATUS": "3", "UNREGISTER_STATUS": "1"}, 3, id="preserve-registration-error"),
])
def test_failures_unregister_and_preserve_status(tmp_path, variables, status):
    result, calls = run_build(tmp_path, **variables)
    assert result.returncode == status
    assert calls[-1] == "subscription-manager unregister"
    if "REGISTER_STATUS" in variables:
        assert not any(call.startswith("./bin/image-builder") for call in calls)


@pytest.mark.parametrize("build_status", ["0", "7"])
def test_existing_registration_is_preserved(tmp_path, build_status):
    result, calls = run_build(tmp_path, IDENTITY_STATUS="0", BUILD_STATUS=build_status)
    assert result.returncode == int(build_status), result.stderr
    assert [call for call in calls if call.startswith("subscription-manager")] == ["subscription-manager identity"]


@pytest.mark.parametrize("credential", ["SUBSCRIPTION_ORG", "SUBSCRIPTION_ACTIVATION_KEY"])
def test_missing_credentials_never_builds(tmp_path, credential):
    result, calls = run_build(tmp_path, **{credential: ""})
    assert result.returncode != 0
    assert calls == ["subscription-manager identity"]


def test_non_rhel_build_does_not_manage_subscriptions(tmp_path):
    result, calls = run_build(tmp_path, DISTRO="centos-10", SUBSCRIPTION_ORG="", SUBSCRIPTION_ACTIVATION_KEY="")
    assert result.returncode == 0, result.stderr
    assert not any(call.startswith("subscription-manager") for call in calls)
