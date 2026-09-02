import platform

import imgtestlib as testlib
import pytest

import testutil
# pylint: disable=unused-import
from test_build_disk import (ImageBuildResult, assert_disk_image_boots,
                             build_container_fixture, gpg_conf_fixture,
                             image_type_fixture, registry_conf_fixture,
                             shared_tmpdir_fixture)
from testcases import gen_testcases


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("ami-cross"), indirect=["image_type"])
def test_image_boots_cross(image_type: ImageBuildResult, force_aws_upload):
    if not testutil.write_aws_creds("/dev/null"):
        if force_aws_upload:
            # upload forced but credentials aren't set
            raise RuntimeError("AWS credentials not available")
        pytest.skip("AWS credentials not available (upload not forced)")

    # check that upload progress is in the output log. Uploads looks like:
    # 4.30 GiB / 10.00 GiB [------------>____________] 43.02% 58.04 MiB p/s
    assert "] 100.00%" in image_type.bib_output

    arch = {
        "amd64": "x86_64",
        "arm64": "aarch64",
    }[image_type.img_arch]
    with testlib.vm.AWS(image_type.metadata["ami_id"], arch) as test_vm:
        test_vm.run("true", user=image_type.username, password=image_type.password)
        ret = test_vm.run(["echo", "hello"], user=image_type.username, password=image_type.password)
        assert "hello" in ret.stdout


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("ami-cross"), indirect=["image_type"])
def test_image_builds_cross(image_type: ImageBuildResult):
    assert image_type.img_path.exists()
