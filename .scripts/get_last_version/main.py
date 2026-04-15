# Copyright 2026 PointerByte Contributors
# SPDX-License-Identifier: Apache-2.0

import argparse
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

REPO_URL = "https://github.com/PointerByte/QuicksGo.git"
PROJECT_ROOT = Path(__file__).resolve().parents[2]


@dataclass(frozen=True)
class ManagedModule:
    name: str
    directory: str
    module_path: str
    tag_prefix: str


MANAGED_MODULES = (
    ManagedModule(
        name="root",
        directory=".",
        module_path="github.com/PointerByte/QuicksGo",
        tag_prefix="",
    ),
    ManagedModule(
        name="logger",
        directory="logger",
        module_path="github.com/PointerByte/QuicksGo/logger",
        tag_prefix="logger/",
    ),
    ManagedModule(
        name="encrypt",
        directory="encrypt",
        module_path="github.com/PointerByte/QuicksGo/encrypt",
        tag_prefix="encrypt/",
    ),
    ManagedModule(
        name="security",
        directory="security",
        module_path="github.com/PointerByte/QuicksGo/security",
        tag_prefix="security/",
    ),
    ManagedModule(
        name="cmd/qgo",
        directory="cmd/qgo",
        module_path="github.com/PointerByte/QuicksGo/cmd/qgo",
        tag_prefix="cmd/qgo/",
    ),
)

SEMVER_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")
REQUIRE_RE_TEMPLATE = r"(?m)^(\s*{module_path}\s+)v\d+\.\d+\.\d+(\s*(?://.*)?)$"


