package rotate

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// --- helpers ---

func newCfg() *RotatorCfg {
	return &RotatorCfg{
		DateFormat: "20060102-150405", // seguro cross-platform
		MaxSizeMB:  1,                 // 1MB para forzar rotación fácil
		MaxBackups: 3,
		MaxAgeDays: 0,
		Compress:   false,
	}
}

func writeNBytes(t *testing.T, path string, n int) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	chunk := make([]byte, 1024)
	written := 0
	for written < n {
		toWrite := chunk
		if len(toWrite) > n-written {
			toWrite = toWrite[:n-written]
		}
		if _, err := f.Write(toWrite); err != nil {
			t.Fatalf("write: %v", err)
		}
		written += len(toWrite)
	}
}

func touchWithMod(t *testing.T, path string, mod time.Time) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		writeNBytes(t, path, 10)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

func listBackups(t *testing.T, dir, base, ext string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, base+"-*"+ext))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}

func TestNewRotatorCfgFromViper_LoadsValues(t *testing.T) {
	t.Parallel()

	// Arrange
	viper.Set("dateFormat", "2006-01-02T15:04:05.000Z")
	viper.Set("MaxSizeMB", 42)
	viper.Set("MaxBackups", 7)
	viper.Set("MaxAgeDays", 9)
	viper.Set("Compress", true)

	// Act
	cfg := NewRotatorCfgFromViper()

	// Assert
	if cfg.DateFormat != "2006-01-02T15:04:05.000Z" {
		t.Fatalf("dateFormat not loaded: %q", cfg.DateFormat)
	}
	if cfg.MaxSizeMB != 42 {
		t.Fatalf("MaxSizeMB not loaded: %d", cfg.MaxSizeMB)
	}
	if cfg.MaxBackups != 7 {
		t.Fatalf("MaxBackups not loaded: %d", cfg.MaxBackups)
	}
	if cfg.MaxAgeDays != 9 {
		t.Fatalf("MaxAgeDays not loaded: %d", cfg.MaxAgeDays)
	}
	if !cfg.Compress {
		t.Fatalf("Compress not loaded: %v", cfg.Compress)
	}
}

func TestStart_AppliesDefaultsAndCancels(t *testing.T) {
	t.Parallel() // Evitar por viper/globales si se usan

	// cfg sin inicializar -> Start debe aplicar defaults
	cfg := &RotatorCfg{
		DateFormat: "",
		MaxSizeMB:  0,
		// el resto nos da igual en este test
	}

	// Contexto cancelable: cancelamos rápido para no esperar al ticker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// No necesitamos un archivo real; Start solo valida defaults y crea la goroutine
	cfg.Start(ctx, "/tmp/dummy/app.log")

	// Dar un micro respiro a la goroutine para entrar y salir por cancel (no esperamos al tick)
	time.Sleep(5 * time.Millisecond)
	cancel() // asegurar cancel

	// Verificar defaults aplicados por Start
	if cfg.DateFormat != "2006-01-02T15:04:05.000Z" {
		t.Fatalf("DateFormat default not applied, got %q", cfg.DateFormat)
	}
	if cfg.MaxSizeMB != 20 {
		t.Fatalf("MaxSizeMB default not applied, got %d", cfg.MaxSizeMB)
	}
}

func TestStart_RespectsExistingValues(t *testing.T) {
	// Si ya hay valores válidos, Start NO debe sobreescribirlos
	cfg := &RotatorCfg{
		DateFormat: "20060102-150405",
		MaxSizeMB:  5,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg.Start(ctx, "/tmp/dummy/app.log")
	time.Sleep(5 * time.Millisecond)
	cancel()

	if cfg.DateFormat != "20060102-150405" {
		t.Fatalf("DateFormat should be preserved, got %q", cfg.DateFormat)
	}
	if cfg.MaxSizeMB != 5 {
		t.Fatalf("MaxSizeMB should be preserved, got %d", cfg.MaxSizeMB)
	}
}

// --- tests ---

func TestCheckAndRotate_NoFile(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	// No existe -> no error
	if err := cfg.checkAndRotate(logPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckAndRotate_RotateBySize_RenamePath(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	// >1MB para forzar rotación
	writeNBytes(t, logPath, 1*1024*1024+1024)

	if err := cfg.checkAndRotate(logPath); err != nil {
		t.Fatalf("rotate error: %v", err)
	}

	// Debe existir nuevo app.log vacío
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat new log: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty new log, got %d", info.Size())
	}

	// Debe existir un backup con patrón base-*.log
	backups := listBackups(t, dir, "app", ".log")
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
}

func TestCleanupBackups_MaxBackups_CompressFalse(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	cfg.MaxBackups = 2
	cfg.Compress = false

	dir := t.TempDir()
	base := "app"
	ext := ".log"
	logPath := filepath.Join(dir, base+ext)
	writeNBytes(t, logPath, 10)

	// Crear 5 backups con mtimes diferentes
	now := time.Now()
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, base+"-old"+time.Duration(i).String()+ext)
		writeNBytes(t, name, 10)
		touchWithMod(t, name, now.Add(time.Duration(-i-1)*time.Hour))
	}

	if err := cfg.cleanupBackups(logPath); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	// Deben quedar solo 2 backups y NO zip
	backups := listBackups(t, dir, base, ext)
	if len(backups) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(backups))
	}
	zips, _ := filepath.Glob(filepath.Join(dir, base+"-archive-*.zip"))
	if len(zips) != 0 {
		t.Fatalf("did not expect zip, got %d", len(zips))
	}
}

