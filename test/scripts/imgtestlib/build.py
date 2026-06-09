import json
import os
from typing import Dict

from .gitlab import log_section
from .run import runcmd
from .testenv import get_host_distro, get_osbuild_commit, rng_seed_env

BUILD_LOG_PATH = "./logs"


@log_section("Building image")
def build_image(distro, arch, image_type, config_path):
    with open(config_path, "r", encoding="utf-8") as config_file:
        config = json.load(config_file)

    config_name = config["name"]

    # print the config for logging
    print(json.dumps(config, indent=2))

    runcmd(["go", "build", "-o", "./bin/build", "./cmd/build"])

    cmd = ["sudo", "-E", "./bin/build", "--output", "./build", "--checkpoints", "build",
           "--distro", distro, "--arch", arch, "--type", image_type, "--config", config_path]
    stdout, stderr = runcmd(cmd, extra_env=rng_seed_env())

    build_name = gen_build_name(distro, arch, image_type, config_name)
    save_logs(build_name, stdout, stderr)

    # Build artifacts are owned by root. Make them world accessible.
    runcmd(["sudo", "chmod", "a+rwX", "-R", "./build"])

    build_dir = os.path.join("build", gen_build_name(distro, arch, image_type, config_name))
    manifest_path = os.path.join(build_dir, "manifest.json")
    with open(manifest_path, "r", encoding="utf-8") as manifest_fp:
        manifest_data = json.load(manifest_fp)
    manifest_id = get_manifest_id(manifest_data)

    osbuild_ver, _ = runcmd(["osbuild", "--version"])

    distro_version = get_host_distro()
    osbuild_commit = get_osbuild_commit(distro_version)
    if osbuild_commit is None:
        osbuild_commit = "RELEASE"

    build_info = {
        "distro": distro,
        "arch": arch,
        "image-type": image_type,
        "config": config_name,
        "manifest-checksum": manifest_id,
        "osbuild-version": osbuild_ver.decode().strip(),
        "osbuild-commit": osbuild_commit,
        "commit": os.environ.get("CI_COMMIT_SHA", "N/A"),
        "runner-distro": distro_version,
    }
    write_build_info(build_dir, build_info)


def read_build_info(build_path: str) -> Dict:
    """
    Read the info.json file from the build directory and return the data as a dictionary.
    """
    info_file_path = os.path.join(build_path, "info.json")
    with open(info_file_path, encoding="utf-8") as info_fp:
        return json.load(info_fp)


def write_build_info(build_path: str, data: Dict):
    """
    Write the data to the info.json file in the build directory.
    """
    info_file_path = os.path.join(build_path, "info.json")
    with open(info_file_path, "w", encoding="utf-8") as info_fp:
        json.dump(data, info_fp, indent=2)


def get_manifest_id(manifest_data):
    md = json.dumps(manifest_data).encode()
    out, _ = runcmd(["osbuild", "--inspect", "-"], stdin=md)
    data = json.loads(out)
    # last stage ID depends on all previous stage IDs, so we can use it as a manifest ID
    return data["pipelines"][-1]["stages"][-1]["id"]


def gen_build_name(distro, arch, image_type, config_name):
    return f"{_u(distro)}-{_u(arch)}-{_u(image_type)}-{_u(config_name)}"


def _u(s):
    return s.replace("-", "_")


def save_logs(build_name, out, err):
    """
    Save stdout and stderr output for a job to the BUILD_LOG_PATH.
    """
    os.makedirs(BUILD_LOG_PATH, exist_ok=True)
    with open(os.path.join(BUILD_LOG_PATH, f"{build_name}.out"), mode="w", encoding="utf-8") as log:
        log.write(out.decode())
    with open(os.path.join(BUILD_LOG_PATH, f"{build_name}.err"), mode="w", encoding="utf-8") as log:
        log.write(err.decode())
