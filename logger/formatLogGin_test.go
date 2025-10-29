package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
)

var mux sync.Mutex

// helper para ejecutar una request y capturar el log producido por gin.LoggerWithFormatter
func performRequest(t *testing.T, r *gin.Engine, method, path string) (status int, body string, logs string) {
	t.Helper()

	mux.Lock()
	defer mux.Unlock()
	// Capturamos la salida del logger de Gin.
	var buf bytes.Buffer
	origWriter := gin.DefaultWriter
	gin.DefaultWriter = &buf
	defer func() { gin.DefaultWriter = origWriter }()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)

	resBodyBytes, _ := io.ReadAll(w.Result().Body)
	return w.Code, string(resBodyBytes), buf.String()
}

// Parseamos la última línea JSON del buffer de logs (el formateador retorna una sola línea por request)
func lastJSONLine(t *testing.T, logs string) map[string]any {
	t.Helper()
	logs = strings.TrimSpace(logs)
	if logs == "" {
		return nil
	}
	lines := strings.Split(logs, "\n")
	raw := lines[len(lines)-1]
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("no se pudo parsear JSON del log: %v\nraw: %s", err, raw)
	}
	return m
}

func TestCustomLogFormatGin_DefaultSuccessMessage(t *testing.T) {
	t.Parallel()

	mocksLogger := new(Mocks)
	mocksLogger.On("emitOtel", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	emitOtel = mocksLogger.emitOtel
	defer mocksLogger.AssertExpectations(t)

	viper.Set("logger.dateFormat", time.RFC3339)
	gin.SetMode(gin.TestMode)

	mux.Lock()
	defer mux.Unlock()
	// Captura el writer ANTES de registrar el middleware
	var buf bytes.Buffer
	origWriter := gin.DefaultWriter
	gin.DefaultWriter = &buf
	defer func() { gin.DefaultWriter = origWriter }()

	r := gin.New()
	r.Use(MiddlewaresInitLogger(), CustomLogFormatGin())

	r.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status inesperado: got=%d want=%d", w.Code, http.StatusNoContent)
	}

	logs := buf.String()
	m := lastJSONLine(t, logs)
	if m == nil {
		t.Fatal("se esperaba una línea de log JSON, pero no hubo salida")
	}

	// claves en minúsculas
	if m["message"] == "" {
		t.Errorf("message no debería estar vacío en success (debería ser msgSuccess)")
	}
	attrs, ok := m["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes debería ser un objeto JSON")
	}
	if attrs["method"] != "GET" {
		t.Errorf("attributes.method got=%v want=GET", attrs["method"])
	}
	if attrs["path"] != "/ok" {
		t.Errorf("attributes.path got=%v want=/ok", attrs["path"])
	}
	if _, ok := m["level"]; !ok {
		t.Errorf("falta campo level en el log")
	}
	if _, ok := m["timestamp"]; !ok {
		t.Errorf("falta campo timestamp en el log")
	}
}

// Añade este helper (reemplaza lastJSONLine si quieres unificar)
func lastNonEmptyLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if ln := strings.TrimSpace(lines[i]); ln != "" {
			return ln
		}
	}
	return ""
}

