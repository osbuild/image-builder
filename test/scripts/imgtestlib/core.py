import argparse
import json
import os
import pathlib
import sys
from glob import glob
from typing import Dict

from .build import get_manifest_id
from .cache import dl_build_info, gen_build_info_dir_path_prefix, touch_s3
from .gitlab import log_section
from .run import runcmd
from .testenv import get_bib_ref, host_container_arch, rng_seed_env

TEST_CACHE_ROOT = ".cache/osbuild-images"
CONFIGS_PATH = "./test/configs"
CONFIG_LIST = "./test/config-list.json"

BIB_TYPES = [
    "iot-bootable-container"
]


# image types that can be boot tested
# Keep in sync with test/scripts/boot-image which has the same checks again
CAN_BOOT_TEST = {
    "*": [
        "ami",
        "ec2",
        "ec2-ha",
        "ec2-sap",
        "edge-ami",
        "iot-bootable-container",
        "vhd",
        "cloud-ec2",
    ],
    "x86_64": [
        # "image-installer", "minimal-installer", "network-installer",
        # "qcow2", "generic-qcow2", "cloud-qcow2",
        "wsl", "generic-wsl",
    ]
}


def list_images(distros=None, arches=None, images=None):
    distros_arg = "*"
    if distros:
        distros_arg = ",".join(distros)
    arches_arg = "*"
    if arches:
        arches_arg = ",".join(arches)
    images_arg = "*"
    if images:
        images_arg = ",".join(images)
    env = {"GOPROXY": "https://proxy.golang.org,direct"}
    out, _ = runcmd(["go", "run", "./cmd/list-images", "--json",
                     "--distros", distros_arg, "--arches", arches_arg, "--types", images_arg], extra_env=env)
    return json.loads(out)


def check_config_names():
    """
    Check that all the configs we rely on have names that match the file name, otherwise the test skipping and pipeline
    generation will be incorrect.
    """
    bad_configs = []
    for file in pathlib.Path(CONFIGS_PATH).glob("*.json"):
        config = json.loads(file.read_text())
        if file.stem != config["name"]:
            bad_configs.append(str(file))

    if bad_configs:
        print("☠️ ERROR: The following test configs have names that don't match their filenames.")
        print("\n".join(bad_configs))
        print("This will produce incorrect test generation and results.")
        print("Aborting.")
        sys.exit(1)


def gen_manifests(outputdir, config_list=None, distros=None, arches=None, images=None,
                  commits=False, flatpaks=False, skip_no_config=False):
    # pylint: disable=too-many-arguments,too-many-positional-arguments
    cmd = ["go", "run", "./cmd/gen-manifests",
           "--cache", os.path.join(TEST_CACHE_ROOT, "rpmmd"),
           "--output", outputdir,
           "--workers", "100"]
    if config_list:
        cmd.extend(["--config-list", config_list])
    if distros:
        cmd.extend(["--distros", ",".join(distros)])
    if arches:
        cmd.extend(["--arches", ",".join(arches)])
    if images:
        cmd.extend(["--types", ",".join(images)])
    if commits:
        cmd.append("--commits")
    if flatpaks:
        cmd.append("--flatpaks")
    if skip_no_config:
        cmd.append("--skip-noconfig")
    env = rng_seed_env()
    env["GOPROXY"] = "https://proxy.golang.org,direct"
    print("⌨️" + " ".join(cmd) + " ENV: " + str(env))
    _, stderr = runcmd(cmd, extra_env=env)
    return stderr


def read_manifests(path):
    """
    Read all manifests in the given path, calculate their IDs, and return a dictionary mapping each filename to the data
    and its ID.
    """
    print(f"📖 Reading manifests in {path}")
    manifests = {}
    for manifest_fname in os.listdir(path):
        manifest_path = os.path.join(path, manifest_fname)
        with open(manifest_path, encoding="utf-8") as manifest_file:
            manifest_data = json.load(manifest_file)
        manifests[manifest_fname] = {
            "data": manifest_data,
            "id": get_manifest_id(manifest_data["manifest"]),
        }
    print("✅ Done")
    return manifests


