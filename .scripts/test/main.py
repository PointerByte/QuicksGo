# Copyright 2026 PointerByte Contributors
# SPDX-License-Identifier: Apache-2.0

import os
import subprocess


PROJECT_ROOT = "/run/media/mblanco/Multimedia/Proyects/Practices/QuicksGo"
DEFAULT_COVERAGE_FILE = "coverage.out"

REQUIRED_SETUP_MODULES = [
    {"name": "root", "relative_dir": "."},
    {"name": "logger", "relative_dir": "logger"},
    {"name": "encrypt", "relative_dir": "encrypt"},
    {"name": "security", "relative_dir": "security"},
    {"name": "cmd/qgo", "relative_dir": "cmd/qgo"},
    {"name": "cmd/go-openssl", "relative_dir": "cmd/go-openssl"},
]

OPTIONAL_SETUP_MODULES = [
    {
        "name": "cmd/qgo",
        "relative_dir": "cmd/qgo",
        "prompt": "Deseas preparar dependencias para cmd/qgo? (s/n): ",
        "skip_message": "INFO: Omitiendo la preparación opcional de dependencias para cmd/qgo.",
    },
    {
        "name": "cmd/go-openssl",
        "relative_dir": "cmd/go-openssl",
        "prompt": "Deseas preparar dependencias para cmd/go-openssl? (s/n): ",
        "skip_message": "INFO: Omitiendo la preparación opcional de dependencias para cmd/go-openssl.",
    },
]

CORE_TEST_PACKAGES = [
    {"name": "logger", "relative_dir": "logger", "test_path": "./...", "coverage": DEFAULT_COVERAGE_FILE},
    {"name": "encrypt", "relative_dir": "encrypt", "test_path": "./...", "coverage": DEFAULT_COVERAGE_FILE},
    {"name": "security", "relative_dir": "security", "test_path": "./...", "coverage": DEFAULT_COVERAGE_FILE},
    {"name": "root", "relative_dir": ".", "test_path": "./...", "coverage": DEFAULT_COVERAGE_FILE},
]

OPTIONAL_TEST_PACKAGES = [
    {
        "name": "cmd/qgo",
        "relative_dir": "cmd/qgo",
        "test_path": "./...",
        "coverage": DEFAULT_COVERAGE_FILE,
        "prompt": "Deseas ejecutar las pruebas opcionales de cmd/qgo? (s/n): ",
        "run_message": ">>> EJECUTANDO PRUEBAS OPCIONALES PARA cmd/qgo <<<",
        "skip_message": "INFO: Omitiendo la Fase 3 (Pruebas de cmd/qgo) según instrucción del usuario.",
        "warning_message": "WARNING: Las pruebas opcionales de cmd/qgo fallaron. Revisar el error anterior.",
    },
    {
        "name": "cmd/go-openssl",
        "relative_dir": "cmd/go-openssl",
        "test_path": "./...",
        "coverage": DEFAULT_COVERAGE_FILE,
        "prompt": "Deseas ejecutar las pruebas opcionales de cmd/go-openssl? (s/n): ",
        "run_message": ">>> EJECUTANDO PRUEBAS OPCIONALES PARA cmd/go-openssl <<<",
        "skip_message": "INFO: Omitiendo la Fase 3 (Pruebas de cmd/go-openssl) según instrucción del usuario.",
        "warning_message": "WARNING: Las pruebas opcionales de cmd/go-openssl fallaron. Revisar el error anterior.",
    },
]


def project_path(relative_dir):
    """Construye una ruta absoluta dentro de la raíz del proyecto."""
    return PROJECT_ROOT if relative_dir == "." else os.path.join(PROJECT_ROOT, relative_dir)


def prompt_yes_no(message, default="n"):
    """Solicita confirmación interactiva y devuelve True si la respuesta es 's'."""
    try:
        return input(message).lower() == "s"
    except EOFError:
        return default == "s"
    except Exception:
        return default == "s"


def print_manual_intervention(message):
    """Muestra un bloque estándar para confirmaciones manuales."""
    print("\n************************************************************************")
    print("ATENCION: MANUAL INTERVENCION REQUERIDA")
    print("************************************************************************")
    print(message)