func TestCustomLogFormatGin_RespectsSetMessageLog(t *testing.T) {
	t.Parallel()

	mocksLogger := new(Mocks)
	mocksLogger.On("emitOtel", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe().Return().Maybe()
	emitOtel = mocksLogger.emitOtel
	defer mocksLogger.AssertExpectations(t)

	viper.Set("logger.dateFormat", time.RFC3339)
	gin.SetMode(gin.TestMode)

	mux.Lock()
	defer mux.Unlock()
	var buf bytes.Buffer
	origWriter := gin.DefaultWriter
	gin.DefaultWriter = &buf
	defer func() { gin.DefaultWriter = origWriter }()

	r := gin.New()
	r.Use(MiddlewaresInitLogger(), CustomLogFormatGin())

	r.GET("/custom", func(c *gin.Context) {
		// Inyectamos atributos personalizados en el *request context*
		extra := map[string]any{
			"userId": "u-123",
			"custom": 42,
			// Sobrescribe la clave base "method"
			"method": "PATCH",
		}
		c.Set(attributesKey, extra)
		SetMessageLog(c, ERROR, "boom!")
		c.String(http.StatusInternalServerError, "err")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status inesperado: got=%d want=%d", w.Code, http.StatusInternalServerError)
	}

	raw := lastNonEmptyLine(buf.String())
	if raw == "" {
		t.Fatalf("no se obtuvo salida de log")
	}

	var entry LogFormat
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("no se pudo parsear LogEntry: %v\nraw: %q", err, raw)
	}

	// Mensaje y nivel vienen de SetMessageLog
	if entry.Message != "boom!" {
		t.Errorf("Message got=%q want=%q", entry.Message, "boom!")
	}
	if entry.Level != ERROR {
		t.Errorf("Level got=%v want=%v", entry.Level, ERROR)
	}

	// Atributos base
	if got := entry.Attributes["path"]; got != "/custom" {
		t.Errorf("Attributes[path] got=%v want=/custom", got)
	}

	// 🔸 Merge + override verificado
	if got := entry.Attributes["method"]; got != "PATCH" {
		t.Errorf("Attributes[method] got=%v want=PATCH (override)", got)
	}
	if got := entry.Attributes["userId"]; got != "u-123" {
		t.Errorf("Attributes[userId] got=%v want=u-123", got)
	}
	// JSON decodifica números como float64
	if v, ok := entry.Attributes["custom"].(float64); !ok || v != 42 {
		t.Errorf("Attributes[custom] got=%v (type %T) want=42", entry.Attributes["custom"], entry.Attributes["custom"])
	}
}

func TestCustomLogFormatGin_WithAutoLogFalse_DisablesLogging(t *testing.T) {
	t.Parallel()

	mocksLogger := new(Mocks)
	mocksLogger.On("emitOtel", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe().Return().Maybe()
	emitOtel = mocksLogger.emitOtel
	defer mocksLogger.AssertExpectations(t)

	viper.Set("logger.dateFormat", time.RFC3339)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Marcamos la clave para desactivar el auto-log.
	r.Use(func(c *gin.Context) {
		// La clave que usa el formateador es params.Keys[WithAutoLog]
		// En Gin, ctx.Set escribe en Keys y luego LoggerWithFormatter las recibe en params.Keys.
		c.Set(WithAutoLog, false)
		c.Next()
	})
	r.Use(MiddlewaresInitLogger(), CustomLogFormatGin())

	r.GET("/silent", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	status, _, logs := performRequest(t, r, http.MethodGet, "/silent")
	if status != http.StatusOK {
		t.Fatalf("status inesperado: got=%d want=%d", status, http.StatusOK)
	}
	if strings.TrimSpace(logs) != "" {
		t.Fatalf("no debería haberse emitido log cuando WithAutoLog=false; got: %q", logs)
	}
}

// Útil si quieres seguir trabajando con map
func parseJSONToMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	if raw == "" {
		t.Fatalf("no se obtuvo salida de log")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("no se pudo parsear JSON: %v\nraw: %q", err, raw)
	}
	return m
}

func TestSetMessageLog_SetsKeysInContext(t *testing.T) {
	t.Parallel()

	mocksLogger := new(Mocks)
	mocksLogger.On("emitOtel", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe().Return().Maybe()
	emitOtel = mocksLogger.emitOtel
	defer mocksLogger.AssertExpectations(t)

	viper.Set("logger.dateFormat", time.RFC3339)
	gin.SetMode(gin.TestMode)

	mux.Lock()
	defer mux.Unlock()
	// 1) Captura el writer ANTES del middleware
	var buf bytes.Buffer
	origWriter := gin.DefaultWriter
	gin.DefaultWriter = &buf
	defer func() { gin.DefaultWriter = origWriter }()

	r := gin.New()
	r.Use(MiddlewaresInitLogger(), CustomLogFormatGin())

	r.GET("/keys", func(c *gin.Context) {
		SetMessageLog(c, UNKNOWN, "hola")
		// Validación directa en ctx.Keys
		if v, ok := c.Get(messageKey); !ok || v.(string) != "hola" {
			t.Fatalf(`ctx.Keys["message"] no seteado correctamente; got=%v ok=%v`, v, ok)
		}
		if v, ok := c.Get(LevelKey); !ok || v.(Level) != UNKNOWN {
			t.Fatalf(`ctx.Keys["level"] no seteado correctamente; got=%v ok=%v`, v, ok)
		}
		c.Status(http.StatusOK)
	})

	// Ejecuta la request
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/keys", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status inesperado: got=%d want=%d", w.Code, http.StatusOK)
	}

	// 2) Toma la última línea NO vacía y parsea
	raw := lastNonEmptyLine(buf.String())
	// Puedes usar map...
	m := parseJSONToMap(t, raw)

	// ...o si prefieres LogEntry, asegúrate de tener tags json en minúsculas en el struct.
	// var entry LogEntry
	// if err := json.Unmarshal([]byte(raw), &entry); err != nil {
	// 	t.Fatalf("no se pudo parsear LogEntry: %v\nraw: %q", err, raw)
	// }

	// 3) Asserts — OJO: claves en minúsculas
	if m["message"] != "hola" {
		t.Errorf("message got=%v want=%v", m["message"], "hola")
	}
	if m["level"] != "UNKNOWN" && m["level"] != UNKNOWN {
		t.Errorf("level got=%v want=%v", m["level"], "UNKNOWN")
	}
	attrs, ok := m["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes debería ser un objeto JSON")
	}
	if attrs["path"] != "/keys" {
		t.Errorf("attributes[path] got=%v want=/keys", attrs["path"])
	}
	if attrs["method"] != "GET" {
		t.Errorf("attributes[method] got=%v want=GET", attrs["method"])
	}
}

func TestCustomLogFormatGin_DefaultErrorMessage(t *testing.T) {
	t.Parallel()

	viper.Set("logger.dateFormat", time.RFC3339)
	gin.SetMode(gin.TestMode)

	mux.Lock()
	defer mux.Unlock()
	var buf bytes.Buffer
	orig := gin.DefaultWriter
	gin.DefaultWriter = &buf
	defer func() { gin.DefaultWriter = orig }()

	r := gin.New()
	r.Use(MiddlewaresInitLogger(), CustomLogFormatGin())

	r.GET("/fail", func(c *gin.Context) {
		// No seteamos message ni level → debe usar msgError
		c.Status(http.StatusBadRequest)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	r.ServeHTTP(w, req)

	raw := lastNonEmptyLine(buf.String())
	m := parseJSONToMap(t, raw)

	want := string(MsgError)
	if m["message"] != want {
		t.Errorf("se esperaba msgError, got=%v want=%v", m["message"], want)
	}
}
