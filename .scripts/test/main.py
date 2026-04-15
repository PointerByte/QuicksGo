# Copyright 2026 PointerByte Contributors
# SPDX-License-Identifier: Apache-2.0

import os
import subprocess

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
             # Mostrar stderr solo si es útil o si el comando falló ligeramente
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

def run_test_and_capture(test_path, coverage_file, cwd=None):
    """Ejecuta go test y captura la salida de cobertura."""
    location = cwd if cwd else CURRENT_DIR
    print(f"\n>>> INICIANDO PRUEBAS PARA: {test_path} ({location}) <<<")
    # Intentamos forzar el testeo en el path específico
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

def main():
    print("===================================================================================")
    print("       INICIO DEL PROTOCOLO AUTOMATIZADO DE VALIDACIÓN DE CÓDIGO (Go)      ")
    print("===================================================================================")
    
    # Cambiar el directorio de trabajo a la raíz del proyecto
    global CURRENT_DIR
    CURRENT_DIR = "/run/media/mblanco/Multimedia/Proyects/Practices/QuicksGo"
    
    # ================================================================================
    # FASE 1: Preparación de Dependencias (Setup - Lógica Condicional)
    # =============================================================================
    print("\n\n=======================================================================")
    print("FASE 1: PREPARACIÓN DE DEPENDENCIAS (SETUP)")
    
    # Ejecutamos todo el script en el directorio deseado
    os.chdir(CURRENT_DIR)
    
    has_gomodsum = os.path.exists("go.sum")
    
    if has_gomodsum:
        print("Detectado go.sum. Ejecutando 'go mod download' para asegurar dependencias.")
        success, _ = run_command(["go", "mod", "download"])
    else:
        print("No encontrado go.sum. Intentando con 'go mod tidy' por defecto.")
        success, _ = run_command(["go", "mod", "tidy"])
    setup_successful = success

    
    # ========================================================================
    # FASE 2: Pruebas Unitarias Core (Testing Core) - Orden Específico
    # =========================================================================
    print("\n\n=============================================================================")
    print("FASE 2: PRUEBAS UNITARIAS CORE (Testing Core)")
    
    # Definición de paquetes críticos y sus respectivos archivos de cobertura
    core_packages = [
        {"name": "logger", "cwd": os.path.join(CURRENT_DIR, "logger"), "test_path": "./...", "coverage": "coverage.out"},
        {"name": "encrypt", "cwd": os.path.join(CURRENT_DIR, "encrypt"), "test_path": "./...", "coverage": "coverage.out"},
        {"name": "security", "cwd": os.path.join(CURRENT_DIR, "security"), "test_path": "./...", "coverage": "coverage.out"},
        {"name": "root", "cwd": CURRENT_DIR, "test_path": "./...", "coverage": "coverage.out"},
    ]
    
    all_core_tests_successful = True
    for package in core_packages:
        success, _ = run_test_and_capture(package["test_path"], package["coverage"], cwd=package["cwd"])
        if not success:
            all_core_tests_successful = False

    # ==============================================================================
    # FASE 3: Testeo Opcional de Paquetes Adicionales (Checkpoint)
    # ==========================================================================
    print("\n\n=============================================================================")
    print("PAUSA DE EJECUCIÓN: CHECKPOINT")
    print("=============================================================================")
    print("Hemos completado las pruebas de los paquetes CORE (logger, encrypt, security, raíz).")
    
    print("\n************************************************************************")
    print("ATENCION: MANUAL INTERVENCION REQUERIDA")
    print("************************************************************************")
    print("Por favor, confirma si deseas ejecutar las pruebas opcionales para 'cmd/qgo'.")
    
    # Esta parte DEBE ser interactiva para que el usuario decida.
    try:
        user_input = input("Deseas ejecutar las pruebas opcionales de cmd/qgo? (s/n): ").lower()
    except EOFError:
        user_input = 'n' # Asume 'n' si la entrada no está disponible
    except Exception:
        user_input = 'n'
    
    optional_test_ran = False
    optional_test_successful = False
    if user_input == 's':
        optional_test_ran = True
        optional_test_successful = True
        print("\n>>> EJECUTANDO PRUEBAS OPCIONALES PARA cmd/qgo <<<")
        # Ejecutar la prueba del paquete opcional
        success, _ = run_test_and_capture("./...", "coverage.out", cwd=os.path.join(CURRENT_DIR, "cmd/qgo"))
        if not success:
            print("WARNING: Las pruebas opcionales de cmd/qgo fallaron. Revisar el error anterior.")
            optional_test_successful = False
    else:
        print("\nINFO: Omitiendo la Fase 3 (Pruebas de cmd/qgo) según instrucción del usuario.")


    # =======================================================================
    # FASE 4: Reporte Consolidado (Reporting)
    # ====================================================================================
    print("\n\n===================================================================================")
    print("FASE 4: GENERACIÓN DEL REPORTE CONSOLIDADO DE COBERTURA")
    print("===================================================================================")
    
    # Determinación de archivos de cobertura para el reporte
    all_coverage_files = [
        os.path.join(CURRENT_DIR, "logger", "coverage.out"),
        os.path.join(CURRENT_DIR, "encrypt", "coverage.out"),
        os.path.join(CURRENT_DIR, "security", "coverage.out"),
        os.path.join(CURRENT_DIR, "coverage.out"),
    ]
    
    if optional_test_ran and optional_test_successful:
        all_coverage_files.append(os.path.join(CURRENT_DIR, "cmd/qgo", "coverage.out"))
    
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
            if setup_successful and all_core_tests_successful and (not optional_test_ran or optional_test_successful):
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
    if optional_test_ran and not optional_test_successful:
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