def run_command(command, cwd=None, capture_output=True, shell=False):
    """Helper function to run shell commands safely."""
    print(f"--- EJECUTANDO COMANDO: {' '.join(command) if not shell else command} ---")
    try:
        if shell:
            result = subprocess.run(command, cwd=cwd, shell=True, check=True, capture_output=capture_output)
        else:
            result = subprocess.run(command, cwd=cwd, check=True, capture_output=capture_output)

        print("--- COMANDO EJECUTADO EXITOSAMENTE ---")
        if result.stdout:
            print(f"STDOUT:\n{result.stdout.decode()}")
        if result.stderr and result.stderr.decode().strip():
            print(f"STDERR (Advertencias/Errores): {result.stderr.decode()}")
        return True, result
    except subprocess.CalledProcessError as e:
        print(f"!!! FALLO AL EJECUTAR COMANDO (Código {e.returncode}) !!!")
        print(f"STDOUT:\n{e.stdout.decode()}")
        print(f"STDERR:\n{e.stderr.decode()}")
        return False, e
    except FileNotFoundError:
        print("!!! ERROR: Comando o herramienta no encontrada. ¿Está 'go' en el PATH?")
        return False, None


def prepare_module_dependencies(module_name, module_dir):
    """Prepara dependencias Go para un modulo especifico."""
    print(f"\n>>> PREPARANDO DEPENDENCIAS PARA: {module_name} ({module_dir}) <<<")

    go_sum_path = os.path.join(module_dir, "go.sum")
    if os.path.exists(go_sum_path):
        print("Detectado go.sum. Ejecutando 'go mod download' para asegurar dependencias.")
        return run_command(["go", "mod", "download"], cwd=module_dir)

    print("No encontrado go.sum. Intentando con 'go mod tidy' por defecto.")
    return run_command(["go", "mod", "tidy"], cwd=module_dir)


def run_test_and_capture(test_path, coverage_file, cwd=None):
    """Ejecuta go test y captura la salida de cobertura."""
    location = cwd if cwd else CURRENT_DIR
    print(f"\n>>> INICIANDO PRUEBAS PARA: {test_path} ({location}) <<<")
    command = ["go", "test", "-v", "-cover", "-covermode=atomic", f"-coverprofile={coverage_file}", test_path]
    success, result = run_command(command, cwd=cwd)

    if success:
        print(f"Las pruebas de {test_path} terminaron. Reporte guardado en {os.path.join(location, coverage_file)}")
    else:
        print(f"FALLO: Las pruebas para {test_path} fallaron. Revisar el error anterior.")
    return success, result


def merge_coverage_files(coverage_files, output_file):
    """Consolida multiples archivos de cobertura Go en uno solo."""
    valid_files = [path for path in coverage_files if os.path.exists(path)]
    if not valid_files:
        print("No se encontraron archivos de cobertura para consolidar.")
        return False

    with open(output_file, "w", encoding="utf-8") as merged:
        merged.write("mode: atomic\n")
        for path in valid_files:
            with open(path, "r", encoding="utf-8") as source:
                lines = source.readlines()
            if not lines:
                continue
            for line in lines[1:]:
                merged.write(line)
    return True


def write_text_report(result, output_file):
    """Guarda el resumen de cobertura en un archivo .txt."""
    if result is None or not getattr(result, "stdout", None):
        print("No hubo salida de cobertura para guardar en el reporte .txt.")
        return False

    with open(output_file, "w", encoding="utf-8") as report:
        report.write(result.stdout.decode())
    return True


def prepare_required_modules(modules):
    """Ejecuta la preparación de dependencias para módulos obligatorios."""
    setup_successful = True
    for module in modules:
        success, _ = prepare_module_dependencies(module["name"], project_path(module["relative_dir"]))
        if not success:
            setup_successful = False
    return setup_successful


def prepare_optional_modules(modules):
    """Ejecuta la preparación opcional de dependencias según confirmación del usuario."""
    results = []
    for module in modules:
        print_manual_intervention(f"Por favor, confirma si deseas preparar dependencias para '{module['name']}'.")
        ran = prompt_yes_no(module["prompt"])
        successful = False

        if ran:
            successful, _ = prepare_module_dependencies(module["name"], project_path(module["relative_dir"]))
        else:
            print(f"\n{module['skip_message']}")

        results.append({"name": module["name"], "ran": ran, "successful": successful})
    return results


def run_core_package_tests(packages):
    """Ejecuta pruebas para paquetes core."""
    all_core_tests_successful = True
    for package in packages:
        success, _ = run_test_and_capture(
            package["test_path"],
            package["coverage"],
            cwd=project_path(package["relative_dir"]),
        )
        if not success:
            all_core_tests_successful = False
    return all_core_tests_successful


def run_optional_package_tests(packages):
    """Ejecuta pruebas opcionales según confirmación del usuario."""
    results = []
    for package in packages:
        print_manual_intervention(f"Por favor, confirma si deseas ejecutar las pruebas opcionales para '{package['name']}'.")
        ran = prompt_yes_no(package["prompt"])
        successful = False

        if ran:
            print(f"\n{package['run_message']}")
            successful, _ = run_test_and_capture(
                package["test_path"],
                package["coverage"],
                cwd=project_path(package["relative_dir"]),
            )
            if not successful:
                print(package["warning_message"])
        else:
            print(f"\n{package['skip_message']}")

        results.append({"name": package["name"], "ran": ran, "successful": successful})
    return results


