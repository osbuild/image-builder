import platform

import pytest

# pylint: disable=unused-import
from test_build_disk import (ImageBuildResult, assert_disk_image_boots,
                             build_container_fixture, gpg_conf_fixture,
                             image_type_fixture, registry_conf_fixture,
                             shared_tmpdir_fixture)
from testcases import gen_testcases


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("qemu-cross"), indirect=["image_type"])
@pytest.mark.skip("disabled while rewriting to boot in AWS")
def test_image_boots_cross(image_type):
    assert_disk_image_boots(image_type)


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("qemu-cross"), indirect=["image_type"])
def test_image_builds_cross(image_type: ImageBuildResult):
    assert image_type.img_path.exists()
