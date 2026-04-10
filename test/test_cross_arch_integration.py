import os
import re
import subprocess

import pytest

import scripts.imgtestlib as testlib

if os.getuid() != 0:
    pytest.skip(reason="need root to build the images", allow_module_level=True)


@pytest.mark.images_integration
@pytest.mark.parametrize("arch", ["ppc64le", "s390x"])
def test_build_boot_cross_arch_smoke(arch):
    if arch == "s390x":
        output = subprocess.check_output(["qemu-s390x-static", "--version"], text=True)
        m = re.match(r'(?m).*version ([0-9]+)\.([0-9]+)\.([0-9]+)', output)
        if not m:
            pytest.skip(f"failed to match version string for qemu-user ({output})")
        major, minor, patch = m.group(1, 2, 3)
        if not (int(major) >= 10 and int(minor) >= 1 and int(patch) >= 91):
            pytest.skip("need qemu-user >= 10.2 to run s390x builds")

    # very minimal as this just a smoke test
    distro = "centos-10"
    image_type = "qcow2"
    config_name = "empty"
    config_path = f"test/configs/{config_name}.json"
    subprocess.check_call(
        ["./test/scripts/build-image", f"--arch={arch}", distro, image_type, config_path])
    build_dir = os.path.join("build", testlib.gen_build_name(distro, arch, image_type, config_name))
    subprocess.check_call(
        ["./test/scripts/boot-image", build_dir, config_path])
