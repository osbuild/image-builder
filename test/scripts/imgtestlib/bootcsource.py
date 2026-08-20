import json
import os

BOOTCREFS_PATH = "./test/data/bootcrefs"
VANILLA_REF_IMAGE_TYPES = {"raw", "vmdk", "ova", "qcow2"}
CLOUD_IMAGE_TYPES = {"ami", "gce", "vhd"}
DISK_IMAGE_TYPES = VANILLA_REF_IMAGE_TYPES | CLOUD_IMAGE_TYPES
INSTALLER_IMAGE_TYPES = {"bootc-generic-iso", "bootc-installer"}


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
