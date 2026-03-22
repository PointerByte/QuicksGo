# Copyright 2026 PointerByte Contributors
# SPDX-License-Identifier: Apache-2.0

import argparse
import json
import subprocess
from pathlib import Path

DEFAULT_OUTPUT = "../../NOTICE"

def run(cmd: list[str], cwd: Path | None = None) -> str:
    result = subprocess.run(
        cmd,
        capture_output=True,
        text=True,
        check=True,
        cwd=str(cwd) if cwd else None,
    )
    return result.stdout.strip()


def validate_go_project(project_path: Path) -> None:
    if not project_path.exists():
        raise FileNotFoundError(f"No existe la ruta: {project_path}")
    if not project_path.is_dir():
        raise NotADirectoryError(f"La ruta no es un directorio: {project_path}")
    if not (project_path / "go.mod").exists():
        raise FileNotFoundError(f"No se encontró go.mod en: {project_path}")


def get_modules(project_path: Path, include_indirect: bool = False) -> list[dict]:
    output = run(["go", "list", "-m", "-json", "all"], cwd=project_path)
    decoder = json.JSONDecoder()

    modules = []
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

        mod = {
            "Path": obj.get("Path", ""),
            "Version": obj.get("Version", ""),
            "Dir": obj.get("Dir", ""),
            "Indirect": obj.get("Indirect", False),
        }

        if include_indirect or not mod["Indirect"]:
            modules.append(mod)

    return modules


def find_notice_file(module_dir: Path) -> Path | None:
    if not module_dir.exists() or not module_dir.is_dir():
        return None

    candidates = [
        "NOTICE",
        "NOTICE.txt",
        "NOTICE.md",
    ]

    for name in candidates:
        p = module_dir / name
        if p.is_file():
            return p

    try:
        for item in module_dir.iterdir():
            if item.is_file() and item.name.upper().startswith("NOTICE"):
                return item
    except OSError:
        return None

    return None


def read_text_file(path: Path) -> str:
    return path.read_text(encoding="utf-8", errors="ignore").strip()


def build_notice(modules: list[dict], include_empty_section: bool = False) -> str:
    lines = []
    lines.append("NOTICE")
    lines.append("")
    lines.append("This product includes third-party software.")
    lines.append("")

    found_any = False

    for mod in modules:
        module_path = mod["Path"]
        version = mod["Version"]
        dir_value = mod["Dir"]

        if not dir_value:
            if include_empty_section:
                lines.append("=" * 80)
                lines.append(f"Module: {module_path}")
                lines.append(f"Version: {version}")
                lines.append("NOTICE: No encontrado (sin directorio local)")
                lines.append("")
            continue

        module_dir = Path(dir_value)
        notice_file = find_notice_file(module_dir)

        if not notice_file:
            if include_empty_section:
                lines.append("=" * 80)
                lines.append(f"Module: {module_path}")
                lines.append(f"Version: {version}")
                lines.append("NOTICE: No encontrado")
                lines.append("")
            continue

        found_any = True
        notice_text = read_text_file(notice_file)

        lines.append("=" * 80)
        lines.append(f"Module: {module_path}")
        lines.append(f"Version: {version}")
        lines.append("-" * 80)
        lines.append(notice_text)
        lines.append("")

    if not found_any and not include_empty_section:
        lines.append("No se encontraron archivos NOTICE en las dependencias revisadas.")
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


def main() -> None:
    parser = argparse.ArgumentParser(description="Generador de NOTICE para proyecto Go")
    parser.add_argument(
        "--project",
        required=True,
        help="Ruta al proyecto Go donde existe go.mod",
    )
    parser.add_argument(
        "--output",
        default=DEFAULT_OUTPUT,
        help="Archivo de salida. Default: NOTICE",
    )
    parser.add_argument(
        "--include-indirect",
        action="store_true",
        help="Incluir dependencias indirectas",
    )
    parser.add_argument(
        "--include-empty-section",
        action="store_true",
        help="Agregar también módulos donde no se encontró NOTICE",
    )

    args = parser.parse_args()

    project_path = Path(args.project).resolve()
    output_path = Path(args.output).resolve()

    try:
        validate_go_project(project_path)

        modules = get_modules(
            project_path=project_path,
            include_indirect=args.include_indirect,
        )

        print(f"Proyecto Go: {project_path}")
        print(f"Módulos revisados: {len(modules)}")

        content = build_notice(
            modules=modules,
            include_empty_section=args.include_empty_section,
        )

        output_path.write_text(content, encoding="utf-8")
        print(f"Archivo generado: {output_path}")

    except subprocess.CalledProcessError as exc:
        print("Error ejecutando comando:")
        print("CMD:", " ".join(exc.cmd) if isinstance(exc.cmd, list) else exc.cmd)
        print("STDERR:", exc.stderr.strip() if exc.stderr else "Sin detalle")
    except Exception as exc:
        print(f"Error: {exc}")


if __name__ == "__main__":
    main()