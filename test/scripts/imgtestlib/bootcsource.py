import json
import os

BOOTCREFS_PATH = "./test/data/bootcrefs"


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

    ref = entry.get("ref")
    if not isinstance(ref, str) or not ref:
        raise ValueError(f"bootc source {source_name} entry for arch {arch} must define a non-empty 'ref' string")

    return entry
