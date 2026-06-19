#!/usr/bin/env python3

import argparse
import os
import subprocess
import sys
from collections import namedtuple
from typing import List, Tuple

from packaging.version import Version

SpecDep = namedtuple('SpecDep', ['name', 'op', 'version'])
SpecDep.__str__ = lambda self: f'{self.name} {self.op} {self.version}'


def get_spec_deps(spec: str) -> List[Tuple[str, str, str]]:
    """
    Get the RPM dependencies from a SPEC file using 'rpmspec'
    """
    deps = []
    cmd = ['rpmspec', '-q', spec, '--requires']
    try:
        output = subprocess.check_output(cmd, text=True)
    except subprocess.CalledProcessError as e:
        print(f"::error file={spec},title=⛔ error hint::Failed to extract dependencies using 'rpmspec': {e}")
        sys.exit(1)

    for line in set(output.splitlines()):
        line = line.strip()
        if line:
            parts = iter(line.split(maxsplit=2))
            dep_name, dep_op, dep_version = [next(parts, None) for _ in range(3)]
            deps.append(SpecDep(dep_name, dep_op, dep_version))
    return deps


ImagesDep = namedtuple('ImagesDep', ['name', 'min_version'])


def get_images_deps(images_dir: str) -> List[Tuple[str, str]]:
    """
    Get the osbuild/image-builder dependencies from the images directory

    Each non-go file in the 'data/dependencies' directory is considered a dependency.
    The filename is the name of the dependency and the content is the minimum version.
    """
    deps = []
    deps_dir = os.path.abspath(os.path.join(images_dir, 'data', 'dependencies'))
    # prevent directory traversal
    if not deps_dir.startswith(os.getcwd()):
        print(f"::error file={images_dir},title=⛔ error hint::Normalization of the path failed. "
              f"{deps_dir} is not within {os.getcwd()}")
        sys.exit(1)
    if not os.path.exists(deps_dir):
        print(f"::error file={images_dir},title=⛔ error hint::Dependencies directory not found")
        sys.exit(1)

    for dep_file in os.listdir(deps_dir):
        # Skip go and markdown files
        if dep_file.endswith('.go') or dep_file.endswith('.md'):
            continue

        dep_name = dep_file
        with open(os.path.join(deps_dir, dep_file), 'r', encoding='utf-8') as f:
            dep_version = f.read().strip()
        deps.append(ImagesDep(dep_name, dep_version))

    return deps


def main():
    parser = argparse.ArgumentParser(description="Check SPEC file dependencies")
    parser.add_argument(
        'SPEC',
        help="Path to the SPEC file to check"
    )
    parser.add_argument(
        'IMAGES_DIR',
        help="Path to the directory containing the images",
        nargs='?',
        default='./vendor/github.com/osbuild/image-builder'
    )

    args = parser.parse_args()
    spec = args.SPEC
    images_dir = args.IMAGES_DIR

    if not os.path.exists(spec):
        print(f"::error file={spec},title=⛔ error hint::File not found")
        sys.exit(1)

    if not os.path.isdir(images_dir):
        print(f"::error file={images_dir},title=⛔ error hint::osbuild/image-builder directory not found")
        sys.exit(1)

    spec_deps = get_spec_deps(spec)
    print(f"::debug::spec_deps={spec_deps}")

    images_deps = get_images_deps(images_dir)
    print(f"::debug::images_deps={images_deps}")

    error = False
    # SPEC file must depend at least on the minimum version defined in the dependency file
    #
    # The dependency operators must be either `=` or `>` or `>=`, but not `<` or `<=`,
    # because the library defines the minimum required version.
    for dep in images_deps:
        spec_dep = next((spec_dep for spec_dep in spec_deps if spec_dep.name == dep.name), None)
        if not spec_dep:
            print(f"::error file={spec},title=⛔ error hint::Missing dependency in the SPEC file: {dep.name}")
            error = True
            continue

        if not spec_dep.op or not spec_dep.version:
            print(f"::error file={spec},title=⛔ error hint::Missing operator or version in the SPEC file dependency: "
                  f"{spec_dep.name}")
            error = True
            continue

        if spec_dep.op in ['<', '<=']:
            print(f"::error file={spec},title=⛔ error hint::Unsupported operator in the SPEC file: '{spec_dep}'. "
                  "he osbuild/image-builder defines the minimum version, so only '=', '>=', '>' operators are allowed")
            error = True
            continue

        # If the operator is `>`, the version must be at maximum one version less than the minimum version.
        # However, we don't know which version it was and what versioning scheme is used by the dependency.
        # Therefore we only assume that the version in SPEC must be at least the minimum version.
        # Effectively, this is the same as the `>=` and '=' operator.
        if spec_dep.op in ['=', '>=', '>'] and Version(spec_dep.version) < Version(dep.min_version):
            print(f"::error file={spec},title=⛔ error hint::Incorrect required dependency version in the SPEC file: "
                  f"'{spec_dep}'. Expected: {dep.name} {spec_dep.op} {dep.min_version} (at least)")
            error = True
            continue

        print(f"✅ SPEC file dependency for '{spec_dep}' is correct")

    if error:
        print(f"::error file={spec},title=⛔ error hint::Incorrect dependencies in the SPEC file")
        sys.exit(1)
    else:
        print("✅ All SPEC file dependencies are correct")


if __name__ == '__main__':
    main()