def build_coverage_files(core_packages, optional_packages, optional_results):
    """Determina los archivos de cobertura a consolidar."""
    coverage_files = [
        os.path.join(project_path(package["relative_dir"]), package["coverage"])
        for package in core_packages
    ]

    optional_results_by_name = {result["name"]: result for result in optional_results}
    for package in optional_packages:
        result = optional_results_by_name.get(package["name"])
        if result and result["ran"] and result["successful"]:
            coverage_files.append(os.path.join(project_path(package["relative_dir"]), package["coverage"]))

    return coverage_files


def main():
    print("===================================================================================")
    print("       INICIO DEL PROTOCOLO AUTOMATIZADO DE VALIDACIÓN DE CÓDIGO (Go)      ")
    print("===================================================================================")

    global CURRENT_DIR
    CURRENT_DIR = PROJECT_ROOT

    print("\n\n=======================================================================")
    print("FASE 1: PREPARACIÓN DE DEPENDENCIAS (SETUP)")

    os.chdir(CURRENT_DIR)

    setup_successful = prepare_required_modules(REQUIRED_SETUP_MODULES)
    optional_setup_results = prepare_optional_modules(OPTIONAL_SETUP_MODULES)
    if any(result["ran"] and not result["successful"] for result in optional_setup_results):
        setup_successful = False

    print("\n\n=============================================================================")
    print("FASE 2: PRUEBAS UNITARIAS CORE (Testing Core)")

    all_core_tests_successful = run_core_package_tests(CORE_TEST_PACKAGES)

    print("\n\n=============================================================================")
    print("PAUSA DE EJECUCIÓN: CHECKPOINT")
    print("=============================================================================")
    print("Hemos completado las pruebas de los paquetes CORE (logger, encrypt, security, raíz).")

    optional_test_results = run_optional_package_tests(OPTIONAL_TEST_PACKAGES)

    print("\n\n===================================================================================")
    print("FASE 4: GENERACIÓN DEL REPORTE CONSOLIDADO DE COBERTURA")
    print("===================================================================================")

    all_coverage_files = build_coverage_files(CORE_TEST_PACKAGES, OPTIONAL_TEST_PACKAGES, optional_test_results)

    if not all_coverage_files:
        print("No se encontraron archivos de cobertura para generar un reporte.")
    else:
        print("Consolidando todos los reportes de cobertura disponibles...")

        merged_coverage_file = os.path.join(CURRENT_DIR, "coverage.out.total")
        text_report_file = os.path.join(CURRENT_DIR, "coverage_report.txt")
        if not merge_coverage_files(all_coverage_files, merged_coverage_file):
            print("ALERTA: No se pudo consolidar el reporte de cobertura.")
            return

        report_command = ["go", "tool", "cover", f"-func={merged_coverage_file}"]

        print(f"Ejecutando: {report_command}")
        success, result = run_command(report_command)

        if success:
            if write_text_report(result, text_report_file):
                print(f"Reporte de cobertura en texto guardado en {text_report_file}")

            print("\n===================================================================================")
            if setup_successful and all_core_tests_successful and all(
                not result["ran"] or result["successful"] for result in optional_test_results
            ):
                print("PROTOCOLO COMPLETADO EXITOSAMENTE")
            else:
                print("PROTOCOLO COMPLETADO CON OBSERVACIONES")
            print("===================================================================================")
            print("El reporte final de cobertura fue generado y mostrado arriba.")
        else:
            print("\n===================================================================================")
            print("ALERTA: Falló la generación del reporte final.")
            print("===================================================================================")
            print("Esto indica que la compilación o el testeo falló, y el reporte de cobertura no se pudo generar.")
            return 1

    if not setup_successful:
        return 1
    if not all_core_tests_successful:
        return 1
    if any(result["ran"] and not result["successful"] for result in optional_test_results):
        return 1
    if any(result["ran"] and not result["successful"] for result in optional_setup_results):
        return 1
    return 0


# Este comentario final muestra al usuario cómo ejecutarlo desde la raíz del proyecto.
# ----------------------------------------------------------------------------------------------------
# INSTRUCCIÓN DE EJECUCIÓN FINAL:
# Para ejecutar este protocolo completo, navega al directorio raíz del proyecto y ejecuta:
# python ./.scripts/test/main.py
# ====================================================================================================
if __name__ == "__main__":
    raise SystemExit(main())
