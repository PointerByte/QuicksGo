# Copyright 2026 PointerByte Contributors
# SPDX-License-Identifier: Apache-2.0

import argparse
import json
import re
import subprocess
from collections import defaultdict
from pathlib import Path

OUTPUT_FILE = "../../THIRD_PARTY_LICENSES.txt"
MAX_DEPS = 10000

# 🔥 Ignore internal modules
IGNORE_PREFIXES = [
    "github.com/PointerByte/QuicksGo",
]

LICENSE_CANDIDATES = [
    "LICENSE",
    "LICENSE.txt",
    "LICENSE.md",
    "LICENCE",
    "LICENCE.txt",
    "COPYING",
    "COPYING.txt",
    "NOTICE",
    "NOTICE.txt",
]

LICENSE_PATTERNS = [
    ("Apache License 2.0", [r"apache license", r"version 2\.0"]),
    ("MIT License", [r"\bmit license\b"]),
    ("BSD 3-Clause", [r"redistribution and use in source and binary forms", r"neither the name"]),
    ("BSD 2-Clause", [r"redistribution and use in source and binary forms"]),
    ("GPL", [r"gnu general public license"]),
    ("LGPL", [r"gnu lesser general public license"]),
    ("MPL 2.0", [r"mozilla public license", r"2\.0"]),
    ("CC0 1.0 Universal", [r"(creative commons(?: zero)?|cc0)", r"1\.0", r"(universal|public domain)"]),
]


def should_ignore(module_path: str) -> bool:
    return any(module_path.startswith(prefix) for prefix in IGNORE_PREFIXES)


def run(cmd: list[str], cwd: Path) -> str:
    result = subprocess.run(
        cmd,
        capture_output=True,
        text=True,
        check=True,
        cwd=str(cwd),
    )
    return result.stdout.strip()


def validate_go_project(project_path: Path) -> None:
    if not project_path.exists():
        raise FileNotFoundError(f"Path does not exist: {project_path}")

    if not project_path.is_dir():
        raise NotADirectoryError(f"Path is not a directory: {project_path}")

    go_mod = project_path / "go.mod"
    if not go_mod.exists():
        raise FileNotFoundError(f"go.mod was not found in: {project_path}")


def get_direct_dependencies(project_path: Path) -> list[dict]:
    output = run(["go", "list", "-m", "-json", "all"], cwd=project_path)
    decoder = json.JSONDecoder()

    deps = []
    idx = 0
    length = len(output)

    while idx < length:
        while idx < length and output[idx].isspace():
            idx += 1
        if idx >= length:
            break

        obj, end = decoder.raw_decode(output, idx)
        idx = end

        if obj.get("Main", False):
            continue

        dep = {
            "Path": obj.get("Path", ""),
            "Version": obj.get("Version", ""),
            "Dir": obj.get("Dir", ""),
            "Indirect": obj.get("Indirect", False),
        }
        deps.append(dep)

    direct_deps = [d for d in deps if not d["Indirect"]]
    return direct_deps[:MAX_DEPS]


def find_license_file(module_dir: Path) -> Path | None:
    if not module_dir.exists() or not module_dir.is_dir():
        return None

    for name in LICENSE_CANDIDATES:
        candidate = module_dir / name
        if candidate.is_file():
            return candidate

    try:
        for item in module_dir.iterdir():
            if item.is_file():
                upper_name = item.name.upper()
                if (
                    upper_name.startswith("LICENSE")
                    or upper_name.startswith("LICENCE")
                    or upper_name.startswith("COPYING")
                    or upper_name.startswith("NOTICE")
                ):
                    return item
    except OSError:
        return None

    return None


def detect_license_type(text: str) -> str:
    content = text.lower()

    # 🔥 Strong CC0 detection
    if (
        (re.search(r"(creative commons(?: zero)?|cc0)", content) and re.search(r"1\.0", content))
        or (re.search(r"public domain", content) and re.search(r"no copyright", content))
    ):
        return "CC0 1.0 Universal"

    for license_name, patterns in LICENSE_PATTERNS:
        if all(re.search(pattern, content) for pattern in patterns):
            return license_name

    return "Unknown"


def read_text_file(path: Path) -> str:
    try:
        return path.read_text(encoding="utf-8", errors="ignore")
    except Exception:
        return ""


