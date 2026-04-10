import os
import platform
import subprocess

import pytest

import scripts.imgtestlib as testlib

if os.getuid() != 0:
    pytest.skip(reason="need root to build the images", allow_module_level=True)


def _test_cases():
    # XXX: make testcase a data class
    all_test_cases = subprocess.check_output([
        "go", "run", "-buildvcs=false", "./cmd/gen-manifests",
        # we may consider cross arch tests here at some point but for now
        # assume we run native
        "-arches", platform.uname().machine,
        "-dry-run",
    ], text=True).strip().split("\n")
    boot_tests = set()
    for tcase in all_test_cases:
        _, arch, image_type, _ = tcase.split(",")
        if not (image_type in testlib.CAN_BOOT_TEST["*"] or image_type in testlib.CAN_BOOT_TEST.get(arch, [])):
            continue
        # XXX: we need to filter further here, i.e. not all qemu tests can be run currently, e.g.
        # if sshd is missing (see boot_image.ensure_can_run_qemu_test - however this needs
        # a build/ dir so we cannot filter right now without also building all manifests. This
        # should instead be refactored so that we have a testcase class and we can filter more
        # flexible on that and exclude e.g. all image types with "minmal" and "minimal-pkgs"
        # as their config_name
        boot_tests.add(tcase)
    return {
        "build_only": set(all_test_cases)-boot_tests,
        "build_and_boot": boot_tests,
    }


@pytest.mark.images_integration
@pytest.mark.parametrize("distro,arch,image_type,config_name",
                         [tcase.split(",") for tcase in _test_cases()["build_only"]])
def test_build_only(distro, arch, image_type, config_name):
    config_path = f"test/configs/{config_name}.json"
    subprocess.check_call(
        ["./test/scripts/build-image", distro, image_type, config_path])
    build_dir = os.path.join("build", testlib.gen_build_name(distro, arch, image_type, config_name))
    subprocess.check_call(
        ["./test/scripts/boot-image", build_dir, config_path])


@pytest.mark.images_integration
@pytest.mark.parametrize("distro,arch,image_type,config_name",
                         [tcase.split(",") for tcase in _test_cases()["build_and_boot"]])
def test_build_and_boot(distro, arch, image_type, config_name):
    config_path = f"test/configs/{config_name}.json"
    subprocess.check_call(
        ["./test/scripts/build-image", distro, image_type, config_path])
    build_dir = os.path.join("build", testlib.gen_build_name(distro, arch, image_type, config_name))
    subprocess.check_call(
        ["./test/scripts/boot-image", build_dir, config_path])
