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
]


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
        raise FileNotFoundError(f"No existe la ruta: {project_path}")

    if not project_path.is_dir():
        raise NotADirectoryError(f"La ruta no es un directorio: {project_path}")

    go_mod = project_path / "go.mod"
    if not go_mod.exists():
        raise FileNotFoundError(f"No se encontró go.mod en: {project_path}")


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
            item["notes"] = "El módulo no incluye directorio local. Ejecuta `go mod download`."
            results.append(item)
            continue

        if not module_dir.exists():
            item["notes"] = "El directorio del módulo no existe en caché. Ejecuta `go mod download`."
            results.append(item)
            continue

        license_file = find_license_file(module_dir)
        if not license_file:
            item["notes"] = "No se encontró archivo de licencia."
            results.append(item)
            continue

        content = read_text_file(license_file)
        item["license_type"] = detect_license_type(content)
        results.append(item)

    return results


def build_report(deps: list[dict]) -> str:
    lines = [
        "THIRD PARTY LICENSES",
        "=" * 80,
        "",
        "This project uses the following third-party dependencies:",
        "",
    ]

    if not deps:
        lines.extend([
            "No se encontraron dependencias directas.",
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
        lines.append(f"{license_type}: {len(grouped[license_type])}")
        for module_name in sorted(grouped[license_type]):
            lines.append(f"  - {module_name}")
        lines.append("")

    return "\n".join(lines)


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Genera THIRD_PARTY_LICENSES.txt resumido a partir de un proyecto Go"
    )
    parser.add_argument(
        "--project",
        required=True,
        help="Ruta al proyecto Go donde existe go.mod",
    )
    parser.add_argument(
        "--output",
        default=OUTPUT_FILE,
        help="Archivo de salida",
    )
    args = parser.parse_args()

    project_path = Path(args.project).resolve()
    output_path = Path(args.output).resolve()

    try:
        validate_go_project(project_path)

        deps = get_direct_dependencies(project_path)

        print(f"Proyecto Go: {project_path}")
        print("Dependencias directas encontradas:")
        if not deps:
            print("- Ninguna")
        else:
            for dep in deps:
                print(f"- {dep['Path']} {dep['Version']}")

        report = build_report(deps)
        output_path.write_text(report, encoding="utf-8")

        print(f"\nArchivo generado: {output_path}")

    except subprocess.CalledProcessError as exc:
        print("Error ejecutando comando:")
        print("CMD:", " ".join(exc.cmd) if isinstance(exc.cmd, list) else exc.cmd)
        print("STDERR:", exc.stderr.strip() if exc.stderr else "Sin detalle")
    except Exception as exc:
        print(f"Error: {exc}")


if __name__ == "__main__":
    main()

# python .\main.py --project "../../config" --project "../../security" --project "../../logger"