def collect_dependency_licenses(deps: list[dict]) -> list[dict]:
    results = []

    for dep in deps:
        module_path = dep.get("Path", "")
        version = dep.get("Version", "")
        dir_value = dep.get("Dir", "")
        module_dir = Path(dir_value) if dir_value else None

        item = {
            "module": module_path,
            "version": version,
            "license_type": "Unknown",
            "notes": "",
        }

        if not module_dir:
            item["notes"] = "The module does not include a local directory. Run `go mod download`."
            results.append(item)
            continue

        if not module_dir.exists():
            item["notes"] = "The module directory does not exist in cache. Run `go mod download`."
            results.append(item)
            continue

        license_file = find_license_file(module_dir)
        if not license_file:
            item["notes"] = "License file was not found."
            results.append(item)
            continue

        content = read_text_file(license_file)
        item["license_type"] = detect_license_type(content)
        results.append(item)

    return results


def merge_dependencies(projects: list[Path]) -> list[dict]:
    merged: dict[tuple[str, str], dict] = {}

    for project_path in projects:
        deps = get_direct_dependencies(project_path)

        for dep in deps:
            module_path = dep.get("Path", "")

            # 🔥 Ignore internal modules
            if should_ignore(module_path):
                continue

            key = (module_path, dep.get("Version", ""))
            if key not in merged:
                merged[key] = dep

    return sorted(
        merged.values(),
        key=lambda item: (item.get("Path", ""), item.get("Version", "")),
    )


def build_report(projects: list[Path], deps: list[dict]) -> str:
    lines = [
        "THIRD PARTY LICENSES",
        "=" * 80,
    ]

    lines.extend([
        "",
        "This project uses the following third-party direct dependencies:",
        "",
    ])

    if not deps:
        lines.extend([
            "No direct dependencies were found.",
            "",
        ])
        return "\n".join(lines)

    collected = collect_dependency_licenses(deps)

    for i, item in enumerate(collected, start=1):
        lines.append(f"{i}. {item['module']}")
        lines.append(f"   Version: {item['version']}")
        lines.append(f"   License: {item['license_type']}")
        if item["notes"]:
            lines.append(f"   Notes: {item['notes']}")
        lines.append("")

    grouped = defaultdict(list)
    for item in collected:
        grouped[item["license_type"]].append(item["module"])

    lines.append("=" * 80)
    lines.append("SUMMARY BY LICENSE")
    lines.append("=" * 80)
    lines.append("")

    for license_type in sorted(grouped.keys()):
        unique_modules = sorted(set(grouped[license_type]))
        lines.append(f"{license_type}: {len(unique_modules)}")
        for module_name in unique_modules:
            lines.append(f"  - {module_name}")
        lines.append("")

    return "\n".join(lines)


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Generate a summarized THIRD_PARTY_LICENSES.txt from one or more Go projects"
    )
    parser.add_argument(
        "--project",
        action="append",
        required=True,
        help="Path to a Go project containing go.mod. Can be used multiple times.",
    )
    parser.add_argument(
        "--output",
        default=OUTPUT_FILE,
        help="Output file path",
    )
    args = parser.parse_args()

    project_paths = [Path(project).resolve() for project in args.project]
    output_path = Path(args.output).resolve()

    try:
        for project_path in project_paths:
            validate_go_project(project_path)

        deps = merge_dependencies(project_paths)

        print("\nDirect dependencies found:")
        if not deps:
            print("- None")
        else:
            for dep in deps:
                print(f"- {dep['Path']} {dep['Version']}")

        report = build_report(project_paths, deps)
        output_path.write_text(report, encoding="utf-8")

        print(f"\nGenerated file: {output_path}")

    except subprocess.CalledProcessError as exc:
        print("Command execution failed:")
        print("CMD:", " ".join(exc.cmd) if isinstance(exc.cmd, list) else exc.cmd)
        print("STDERR:", exc.stderr.strip() if exc.stderr else "No details")
    except Exception as exc:
        print(f"Error: {exc}")


if __name__ == "__main__":
    main()

# python .\main.py --project "../../cmd/qgo" --project "../../security" --project "../../logger" --project "../../config" --project "../../"