func TestCleanupBackups_MaxBackups_CompressTrue(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	cfg.MaxBackups = 2
	cfg.Compress = true

	dir := t.TempDir()
	base := "app"
	ext := ".log"
	logPath := filepath.Join(dir, base+ext)
	writeNBytes(t, logPath, 10)

	// Crear 4 backups
	for i := 0; i < 4; i++ {
		name := filepath.Join(dir, base+f("-b%d", i)+ext)
		writeNBytes(t, name, 10)
		touchWithMod(t, name, time.Now().Add(time.Duration(-i-1)*time.Hour))
	}

	if err := cfg.cleanupBackups(logPath); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	// Quedan 2 backups
	backups := listBackups(t, dir, base, ext)
	if len(backups) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(backups))
	}
	// Debe existir un zip
	zips, _ := filepath.Glob(filepath.Join(dir, base+"-archive-*.zip"))
	if len(zips) != 1 {
		t.Fatalf("expected 1 zip, got %d", len(zips))
	}

	// El zip debe contener 2 entradas (los que se eliminaron)
	zr, err := zip.OpenReader(zips[0])
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()
	if len(zr.File) != 2 {
		t.Fatalf("expected 2 files in zip, got %d", len(zr.File))
	}
}

func TestCleanupBackups_MaxAgeDays(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	cfg.MaxBackups = 10
	cfg.MaxAgeDays = 1 // eliminar mayores a 1 día
	cfg.Compress = false

	dir := t.TempDir()
	base := "app"
	ext := ".log"
	logPath := filepath.Join(dir, base+ext)
	writeNBytes(t, logPath, 10)

	// Uno antiguo (3 días) y uno reciente (1 hora)
	old := filepath.Join(dir, base+"-old"+ext)
	recent := filepath.Join(dir, base+"-recent"+ext)
	writeNBytes(t, old, 10)
	writeNBytes(t, recent, 10)
	touchWithMod(t, old, time.Now().Add(-72*time.Hour))
	touchWithMod(t, recent, time.Now().Add(-1*time.Hour))

	if err := cfg.cleanupBackups(logPath); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	backups := listBackups(t, dir, base, ext)
	if len(backups) != 1 || !strings.Contains(backups[0], "recent") {
		t.Fatalf("expected only recent to remain, got %v", backups)
	}
}

func TestDeleteOrZip_EmptyList(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	dir := t.TempDir()
	if err := cfg.deleteOrZip(dir, "app", nil); err != nil {
		t.Fatalf("empty deleteOrZip: %v", err)
	}
}

func TestDeleteOrZip_CompressTrue(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	cfg.Compress = true
	dir := t.TempDir()

	// crear 2 archivos para comprimir
	f1 := filepath.Join(dir, "a.log")
	f2 := filepath.Join(dir, "b.log")
	writeNBytes(t, f1, 10)
	writeNBytes(t, f2, 10)

	if err := cfg.deleteOrZip(dir, "app", []string{f1, f2}); err != nil {
		t.Fatalf("deleteOrZip: %v", err)
	}

	// archivos originales borrados
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Fatalf("expected %s removed", f1)
	}
	if _, err := os.Stat(f2); !os.IsNotExist(err) {
		t.Fatalf("expected %s removed", f2)
	}

	// existe un zip
	zips, _ := filepath.Glob(filepath.Join(dir, "app-archive-*.zip"))
	if len(zips) != 1 {
		t.Fatalf("expected 1 zip, got %d", len(zips))
	}
}

func TestZipFiles_CreateError(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	dir := t.TempDir()

	// zipPath es un directorio existente -> os.Create debe fallar
	subdir := filepath.Join(dir, "as_dir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err := cfg.zipFiles(subdir, []string{})
	if err == nil {
		t.Fatalf("expected error creating zip on a dir path")
	}
}

func TestAddToZip_FileNotExist(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "x.zip")

	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	defer zw.Close()

	if err := cfg.addToZip(zw, filepath.Join(dir, "missing.log")); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestCopyFile_Success(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.log")
	dst := filepath.Join(dir, "dst.log")
	writeNBytes(t, src, 128)

	if err := cfg.copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("dst missing: %v", err)
	}
}

func TestCopyFile_ErrOpenSrc(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst.log")
	if err := cfg.copyFile(filepath.Join(dir, "nope.log"), dst); err == nil {
		t.Fatalf("expected error opening src")
	}
}

func TestCopyFile_ErrOpenDst(t *testing.T) {
	t.Parallel()

	cfg := newCfg()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.log")
	writeNBytes(t, src, 16)

	// usar un directorio como destino para provocar error de OpenFile
	dst := filepath.Join(dir, "as_dir")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := cfg.copyFile(src, dst); err == nil {
		t.Fatalf("expected error opening dst as directory")
	}
}

// Utilidad pequeña para formatear string sin fmt en nombre (zonas seguras race/alloc)
func f(format string, i int) string {
	return strings.ReplaceAll(format, "%d", itoa(i))
}

// itoa minimal sin strconv para evitar imports extra (suficiente para tests)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
