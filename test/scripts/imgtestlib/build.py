import json
import os
import tempfile
from typing import Dict, List

from .gitlab import log_section
from .run import runcmd, runcmd_nc
from .testenv import get_host_distro, get_osbuild_commit, rng_seed_env


def config_to_cli_args(config: dict) -> List[str]:
    args: List[str] = []

    blueprint = config.get("blueprint", {})
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as bp_file:
        json.dump(blueprint, bp_file)
    args.append(f"--blueprint={bp_file.name}")

    options = config.get("options", {})
    ostree = options.get("ostree", {})
    if ref := ostree.get("ref"):
        args.append(f"--ostree-ref={ref}")
    if url := ostree.get("url"):
        args.append(f"--ostree-url={url}")
    if parent := ostree.get("parent"):
        args.append(f"--ostree-parent={parent}")

    bootc = options.get("bootc", {})
    if payload_ref := bootc.get("installer_payload_ref"):
        args.append(f"--bootc-installer-payload-ref={payload_ref}")
    if bootc.get("use_remote_container_source"):
        args.append("--bootc-pull-container")

    if size := options.get("size"):
        args.append(f"--image-size={size}")

    for repo in config.get("custom_repos", []):
        for url in repo.get("baseurls", []):
            args.append(f"--extra-repo={url}")

    return args


@log_section("Building image")
def build_image(distro, arch, image_type, config_path):
    with open(config_path, "r", encoding="utf-8") as config_file:
        config = json.load(config_file)

    config_name = config["name"]
    build_name = gen_build_name(distro, arch, image_type, config_name)
    build_dir = os.path.join("build", build_name)

    print(f"👷 Building image {distro}/{image_type} using config {config_path}")

    # print the config for logging
    print(json.dumps(config, indent=2))

    runcmd(["go", "build", "-o", "./bin/image-builder", "./cmd/image-builder"])
    seed = rng_seed_env()["OSBUILD_TESTING_RNG_SEED"]
    cmd = [
        "sudo", "-E", "./bin/image-builder", "build", image_type,
        "--distro", distro,
        "--arch", arch,
        "--force-repo-dir", "test/data/repositories",
        "--output-dir", build_dir,
        "--output-name", build_name,
        "--with-manifest",
        "--ignore-warnings",
        "--seed", str(seed),
    ]
    cmd.extend(config_to_cli_args(config))
    runcmd_nc(cmd)

    print("✅ Build finished!!")

    # Build artifacts are owned by root. Make them world accessible.
    runcmd(["sudo", "chmod", "a+rwX", "-R", "./build"])

    osbuild_manifest = os.path.join(build_dir, f"{build_name}.osbuild-manifest.json")
    manifest_path = os.path.join(build_dir, "manifest.json")
    if os.path.exists(osbuild_manifest) and not os.path.exists(manifest_path):
        os.symlink(f"{build_name}.osbuild-manifest.json", manifest_path)

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
