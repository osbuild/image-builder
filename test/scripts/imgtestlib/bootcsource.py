import json
import os

BOOTCREFS_PATH = "./test/data/bootcrefs"
VANILLA_REF_IMAGE_TYPES = {"raw", "vmdk", "ova", "qcow2"}
CLOUD_IMAGE_TYPES = {"ami", "gce", "vhd"}
DISK_IMAGE_TYPES = VANILLA_REF_IMAGE_TYPES | CLOUD_IMAGE_TYPES
INSTALLER_IMAGE_TYPES = {"bootc-generic-iso", "bootc-installer"}

# we test all image types for fedora-44, but only vhd for rhel-10
# since we only have bootc-foundry for rhel-10, we only support vhd for rhel-10
BOOTC_SOURCE_IMAGE_TYPES = {
    "fedora-44": ["qcow2", "ami", "raw", "gce", "vmdk", "ova"],
    "rhel-10": ["vhd"],
}
BOOTC_SOURCES = list(BOOTC_SOURCE_IMAGE_TYPES)
BOOTC_ARCHES = ["x86_64"]
BOOTC_CONFIGS = {"bootc-user"}

BOOTC_QUAY_AUTH_SECURE_FILE = "quay-auth.json"
BOOTC_QUAY_AUTH_PATH = f".secure_files/{BOOTC_QUAY_AUTH_SECURE_FILE}"
QUAY_AUTH_DOWNLOAD_SCRIPT = "./test/scripts/download-quay-auth"


def bootc_image_types_for_source(source_name):
    try:
        return BOOTC_SOURCE_IMAGE_TYPES[source_name]
    except KeyError as exc:
        raise KeyError(f"unknown bootc source {source_name}") from exc


def load_bootc_source(source_name):
    path = os.path.join(BOOTCREFS_PATH, source_name + ".json")
    with open(path, encoding="utf-8") as source_file:
        return json.load(source_file)


def list_bootc_source_arches(source_name):
    return sorted(load_bootc_source(source_name).keys())


def resolve_bootc_source(source_name, arch):
    data = load_bootc_source(source_name)

    try:
        entry = data[arch]
    except KeyError as exc:
        raise KeyError(f"bootc source {source_name} does not define arch {arch}") from exc

    if not isinstance(entry, dict):
        raise TypeError(f"bootc source {source_name} entry for arch {arch} must be an object")

    return entry


def bootc_source_from_distro(distro):
    """Remove the bootc prefix from the distro name."""
    if not distro.startswith("bootc-"):
        return None
    return distro.removeprefix("bootc-").rsplit(".", 1)[0]


def resolve_ref_from_entry(entry, source_name, arch, image_type=None):
    derived_refs = entry.get("derived_refs", {})
    base_ref = entry.get("ref")
    if image_type in DISK_IMAGE_TYPES:
        ref = derived_refs.get(image_type) or derived_refs.get("disk")
        if ref:
            return ref
        if image_type in VANILLA_REF_IMAGE_TYPES:
            return base_ref
        return None
    if image_type in INSTALLER_IMAGE_TYPES:
        return derived_refs.get(image_type) or derived_refs.get("installer")

    ref = base_ref
    if not isinstance(ref, str) or not ref:
        raise ValueError(f"bootc source {source_name} entry for arch {arch} must define a non-empty 'ref' string")
    return ref


def resolve_bootc_source_ref(source_name, arch, image_type=None):
    entry = resolve_bootc_source(source_name, arch)
    return resolve_ref_from_entry(entry, source_name, arch, image_type=image_type)


def needs_quay_auth(bootc_ref):
    return "image-builder-bootc-foundry" in bootc_ref


def quay_auth_file_path():
    return os.path.abspath(BOOTC_QUAY_AUTH_PATH)


def ensure_quay_auth_file():
    path = quay_auth_file_path()
    if not os.path.isfile(path):
        raise RuntimeError(
            f"{BOOTC_QUAY_AUTH_SECURE_FILE} not found at {path}; "
            f"run {QUAY_AUTH_DOWNLOAD_SCRIPT} first"
        )
    return path


def quay_auth_extra_env(bootc_refs):
    """Return env var needed to pull private bootc-foundry images."""
    if not bootc_refs or not any(needs_quay_auth(ref) for ref in bootc_refs):
        return {}
    return {"REGISTRY_AUTH_FILE": ensure_quay_auth_file()}