# pylint: disable=too-many-branches
def check_for_build(manifest_fname, build_request, manifest_data, build_info_dir, errors):
    """
    Checks if a manifest was built (and optionally booted) successfully.

    This function returns True if the image needs to be built.
    """
    build_info_path = os.path.join(build_info_dir, "info.json")
    # rebuild if matching build info is not found
    if not os.path.exists(build_info_path):
        print(f"🟥 Build info not found: {build_info_path}")
        print("  Adding config to build pipeline.")
        return True

    try:
        with open(build_info_path, encoding="utf-8") as build_info_fp:
            dl_config = json.load(build_info_fp)
    except json.JSONDecodeError as jd:
        errors.append((
            f"failed to parse {build_info_path}\n"
            f"{jd.msg}\n"
        ))
        print("  Adding config to build pipeline.")
        return True

    commit = dl_config["commit"]
    pr = dl_config.get("pr")
    url = f"https://github.com/osbuild/images/commit/{commit}"
    print(f"🖼️ Manifest {manifest_fname} was successfully built in commit {commit}\n  {url}")
    if "gh-readonly-queue" in pr:
        print(f"  This commit was on a merge queue: {pr}")
    elif pr:
        print(f"  PR-{pr}: https://github.com/osbuild/images/pull/{pr}")
    else:
        print("  No PR/branch info available")

    image_type = dl_config["image-type"]
    if not can_boot_test(manifest_fname, manifest_data, build_request["image-type"], build_request["arch"],
                         build_request["distro"], build_request["config"].get("blueprint", {})):
        print(f"  Boot testing for {image_type} is not yet supported")
        return False

    # boot testing supported: check if it's been tested, otherwise queue it for rebuild and boot
    if dl_config.get("boot-success", False):
        print("  This image was successfully boot tested")

        # check if it's a BIB type and compare image IDs
        if image_type in BIB_TYPES:
            # Successful boot tests with BIB add a file to the directory as bib-<image ID>. Collect them and compare.
            bib_ids = glob("bib-*", root_dir=build_info_dir)
            # add the _old_ bib ID that we used to keep in the info.json
            config_bib_id = dl_config.get("bib-id")
            if config_bib_id:
                bib_ids.append(f"bib-{config_bib_id}")
            bib_ref = get_bib_ref()
            current_id = skopeo_inspect_id(f"docker://{bib_ref}", host_container_arch())
            if f"bib-{current_id}" not in bib_ids:
                if bib_ids:
                    print("  Container disk image was built with the following bootc-image-builder images:")
                    print("    - " + "\n    -".join(bib_ids))
                else:
                    print("  No bib IDs found.")
                print(f"  Testing {current_id}")
                print("  Adding config to build pipeline.")
                return True

        return False
    print("  Boot test success not found.")

    # default to build
    print("  Adding config to build pipeline.")
    return True


@log_section("Filtering build configurations")
def filter_builds(manifests, distro=None, arch=None, skip_ostree_pull=True):
    """
    Returns a list of build requests for the manifests that have no matching config in the test build cache.
    """
    print(f"⚙️ Filtering {len(manifests)} build configurations")
    dl_root_path = os.path.join(TEST_CACHE_ROOT, "s3configs", "builds")
    dl_path = os.path.join(dl_root_path, gen_build_info_dir_path_prefix(distro, arch))
    os.makedirs(dl_path, exist_ok=True)
    build_requests = []

    out, dl_ok = dl_build_info(dl_path, distro, arch)
    # continue even if the dl failed; will build all configs
    if dl_ok:
        # print output which includes list of downloaded files for CI job log
        print(out)

    errors: list[str] = []
    for manifest_fname, data in manifests.items():
        manifest_id = data["id"]
        data = data.get("data")
        build_request = data["build-request"]
        distro = build_request["distro"]
        arch = build_request["arch"]
        image_type = build_request["image-type"]
        config = build_request["config"]
        config_name = config["name"]
        options = config.get("options", {})

        # check if the config specifies an ostree URL and skip it if requested
        if skip_ostree_pull and options.get("ostree", {}).get("url"):
            print(f"🦘 Skipping {distro}/{arch}/{image_type}/{config_name} (ostree dependency)")
            continue

        # add manifest id to build request
        build_request["manifest-checksum"] = manifest_id

        # check if the hash_fname exists in the synced directory
        build_info_dir = os.path.join(
            dl_root_path,
            gen_build_info_dir_path_prefix(distro, arch, manifest_id)
        )

        if check_for_build(manifest_fname, build_request, data["manifest"], build_info_dir, errors):
            build_requests.append(build_request)
        else:
            # The specific build configuration exists in the cache and wont be rebuilt. Update the file timestamps to
            # keep them fresh in the cache.
            touch_s3(distro, arch, manifest_id)

    print("✅ Config filtering done!\n")
    if errors:
        # print errors at the end so they're visible
        print("⚠️ Errors:")
        print("\n".join(errors))

    return build_requests


def clargs():
    default_arch = os.uname().machine
    parser = argparse.ArgumentParser()
    parser.add_argument("config", type=str, help="path to write config")
    parser.add_argument("--distro", type=str, required=True,
                        help="distro to generate configs for")
    parser.add_argument("--arch", type=str, default=default_arch,
                        help="architecture to generate configs for (defaults to host architecture)")

    return parser


def is_manifest_list(data):
    """Inspect a manifest determine if it's a multi-image manifest-list."""
    media_type = data.get("mediaType")
    #  Check if mediaType is set according to docker or oci specifications
    if media_type in ("application/vnd.docker.distribution.manifest.list.v2+json",
                      "application/vnd.oci.image.index.v1+json"):
        return True

    # According to the OCI spec, setting mediaType is not mandatory. So, if it is not set at all, check for the
    # existence of manifests
    if media_type is None and data.get("manifests") is not None:
        return True

    return False


