import os
import subprocess

import pytest

# put common podman run args in once place
podman_run = ["podman", "run", "--rm", "--privileged"]


# only test a subset here to avoid overly long runtimes
@pytest.mark.parametrize("arch", ["aarch64", "ppc64le", "riscv64", "s390x"])
def test_build_cross_builds(tmp_path, build_container, arch):
    # this is only here to speed up builds by sharing downloaded stuff
    # when this is run locally (we could cache via GH action though)
    os.makedirs("/var/cache/image-builder/store", exist_ok=True)
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call(podman_run + [
        "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
        "-v", "/var/cache/image-builder/store:/var/cache/image-builder/store",
        "-v", f"{output_dir}:/output",
        build_container,
        "build",
        "--progress=verbose",
        "--output-dir=/output",
        "container",
        "--distro", "fedora-41",
        # selecting a foreign arch here automatically triggers a cross-build
        f"--arch={arch}",
    ], text=True)
    assert os.path.exists(output_dir / f"fedora-41-container-{arch}.tar")
