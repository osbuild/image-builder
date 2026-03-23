import json
import os
import pathlib

# Path to the Schutzfile relative to the root of the repository
SCHUTZFILE = str(pathlib.Path(__file__).resolve().parents[3] / "Schutzfile")
OS_RELEASE_FILE = "/etc/os-release"


def get_host_distro():
    """
    Get the host distro version based on data in the os-release file.
    The format is <distro>-<version> (e.g. fedora-41).

    Can be overridden by setting the OSBUILD_IMGTESTLIB_HOST_DISTRO env var.
    """
    # overriding this is useful for running tests locally on any distro version while still being able to reuse the
    # cached images from the CI runners
    if distro := os.environ.get("OSBUILD_IMGTESTLIB_HOST_DISTRO"):
        return distro
    osrelease = read_osrelease()
    return f"{osrelease['ID']}-{osrelease['VERSION_ID']}"


def get_osbuild_commit(distro_version):
    """
    Get the osbuild commit defined in the Schutzfile for the host distro or common.
    If not set, returns None.
    """
    with open(SCHUTZFILE, encoding="utf-8") as schutzfile:
        data = json.load(schutzfile)

    commit = data.get(distro_version, {}).get("dependencies", {}).get("osbuild", {}).get("commit", None)
    if commit is None:
        commit = data.get("common", {}).get("dependencies", {}).get("osbuild", {}).get("commit", None)
    return commit


def get_bib_ref():
    """
    Get the bootc-image-builder ref defined in the Schutzfile for the host distro.
    If not set, returns None.
    """
    with open(SCHUTZFILE, encoding="utf-8") as schutzfile:
        data = json.load(schutzfile)

    return data.get("common", {}).get("dependencies", {}).get("bootc-image-builder", {}).get("ref", None)


def rng_seed_env():
    """
    Read the rng seed from the Schutzfile and return it as a map to use as an environment variable with the appropriate
    key. Assumes the file exists and that it contains the key 'rngseed', otherwise raises an exception.
    """

    with open(SCHUTZFILE, encoding="utf-8") as schutzfile:
        data = json.load(schutzfile)

    seed = data.get("common", {}).get("rngseed")
    if seed is None:
        raise RuntimeError("'common.rngseed' not found in Schutzfile")

    return {"OSBUILD_TESTING_RNG_SEED": str(seed)}


def read_osrelease():
    """Read Operating System Information from `os-release`

    This creates a dictionary with information describing the running operating system. It reads the information from
    the path array provided as `paths`.  The first available file takes precedence. It must be formatted according to
    the rules in `os-release(5)`.
    """
    osrelease = {}

    with open(OS_RELEASE_FILE, encoding="utf8") as orf:
        for line in orf:
            line = line.strip()
            if not line:
                continue
            if line[0] == "#":
                continue
            key, value = line.split("=", 1)
            osrelease[key] = value.strip('"')

    return osrelease


def host_container_arch():
    host_arch = os.uname().machine
    return {
        "x86_64": "amd64",
        "aarch64": "arm64"
    }.get(host_arch, host_arch)


def get_ci_runner_for(arch, image_type):
    with open(SCHUTZFILE, encoding="utf-8") as schutzfile:
        data = json.load(schutzfile)

    if (runner := data.get("common", {}).get("gitlab-ci-runner-for", {}).get(arch, {}).get(image_type)) is not None:
        return runner

    return get_common_ci_runner()


def get_common_ci_runner():
    """
    CI runner for common tasks.

    Currently this is used for all gitlab CI jobs. In the future, we might switch to running build jobs on the same host
    distro as the target image, but this CI runner will still be used for generic tasks like check-build-coverage.
    """
    with open(SCHUTZFILE, encoding="utf-8") as schutzfile:
        data = json.load(schutzfile)

    if (runner := data.get("common", {}).get("gitlab-ci-runner")) is None:
        raise KeyError(f"gitlab-ci-runner not defined in {SCHUTZFILE}")

    return runner


def get_common_ci_runner_distro():
    """
    CI runner distro for common tasks.

    Returns the distro part from the value of the common.gitlab-ci-runner key in the Schutzfile.
    For example, if the value is "aws/fedora-999", this function will return "fedora-999".
    """
    return get_common_ci_runner().split("/")[1]
