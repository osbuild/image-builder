import random
import string
import subprocess
import textwrap

import pytest


def pytest_configure(config):
    config.addinivalue_line(
        "markers", "images_integration"
    )
