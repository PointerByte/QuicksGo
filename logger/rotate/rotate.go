package rotate

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type RotatorCfg struct {
	DateFormat string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

func NewRotatorCfgFromViper() *RotatorCfg {
	return &RotatorCfg{
		DateFormat: viper.GetString("dateFormat"),
		MaxSizeMB:  viper.GetInt("MaxSizeMB"),
		MaxBackups: viper.GetInt("MaxBackups"),
		MaxAgeDays: viper.GetInt("MaxAgeDays"),
		Compress:   viper.GetBool("Compress"),
	}
}

// Start arranca la goroutine que revisa el archivo cada minuto.
// logPath es la ruta COMPLETA del archivo (ej: "/var/log/myapp/app.log")
func (cfg *RotatorCfg) Start(ctx context.Context, logPath string) {
	// Skip rotation in test mode
	if gin.Mode() == gin.TestMode {
		return
	}
	if cfg.DateFormat == "" {
		cfg.DateFormat = "2006-01-02T15:04:05.000Z"
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 20
	}
	ticker := time.NewTicker(time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = cfg.checkAndRotate(logPath)
			}
		}
	}()
}

func (cfg *RotatorCfg) checkAndRotate(logPath string) error {
	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	maxBytes := int64(cfg.MaxSizeMB) * 1024 * 1024
	if info.Size() < maxBytes {
		return cfg.cleanupBackups(logPath)
	}

	dir := filepath.Dir(logPath)
	filename := filepath.Base(logPath)
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	rotatedName := fmt.Sprintf("%s-%s%s", base, time.Now().Format(cfg.DateFormat), ext)
	rotatedPath := filepath.Join(dir, rotatedName)

	// Rotación: rename o copy+truncate si falla
	if err := os.Rename(logPath, rotatedPath); err != nil {
		if err := cfg.copyFile(logPath, rotatedPath); err != nil {
			return fmt.Errorf("rotate: rename failed and copy failed: %w", err)
		}
		if err := os.Truncate(logPath, 0); err != nil {
			return fmt.Errorf("rotate: truncate failed: %w", err)
		}
	} else {
		// Re-crear archivo vacío para seguir escribiendo
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644); err == nil {
			_ = f.Close()
		} else {
			return fmt.Errorf("rotate: recreate empty log failed: %w", err)
		}
	}

	return cfg.cleanupBackups(logPath)
}

func (cfg *RotatorCfg) cleanupBackups(logPath string) error {
	dir := filepath.Dir(logPath)
	filename := filepath.Base(logPath)
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	// Buscar backups: base-*.ext
	matches, err := filepath.Glob(filepath.Join(dir, base+"-*"+ext))
	if err != nil {
		return err
	}

	// Filtro por antigüedad
	if cfg.MaxAgeDays > 0 {
		cutoff := time.Now().Add(-time.Duration(cfg.MaxAgeDays) * 24 * time.Hour)
		var kept []string
		for _, p := range matches {
			if fi, e := os.Stat(p); e == nil && fi.ModTime().After(cutoff) {
				kept = append(kept, p)
			} else if e == nil {
				_ = os.Remove(p)
			}
		}
		matches = kept
	}

	// Orden por mtime asc (antiguos primero)
	sort.Slice(matches, func(i, j int) bool {
		ii, _ := os.Stat(matches[i])
		jj, _ := os.Stat(matches[j])
		if ii == nil || jj == nil {
			return matches[i] < matches[j]
		}
		return ii.ModTime().Before(jj.ModTime())
	})

	// Aplicar MaxBackups
	if cfg.MaxBackups == 0 && len(matches) > 0 {
		return cfg.deleteOrZip(dir, base, matches)
	}
	if len(matches) > cfg.MaxBackups {
		excess := matches[:len(matches)-cfg.MaxBackups]
		return cfg.deleteOrZip(dir, base, excess)
	}
	return nil
}

func (cfg *RotatorCfg) deleteOrZip(dir, base string, files []string) error {
	if len(files) == 0 {
		return nil
	}
	if cfg.Compress {
		zipName := fmt.Sprintf("%s-archive-%s.zip", base, time.Now().Format("20060102-150405"))
		zipPath := filepath.Join(dir, zipName)
		if err := cfg.zipFiles(zipPath, files); err != nil {
			return err
		}
		for _, f := range files {
			_ = os.Remove(f)
		}
		return nil
	}
	for _, f := range files {
		_ = os.Remove(f)
	}
	return nil
}

func (cfg *RotatorCfg) zipFiles(zipPath string, files []string) error {
	zf, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	defer zw.Close()

	for _, f := range files {
		if err := cfg.addToZip(zw, f); err != nil {
			return err
		}
	}
	return nil
}

func (cfg *RotatorCfg) addToZip(zw *zip.Writer, path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	h, err := zip.FileInfoHeader(fi)
	if err != nil {
		return err
	}
	h.Name = filepath.Base(path)
	h.Method = zip.Deflate

	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}

	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	_, err = io.Copy(w, src)
	return err
}

func (cfg *RotatorCfg) copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