def skopeo_inspect_id(image_name: str, arch: str) -> str:
    """
    Returns the image ID (config digest) of the container image. If the image resolves to a manifest list, the config
    digest of the given architecture is resolved.

    Runs with 'sudo' when inspecting a local container because in our tests we need to read the root container storage.
    """
    cmd = ["skopeo", "inspect", "--raw", image_name]
    if image_name.startswith("containers-storage"):
        cmd = ["sudo"] + cmd
    out, _ = runcmd(cmd)
    data = json.loads(out)
    if not is_manifest_list(data):
        return data["config"]["digest"]

    for manifest in data.get("manifests", []):
        platform = manifest.get("platform", {})
        img_arch = platform.get("architecture", "")
        img_ostype = platform.get("os", "")

        if arch != img_arch or img_ostype != "linux":
            continue

        if "@" in image_name:
            image_no_tag = image_name.split("@")[0]
        else:
            image_no_tag = ":".join(image_name.split(":")[:-1])
        manifest_digest = manifest["digest"]
        arch_image_name = f"{image_no_tag}@{manifest_digest}"
        # inspect the arch-specific manifest to get the image ID (config digest)
        return skopeo_inspect_id(arch_image_name, arch)

    # don't error out, just return an empty string and let the caller handle it
    return ""


def get_tag_for(runner):
    if runner.startswith("aws/"):
        return "terraform"
    if runner.startswith("rhos-01/"):
        return "terraform/openstack"

    raise ValueError(f"Unknown runner: {runner}")


def find_image_file(build_path: str) -> str:
    """
    Find the path to the image by reading the manifest and finding the exported pipeline's output directory.
    A manifest may contain multiple pipelines but only one is exported during a build. This function finds the
    exported pipeline by checking which pipeline directory exists in the build output.
    Raises RuntimeError if no or multiple exported directories are found, or if the directory doesn't contain
    exactly one file.
    """
    manifest_file = os.path.join(build_path, "manifest.json")
    with open(manifest_file, encoding="utf-8") as manifest:
        data = json.load(manifest)

    pipeline_names = [p["name"] for p in data["pipelines"] if p["name"] != "build"]
    export_dirs = [p for p in pipeline_names if os.path.isdir(os.path.join(build_path, p))]

    if len(export_dirs) != 1:
        raise RuntimeError(f"Expected exactly one exported pipeline directory in {build_path}, found: {export_dirs}")

    files = os.listdir(os.path.join(build_path, export_dirs[0]))
    if len(files) != 1:
        raise RuntimeError(
            f"Expected exactly one file in export directory '{export_dirs[0]}', found: {files}")

    return os.path.join(build_path, export_dirs[0], files[0])


def read_manifest(build_path: str) -> Dict:
    """
    Read the manifest.json file from the build directory and return the data as a dictionary.
    """
    info_file_path = os.path.join(build_path, "manifest.json")
    with open(info_file_path, encoding="utf-8") as info_fp:
        return json.load(info_fp)


# pylint: disable=too-many-return-statements,too-many-arguments,too-many-positional-arguments
def can_boot_test(manifest_fname, manifest_data, image_type, arch, distro, blueprint):
    if image_type not in CAN_BOOT_TEST.get("*", []) + CAN_BOOT_TEST.get(arch, []):
        return False

    if image_type in ["image-installer", "minimal-installer"]:
        if not blueprint.get("customizations", {}).get("installer", {}).get("unattended"):
            print("  not bootable: only unattended installers are supported")
            return False

    if image_type in ["network-installer", "everything-network-installer", "server-network-installer"]:
        if distro in ["rhel-10.1", "rhel-10.3"]:  # 10.1 should be removed soon
            print("  not bootable: rhel network-installer tests have incomplete repos in nightly snapshot"
                  "and won't install")
            return False
        if distro.startswith("fedora"):
            print("  not bootable: fedora network-installer crashes in sshd,"
                  "see https://bugzilla.redhat.com/show_bug.cgi?id=2415883")
            return False
        if distro == "centos-9":
            print("  not bootable: centos-9 will not start an install and waits on source selection")
            return False
        if distro.startswith("rhel-9"):
            print("  not bootable: rhel-9 will not start an install and waits on source selection")
            return False

    if image_type in ["qcow2", "generic-qcow2", "cloud-qcow2", "image-installer", "minimal-installer",
                      "network-installer", "everything-network-installer"]:
        if blueprint.get("customizations", {}).get("fips") and distro.startswith("fedora"):
            print("  not bootable: fips on fedora is unstable, fails with e.g. dracut:"
                  "FATAL: FIPS integrity test failed")
            return False
        # Note that this needs adjustment when we switch to librepo
        urls = [src["url"] for src in manifest_data["sources"]["org.osbuild.curl"]["items"].values()]
        if not any("ssh-server" in url for url in urls):
            # This can happen e.g. when an image is build with the "minimal: true" customization.
            # We could use guestfs to inject keys, see PR#1995
            print(f"  not bootable: ssh-server not found in manifest {manifest_fname} ({arch} {image_type})")
            return False
        # We need jq in the image many images do not have it
        # (e.g. centos-9/rhel-9 with releasever config) so skip those too
        if not any("jq" in url for url in urls):
            print(f"  not bootable: jq not found in {manifest_fname} ({arch} {image_type})")
            return False

    return True