def run(cmd: list[str], cwd: Path = PROJECT_ROOT) -> str:
    result = subprocess.run(
        cmd,
        cwd=str(cwd),
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout.strip()


def load_remote_tags(repo_url: str) -> list[str]:
    output = run(["git", "ls-remote", "--tags", "--refs", repo_url])
    tags = []
    for line in output.splitlines():
        parts = line.split()
        if len(parts) != 2:
            continue
        ref = parts[1]
        if ref.startswith("refs/tags/"):
            tags.append(ref.removeprefix("refs/tags/"))
    return tags


def load_local_tags() -> list[str]:
    output = run(["git", "tag", "--list"])
    return [line.strip() for line in output.splitlines() if line.strip()]


def semver_key(version: str) -> tuple[int, int, int]:
    match = SEMVER_RE.match(version)
    if not match:
        raise ValueError(f"Invalid semantic version: {version}")
    return tuple(int(part) for part in match.groups())


def latest_tag_for_module(tags: list[str], module: ManagedModule) -> str | None:
    matching_versions: list[tuple[tuple[int, int, int], str]] = []

    for tag in tags:
        if not tag.startswith(module.tag_prefix):
            continue
        version = tag[len(module.tag_prefix):]
        if not SEMVER_RE.match(version):
            continue
        matching_versions.append((semver_key(version), tag))

    if not matching_versions:
        return None

    matching_versions.sort(key=lambda item: item[0])
    return matching_versions[-1][1]


def latest_available_tag_for_module(
    remote_tags: list[str],
    local_tags: list[str],
    module: ManagedModule,
) -> str | None:
    candidates: list[tuple[tuple[int, int, int], str]] = []

    for source in (remote_tags, local_tags):
        tag = latest_tag_for_module(source, module)
        if not tag:
            continue
        version = tag[len(module.tag_prefix):]
        candidates.append((semver_key(version), tag))

    if not candidates:
        return None

    candidates.sort(key=lambda item: item[0])
    return candidates[-1][1]


def bump_version(tag: str | None, bump_type: str, tag_prefix: str) -> str:
    base_version = "v0.0.0"
    if tag:
        base_version = tag[len(tag_prefix):]

    major, minor, patch = semver_key(base_version)

    if bump_type == "major":
        major += 1
        minor = 0
        patch = 0
    elif bump_type == "minor":
        minor += 1
        patch = 0
    else:
        patch += 1

    return f"{tag_prefix}v{major}.{minor}.{patch}"


def changed_files() -> list[str]:
    tracked = run(["git", "diff", "--name-only", "HEAD"])
    untracked = run(["git", "ls-files", "--others", "--exclude-standard"])

    result = []
    for block in (tracked, untracked):
        for line in block.splitlines():
            line = line.strip()
            if line:
                result.append(line)
    return sorted(set(result))


def module_has_changes(module: ManagedModule, files: list[str]) -> bool:
    if module.directory == ".":
        module_paths = tuple(
            f"{managed.directory}/"
            for managed in MANAGED_MODULES
            if managed.directory != "."
        )
        return any(not path.startswith(module_paths) for path in files)

    prefix = f"{module.directory}/"
    return any(path.startswith(prefix) for path in files)


def version_from_tag(tag: str) -> str:
    if "/" not in tag:
        return tag
    return tag.split("/")[-1]


def build_suggested_tags(
    modules: tuple[ManagedModule, ...],
    remote_tags: list[str],
    local_tags: list[str],
    changed_paths: list[str],
    bump_type: str,
) -> dict[str, str]:
    suggestions: dict[str, str] = {}

    for module in modules:
        latest_tag = latest_available_tag_for_module(remote_tags, local_tags, module)
        suggestions[module.module_path] = bump_version(latest_tag, bump_type, module.tag_prefix)

    return suggestions


def go_mod_path_for_module(module: ManagedModule) -> Path:
    if module.directory == ".":
        return PROJECT_ROOT / "go.mod"
    return PROJECT_ROOT / module.directory / "go.mod"


def prompt_go_mod_selection(modules: tuple[ManagedModule, ...]) -> tuple[ManagedModule, ...]:
    go_mod_modules = [module for module in modules if go_mod_path_for_module(module).exists()]

    if not sys.stdin.isatty():
        return tuple(go_mod_modules)

    print("")
    print("Selecciona el go.mod que deseas actualizar:")
    print("0) todos")

    for index, module in enumerate(go_mod_modules, start=1):
        go_mod_path = go_mod_path_for_module(module).relative_to(PROJECT_ROOT)
        print(f"{index}) {go_mod_path}")

    while True:
        selected = input("Opcion: ").strip()
        if selected == "0":
            return tuple(go_mod_modules)
        if selected.isdigit():
            index = int(selected) - 1
            if 0 <= index < len(go_mod_modules):
                return (go_mod_modules[index],)
        print("Opcion invalida. Ingresa un numero de la lista.")


def update_go_mod_dependencies(
    modules: tuple[ManagedModule, ...],
    suggested_tags: dict[str, str],
) -> list[str]:
    updated_files: list[str] = []

    for module in modules:
        go_mod_path = go_mod_path_for_module(module)
        if not go_mod_path.exists():
            continue

        original = go_mod_path.read_text(encoding="utf-8")
        updated = original

        for dependency_path, suggested_tag in suggested_tags.items():
            version = version_from_tag(suggested_tag)
            pattern = REQUIRE_RE_TEMPLATE.format(module_path=re.escape(dependency_path))
            updated = re.sub(pattern, rf"\g<1>{version}\2", updated)

        if updated != original:
            go_mod_path.write_text(updated, encoding="utf-8")
            updated_files.append(str(go_mod_path.relative_to(PROJECT_ROOT)))

    return updated_files


def format_report(
    repo_url: str,
    modules: tuple[ManagedModule, ...],
    remote_tags: list[str],
    local_tags: list[str],
    changed_paths: list[str],
    bump_type: str,
    only_changed: bool,
    suggested_tags: dict[str, str],
    updated_go_mod_files: list[str],
) -> str:
    lines = [
        "ULTIMAS VERSIONES DE MODULOS QUICKSGO",
        "=" * 80,
        f"Repositorio remoto: {repo_url}",
        f"Tipo de incremento sugerido: {bump_type}",
        "",
    ]

    if changed_paths:
        lines.append("Archivos con cambios detectados:")
        for path in changed_paths:
            lines.append(f"- {path}")
        lines.append("")
    else:
        lines.append("No se detectaron cambios locales en el arbol de trabajo.")
        lines.append("")

    lines.append("Resumen por modulo:")
    lines.append("")

    for module in modules:
        has_changes = module_has_changes(module, changed_paths)
        if only_changed and not has_changes:
            continue

        remote_tag = latest_tag_for_module(remote_tags, module)
        local_tag = latest_tag_for_module(local_tags, module)
        suggested_tag = suggested_tags.get(module.module_path, "-")

        lines.append(f"[{module.name}]")
        lines.append(f"modulo: {module.module_path}")
        lines.append(f"directorio: {module.directory}")
        lines.append(f"ultimo tag remoto: {remote_tag or 'sin tags'}")
        lines.append(f"ultimo tag local: {local_tag or 'sin tags'}")
        lines.append(f"tiene cambios: {'si' if has_changes else 'no'}")
        lines.append(f"siguiente tag sugerido: {suggested_tag}")
        lines.append("")

    lines.append("Tags sugeridos para publicar:")
    suggested_count = 0
    for module in modules:
        has_changes = module_has_changes(module, changed_paths)
        if only_changed and not has_changes:
            continue
        suggested_count += 1
        suggested_tag = suggested_tags[module.module_path]
        lines.append(f"- {module.name}: {suggested_tag}")

    if suggested_count == 0:
        lines.append("- No hay modulos para mostrar con el filtro actual.")
        lines.append("")
        lines.append("Comandos git sugeridos:")
        lines.append("- No hay comandos para generar tags con el filtro actual.")
        return "\n".join(lines)

    lines.append("")
    lines.append("Comandos git sugeridos:")
    lines.append("")

    for module in modules:
        has_changes = module_has_changes(module, changed_paths)
        if only_changed and not has_changes:
            continue

        suggested_tag = suggested_tags[module.module_path]

        lines.append(f"# {module.name}")
        lines.append(f"git tag {suggested_tag}")
        lines.append(f"git push origin {suggested_tag}")
        lines.append("")

    lines.append("Actualizacion de go.mod:")
    if updated_go_mod_files:
        for path in updated_go_mod_files:
            lines.append(f"- actualizado: {path}")
    else:
        lines.append("- sin cambios en go.mod")
    lines.append("")

    return "\n".join(lines)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Obtiene los ultimos tags remotos de GitHub para los modulos de QuicksGo y sugiere la siguiente version."
    )
    parser.set_defaults(update_go_mod=True)
    parser.add_argument(
        "--repo-url",
        default=REPO_URL,
        help="Repositorio remoto de GitHub a consultar.",
    )
    parser.add_argument(
        "--bump",
        choices=("patch", "minor", "major"),
        default="patch",
        help="Tipo de incremento para el siguiente tag sugerido.",
    )
    parser.add_argument(
        "--only-changed",
        action="store_true",
        help="Muestra solo los modulos que tienen cambios locales.",
    )
    parser.add_argument(
        "--update-go-mod",
        action="store_true",
        help="Actualiza los go.mod de los modulos dependientes con las versiones sugeridas.",
    )
    parser.add_argument(
        "--skip-update-go-mod",
        dest="update_go_mod",
        action="store_false",
        help="Omite la actualizacion automatica de los go.mod dependientes.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()

    remote_tags: list[str] = []
    try:
        remote_tags = load_remote_tags(args.repo_url)
    except subprocess.CalledProcessError as exc:
        print("No fue posible obtener los tags remotos. Se usaran los tags locales como respaldo.")
        if exc.stderr:
            print(exc.stderr)

    try:
        local_tags = load_local_tags()
        changed_paths = changed_files()
    except subprocess.CalledProcessError as exc:
        print("No fue posible obtener la informacion de versiones.")
        if exc.stdout:
            print(exc.stdout)
        if exc.stderr:
            print(exc.stderr)
        return 1

    suggested_tags = build_suggested_tags(
        modules=MANAGED_MODULES,
        remote_tags=remote_tags,
        local_tags=local_tags,
        changed_paths=changed_paths,
        bump_type=args.bump,
    )
    updated_go_mod_files: list[str] = []
    if args.update_go_mod:
        selected_modules = prompt_go_mod_selection(MANAGED_MODULES)
        updated_go_mod_files = update_go_mod_dependencies(selected_modules, suggested_tags)

    report = format_report(
        repo_url=args.repo_url,
        modules=MANAGED_MODULES,
        remote_tags=remote_tags,
        local_tags=local_tags,
        changed_paths=changed_paths,
        bump_type=args.bump,
        only_changed=args.only_changed,
        suggested_tags=suggested_tags,
        updated_go_mod_files=updated_go_mod_files,
    )
    print(report)
    return 0


# Este comentario final muestra al usuario como ejecutarlo desde la raiz del proyecto.
# ----------------------------------------------------------------------------------------------------
# INSTRUCCION DE EJECUCION FINAL:
# Para consultar los ultimos tags remotos, sugerir la siguiente version por modulo y opcionalmente
# actualizar los go.mod dependientes, navega a la raiz del proyecto y ejecuta:
# python ./.scripts/get_last_version/main.py
# ====================================================================================================
if __name__ == "__main__":
    raise SystemExit(main())
