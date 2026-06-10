import contextlib
import os
import uuid
from datetime import datetime


def running_in_gitlab():
    """
    Returns true if running in GitLab CI.
    """
    return os.environ.get("GITLAB_CI")


def print_section_start(name: str, msg: str = ""):
    """
    Prints a section header with a timestamp for logging output during tests.
    If running in GitLab CI, it also creates a collapsible section.

    https://docs.gitlab.com/ci/jobs/job_logs/#custom-collapsible-sections
    """
    now = datetime.now()
    if running_in_gitlab():
        print(f"\033[0Ksection_start:{int(now.timestamp())}:{name}[collapsed=true]\r\033[0K{msg}")
        return

    # custom line for non CI runs
    isonow = now.isoformat()
    print(f":: [{isonow}] {msg} ({name})")


def print_section_end(name: str):
    now = datetime.now()
    if running_in_gitlab():
        print(f"\033[0Ksection_end:{int(now.timestamp())}:{name}\r\033[0K")
        return

    # custom line for non CI runs
    isonow = now.isoformat()
    print(f":: [{isonow}] Done ({name})")


class log_section(contextlib.ContextDecorator):

    def __init__(self, message):
        self._id = ""
        self._message = message

    def __enter__(self):
        self._id = str(uuid.uuid4())
        print_section_start(self._id, self._message)

    def __exit__(self, *_):
        print_section_end(self._id)
