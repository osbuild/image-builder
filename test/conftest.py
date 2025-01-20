import subprocess

import pytest


def pytest_configure(config):
    config.addinivalue_line(
        "markers", "images_integration"
    )


# XXX: copied from bib
@pytest.fixture(name="build_container", scope="session")
def build_container_fixture():
    """Build a container from the Containerfile and returns the name"""

    container_tag = "image-builder-test"
    subprocess.check_call([
        "podman", "build",
        "-f", "Containerfile",
        "-t", container_tag,
    ])
    return container_tag
