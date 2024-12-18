import os
import platform
import random
import string
import subprocess
import textwrap
from contextlib import contextmanager

import pytest


# XXX: copied from bib
@pytest.fixture(name="build_container", scope="session")
def build_container_fixture():
    """Build a container from the Containerfile and returns the name"""

    container_tag = "image-builder-cli-test"
    subprocess.check_call([
        "podman", "build",
        "-f", "Containerfile",
        "-t", container_tag,
    ])
    return container_tag
