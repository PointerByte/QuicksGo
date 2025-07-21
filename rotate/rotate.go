package rotate

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Rotator handles log file rotation based on a maximum size.
type Rotator struct {
	dir        string // directory for log files
	filename   string // base filename, e.g., "app.log"
	maxSize    int64  // max size in bytes before rotation
	currentZip string // ruta del ZIP en uso

	mux     sync.Mutex
	entries chan []byte   // canal buffered para datos de log
	done    chan struct{} // señal de cierre
	once    sync.Once     // para cerrar solo una vez
}

// New creates a new Rotator and opens the initial file.
func New(dir, filename string, maxSize int64) (*Rotator, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	r := &Rotator{
		dir:      dir,
		filename: filename,
		maxSize:  maxSize,
	}
	go r.background()
	return r, nil
}

// Write implements io.Writer and rotates file if size exceeds maxSize.
func (r *Rotator) Write(p []byte) (n int, err error) {
	buf := make([]byte, len(p))
	copy(buf, p)
	select {
	case r.entries <- buf:
		return len(p), nil
	default:
		// buffer lleno: escribir síncrono
		err = r.syncWrite(buf)
		return len(p), err
	}
}

// Close cierra el canal de entradas y espera a que termine el worker.
func (r *Rotator) Close() {
	r.once.Do(func() {
		close(r.entries)
		<-r.done
	})
}

// background consume entradas y realiza escritura/rotación.
func (r *Rotator) background() {
	for data := range r.entries {
		r.syncWrite(data)
	}
	close(r.done)
}

// syncWrite realiza la lógica de escritura y rotación de forma sincronizada.
func (r *Rotator) syncWrite(p []byte) error {
	r.mux.Lock()
	defer r.mux.Unlock()

	logPath := filepath.Join(r.dir, r.filename)
	// verificar rotación
	if fi, err := os.Stat(logPath); err == nil {
		if fi.Size()+int64(len(p)) > r.maxSize {
			if err := r.rotate(logPath); err != nil {
				return err
			}
		}
	}
	// escribir datos
	f, err := os.OpenFile(logPath,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(p)
	return err
}

// rotate closes the current file and opens a new one with a timestamp suffix.
func (r *Rotator) rotate(logPath string) error {
	// Leer todo el contenido
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nada que rotar
		}
		return err
	}
	if len(data) == 0 {
		return nil // si está vacío, no rotamos
	}

	// inicializar o rotar currentZip según zipSizeLimit
	if r.currentZip == "" {
		r.currentZip = r.newZipPath()
	} else if fi, err := os.Stat(r.currentZip); err == nil {
		if fi.Size()+int64(len(data)) > r.maxSize {
			r.currentZip = r.newZipPath()
		}
	}

	// Añadir al ZIP (creándolo o appending)
	if err := r.appendToZip(r.currentZip, data); err != nil {
		return err
	}

	// Truncar el log original a cero
	return os.Truncate(logPath, 0)
}

// newZipPath genera un nombre nuevo con timestamp completo.
func (r *Rotator) newZipPath() string {
	stamp := time.Now().Format("2006-01-02T15-04-05")
	name := fmt.Sprintf("%s-%s.zip", r.filename, stamp)
	return filepath.Join(r.dir, name)
}

// appendToZip añade data como una nueva entrada timestamped dentro del zip en zipPath.
// appendToZip añade data al ZIP usando funciones helper factorizadas.
func (r *Rotator) appendToZip(zipPath string, data []byte) error {
	if err := ensureDir(filepath.Dir(zipPath)); err != nil {
		return err
	}
	entryName := r.generateEntryName()
	entries, err := r.readEntries(zipPath)
	if err != nil {
		return err
	}
	entries[entryName] = data
	return r.writeZip(zipPath, entries)
}

// ensureDir crea el directorio si es necesario.
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// generateEntryName devuelve el nombre de la entrada basado en timestamp.
func (r *Rotator) generateEntryName() string {
	return time.Now().Format("2006-01-02T15-04-05") + ".log"
}

// readEntries recupera todas las entradas actuales de un ZIP.
func (r *Rotator) readEntries(zipPath string) (map[string][]byte, error) {
	entries := make(map[string][]byte)
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		return entries, nil
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		entries[f.Name] = content
	}
	return entries, nil
}

// writeZip escribe todas las entradas en un ZIP temporal y reemplaza el original.
func (r *Rotator) writeZip(zipPath string, entries map[string][]byte) error {
	tmp := zipPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			zw.Close()
			f.Close()
			return err
		}
		if _, err := w.Write(content); err != nil {
			zw.Close()
			f.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, zipPath)
}
