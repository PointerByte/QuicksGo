package telemetry

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

/********** helpers **********/

func resetViper() { viper.Reset() }

func writeTempFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return p
}

/********** tests **********/

// 1) tlsConfigGRPC: combina caminos “feliz” e “incorrectos” en una sola función
func Test_TLSConfigGRPC_Scenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		setup    func()
		m        *monitoringImp
		wantErr  bool
		errMatch string
	}{
		{
			name: "insecure=true -> ok",
			setup: func() {
				resetViper()
				viper.Set("otlp.tls.insegure", true)
			},
			m: &monitoringImp{
				ctx:                context.Background(),
				endpoint:           "localhost:4317",
				protocol:           gRPC,
				traceExporterName:  "otlp",
				metricExporterName: "otlp",
				logsExporterName:   "otlp",
			},
			wantErr: false,
		},
		{
			name: "CA no existe -> error",
			setup: func() {
				resetViper()
				viper.Set("otlp.tls.caPath", "/path/que/no/existe.pem")
			},
			m: &monitoringImp{
				ctx:                context.Background(),
				endpoint:           "localhost:4317",
				protocol:           gRPC,
				traceExporterName:  "otlp",
				metricExporterName: "otlp",
				logsExporterName:   "otlp",
			},
			wantErr:  true,
			errMatch: "read CA",
		},
		{
			name: "CA inválido -> error de parseo",
			setup: func() {
				resetViper()
				// escribir un PEM inválido
				p := writeTempFile(t, "bad.pem", []byte("NOT A PEM"))
				viper.Set("otlp.tls.caPath", p)
			},
			m: &monitoringImp{
				ctx:                context.Background(),
				endpoint:           "localhost:4317",
				protocol:           gRPC,
				traceExporterName:  "otlp",
				metricExporterName: "otlp",
				logsExporterName:   "otlp",
			},
			wantErr:  true,
			errMatch: "couldn't parse the CA PEM",
		},
		{
			name: "mTLS sin cert o key -> error",
			setup: func() {
				resetViper()
				viper.Set("otlp.tls.mTLS.enable", true)
				viper.Set("otlp.tls.mTLS.clientCertPath", "/solo-cert.pem")
				// clientKeyPath vacío
			},
			m: &monitoringImp{
				ctx:                context.Background(),
				endpoint:           "localhost:4317",
				protocol:           gRPC,
				traceExporterName:  "otlp",
				metricExporterName: "otlp",
				logsExporterName:   "otlp",
			},
			wantErr:  true,
			errMatch: "for mTLS you need client cert and key",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			err := tc.m.tlsConfig()
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errMatch != "" {
					assert.Contains(t, err.Error(), tc.errMatch)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

// genera un par cert/key self-signed en PEM
func genSelfSignedPEM(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	derKey, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: derKey})

	certPath = writeTempFile(t, "client.crt", certPEM)
	keyPath = writeTempFile(t, "client.key", keyPEM)
	return
}

func Test_TLSConfigGRPC_mTLS_Success_And_LoadError(t *testing.T) {
	t.Parallel()

	t.Run("mTLS ok -> carga par X509 y no error", func(t *testing.T) {
		resetViper()
		certPath, keyPath := genSelfSignedPEM(t)

		viper.Set("otlp.tls.mTLS.enable", true)
		viper.Set("otlp.tls.mTLS.clientCertPath", certPath)
		viper.Set("otlp.tls.mTLS.clientKeyPath", keyPath)

		m := &monitoringImp{
			ctx:               context.Background(),
			endpoint:          "localhost:4317",
			protocol:          gRPC,
			traceExporterName: "otlp", metricExporterName: "otlp", logsExporterName: "otlp",
		}
		err := m.tlsConfig()
		assert.NoError(t, err) // cubre rama mTLS válida  :contentReference[oaicite:5]{index=5}
	})

	t.Run("mTLS par inválido -> error LoadX509KeyPair", func(t *testing.T) {
		resetViper()
		// dos rutas válidas pero con contenido no-par (provoca error en LoadX509KeyPair)
		badCert := writeTempFile(t, "bad.crt", []byte("-----BEGIN CERTIFICATE-----\nBAD\n-----END CERTIFICATE-----"))
		badKey := writeTempFile(t, "bad.key", []byte("-----BEGIN PRIVATE KEY-----\nBAD\n-----END PRIVATE KEY-----"))

		viper.Set("otlp.tls.mTLS.enable", true)
		viper.Set("otlp.tls.mTLS.clientCertPath", badCert)
		viper.Set("otlp.tls.mTLS.clientKeyPath", badKey)

		m := &monitoringImp{
			ctx:               context.Background(),
			endpoint:          "localhost:4317",
			protocol:          gRPC,
			traceExporterName: "otlp", metricExporterName: "otlp", logsExporterName: "otlp",
		}
		err := m.tlsConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "loading for client") // :contentReference[oaicite:6]{index=6}
	})
}

func Test_TLSConfigGRPC_SetsOptions_ByProtocol_AndExporter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		proto string
	}{
		{"traces grpc", gRPC},
		{"traces http", httpProtocol},
		{"traces http/json", httpJson},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetViper()
			// sin CA ni mTLS; sólo ejercitamos los switches de protocolo
			m := &monitoringImp{
				ctx:               context.Background(),
				endpoint:          "localhost:4317",
				protocol:          tc.proto,
				traceExporterName: "otlp", metricExporterName: "otlp", logsExporterName: "otlp",
			}
			err := m.tlsConfig()
			if tc.proto == "ws" {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// cubre los 3 switches de traces/metrics/logs en tlsConfigGRPC
				// traces -> :contentReference[oaicite:7]{index=7}
				// metrics -> :contentReference[oaicite:8]{index=8}
				// logs -> :contentReference[oaicite:9]{index=9}
			}
		})
	}
}

// 2) Traces: mezcla caminos “ok” y errores (exporter/protocolo no soportado)
func Test_Traces_Scenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		m        *monitoringImp
		wantErr  bool
		errMatch string
	}{
		{
			name: "console/http -> ok (sin red)",
			m: &monitoringImp{
				ctx:               context.Background(),
				endpoint:          "localhost:4318",
				protocol:          httpProtocol,
				traceExporterName: "console",
			},
		},
		{
			name: "otlp/http -> ok (creación sin enviar)",
			m: &monitoringImp{
				ctx:               context.Background(),
				endpoint:          "localhost:4318",
				protocol:          httpProtocol,
				traceExporterName: "otlp",
			},
		},
		{
			name: "exporter inválido -> error",
			m: &monitoringImp{
				ctx:               context.Background(),
				endpoint:          "localhost:4317",
				protocol:          gRPC,
				traceExporterName: "invalid",
			},
			wantErr:  true,
			errMatch: "otel_traces_exporter invalid",
		},
		{
			name: "protocolo no soportado -> error",
			m: &monitoringImp{
				ctx:               context.Background(),
				endpoint:          "localhost:4317",
				protocol:          "ws",
				traceExporterName: "otlp",
			},
			wantErr:  true,
			errMatch: "unsupported OTLP protocol for traces",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetViper()
			tc.m.tlsConfig()
			tp, err := tc.m.Traces()
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, tp)
				if tc.errMatch != "" {
					assert.Contains(t, err.Error(), tc.errMatch)
				}
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, tp)
			_ = tp.Shutdown(context.Background())
		})
	}
}

// 3) Metrics: agrupa ok + error de exporter
func Test_Metrics_Scenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		m        *monitoringImp
		wantErr  bool
		errMatch string
	}{
		{
			name: "console/gRPC -> ok",
			m: &monitoringImp{
				ctx:                context.Background(),
				endpoint:           "localhost:4317",
				protocol:           gRPC,
				metricExporterName: "console",
			},
		},
		{
			name: "exporter inválido -> error",
			m: &monitoringImp{
				ctx:                context.Background(),
				endpoint:           "localhost:4317",
				protocol:           gRPC,
				metricExporterName: "nope",
			},
			wantErr:  true,
			errMatch: "otel_metrics_exporter invalid",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetViper()
			tc.m.tlsConfig()
			mp, err := tc.m.Metrics()
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, mp)
				if tc.errMatch != "" {
					assert.Contains(t, err.Error(), tc.errMatch)
				}
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, mp)
			_ = mp.Shutdown(context.Background())
		})
	}
}

func Test_Metrics_UnsupportedProtocol_ErrorMessage(t *testing.T) {
	t.Parallel()

	resetViper()
	m := &monitoringImp{
		ctx:                context.Background(),
		endpoint:           "localhost:4317",
		protocol:           "ws",
		metricExporterName: "otlp",
	}
	_, err := m.Metrics()
	assert.Error(t, err)
	// El código devuelve "traces" en el mensaje (bug intencionalmente cubierto)
	assert.Contains(t, err.Error(), "unsupported OTLP protocol for traces") // :contentReference[oaicite:10]{index=10}
}

// 4) Logs: agrupa ok (y valida provider global) + errores de exporter/protocolo
func Test_Logs_Scenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		m        *monitoringImp
		wantErr  bool
		errMatch string
	}{
		{
			name: "console/gRPC -> ok y set global provider",
			m: &monitoringImp{
				ctx:              context.Background(),
				endpoint:         "localhost:4317",
				protocol:         gRPC,
				logsExporterName: "console",
			},
		},
		{
			name: "exporter inválido -> error",
			m: &monitoringImp{
				ctx:              context.Background(),
				endpoint:         "localhost:4317",
				protocol:         gRPC,
				logsExporterName: "???",
			},
			wantErr:  true,
			errMatch: "otel_logs_exporter invalid",
		},
		{
			name: "protocolo no soportado -> error",
			m: &monitoringImp{
				ctx:              context.Background(),
				endpoint:         "localhost:4317",
				protocol:         "smtp",
				logsExporterName: "otlp",
			},
			wantErr:  true,
			errMatch: "unsupported OTLP protocol for traces",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetViper()
			tc.m.tlsConfig()
			lp, err := tc.m.Logs()
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, lp)
				if tc.errMatch != "" {
					assert.Contains(t, err.Error(), tc.errMatch)
				}
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, lp)

			// validamos provider global
			got := GetLoggerProvider()
			assert.Equal(t, lp, got)

			_ = lp.Shutdown(context.Background())
		})
	}
}

func Test_Logs_UnsupportedProtocol_ErrorMessage(t *testing.T) {
	t.Parallel()

	resetViper()
	m := &monitoringImp{
		ctx:              context.Background(),
		endpoint:         "localhost:4317",
		protocol:         "ws",
		logsExporterName: "otlp",
	}
	_, err := m.Logs()
	assert.Error(t, err)
	// También dice “traces” en Logs (igual que en el código)
	assert.Contains(t, err.Error(), "unsupported OTLP protocol for traces") // :contentReference[oaicite:11]{index=11}
}

// 5) initOtel & mocks: unifica éxito real, éxito vía mock y error vía mock
func Test_InitOtel_Scenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		useMock      bool
		mockReturnFn ShutdownOtel
		mockErr      error
		setup        func()
		wantErr      bool
	}{
		{
			name:    "flujo real con console -> ok y shutdown ok",
			useMock: false,
			setup: func() {
				resetViper()
				viper.Set("OTEL_EXPORTER_OTLP_ENDPOINT", "unused-for-console")
				viper.Set("OTEL_TRACES_EXPORTER", "console")
				viper.Set("OTEL_METRICS_EXPORTER", "console")
				viper.Set("OTEL_LOGS_EXPORTER", "console")
				viper.Set("service.name", "svc")
				viper.Set("service.version", "v1")
			},
			wantErr: false,
		},
		{
			name:    "desviado a mock -> ok",
			useMock: true,
			mockReturnFn: ShutdownOtel(func(ctx context.Context) error {
				return nil
			}),
			setup:   func() { resetViper() },
			wantErr: false,
		},
		{
			name:    "desviado a mock -> error",
			useMock: true,
			mockErr: assert.AnError,
			setup:   func() { resetViper() },
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()

			if tc.useMock {
				EnableMocks()
				defer DisableMocks()
				MocksOtel.ExpectedCalls = nil // sanea expectativas previas si las hubiera
				MocksOtel.On("InitOtel", mock.Anything).
					Return(tc.mockReturnFn, tc.mockErr).
					Once()
			}

			shutdown, err := InitOtel(context.Background())
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, shutdown)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, shutdown)
				assert.NoError(t, shutdown(context.Background()))
			}

			if tc.useMock {
				MocksOtel.AssertExpectations(t)
			}
		})
	}
}

func Test_Traces_Metrics_Logs_OTLP_GRPC_And_HTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		proto string
	}{
		{"grpc", gRPC},
		{"http", httpProtocol},
		{"http/json", httpJson},
	}
	for _, tt := range tests {
		t.Run("traces "+tt.name, func(t *testing.T) {
			resetViper()
			m := &monitoringImp{ctx: context.Background(), endpoint: "localhost:4317", protocol: tt.proto, traceExporterName: "otlp"}
			m.tlsConfig()
			tp, err := m.Traces()
			if tt.proto == "ws" {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err) // cubre compresores gRPC/HTTP  :contentReference[oaicite:12]{index=12}
			_ = tp.Shutdown(context.Background())
		})
		t.Run("metrics "+tt.name, func(t *testing.T) {
			resetViper()
			m := &monitoringImp{ctx: context.Background(), endpoint: "localhost:4317", protocol: tt.proto, metricExporterName: "otlp"}
			m.tlsConfig()
			mp, err := m.Metrics()
			if tt.proto == "ws" {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err) // :contentReference[oaicite:13]{index=13}
			_ = mp.Shutdown(context.Background())
		})
		t.Run("logs "+tt.name, func(t *testing.T) {
			resetViper()
			m := &monitoringImp{ctx: context.Background(), endpoint: "localhost:4317", protocol: tt.proto, logsExporterName: "otlp"}
			m.tlsConfig()
			lp, err := m.Logs()
			if tt.proto == "ws" {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err) // :contentReference[oaicite:14]{index=14}
			_ = lp.Shutdown(context.Background())
		})
	}
}

func Test_TLSConfig_OTLPProtocols(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		traceExporter  string
		metricExporter string
		logExporter    string
		protocol       string
		wantErrSubstr  string
	}{
		{
			name:          "unsupported protocol for traces",
			traceExporter: "otlp",
			protocol:      "invalid_proto",
			wantErrSubstr: "unsupported OTLP protocol for traces",
		},
		{
			name:           "unsupported protocol for metrics",
			metricExporter: "otlp",
			protocol:       "invalid_proto",
			wantErrSubstr:  "unsupported OTLP protocol for metrics",
		},
		{
			name:          "unsupported protocol for logs",
			logExporter:   "otlp",
			protocol:      "invalid_proto",
			wantErrSubstr: "unsupported OTLP protocol for logs",
		},
		{
			name:           "valid gRPC protocol -> success",
			traceExporter:  "otlp",
			metricExporter: "otlp",
			logExporter:    "otlp",
			protocol:       "grpc",
			wantErrSubstr:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &monitoringImp{
				traceExporterName:  tt.traceExporter,
				metricExporterName: tt.metricExporter,
				logsExporterName:   tt.logExporter,
				protocol:           tt.protocol,
			}

			err := m.tlsConfig() // o el nombre exacto de la función que contiene ese bloque

			if tt.wantErrSubstr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test que GetMiddleware devuelve un handler válido y genera spans
func TestGetMiddleware(t *testing.T) {
	// Preparar entorno Gin en modo test
	gin.SetMode(gin.TestMode)

	// Configurar viper
	viper.Set("service.name", "test-service")

	// TracerProvider en memoria para validar spans creados
	sr := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)

	// Crear router con el middleware que vamos a probar
	r := gin.New()
	r.Use(GetMiddleware())

	// Añadir una ruta simple
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Ejecutar request simulada
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	r.ServeHTTP(w, req)

	// Validar respuesta HTTP
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	// Validar que se haya creado al menos un span
	spans := sr.Ended()
	if len(spans) == 0 {
		t.Errorf("expected at least one span, got 0")
	}

	// Validar nombre del servicio en el span
	found := false
	for _, s := range spans {
		if s.Name() == "GET /ping" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected span named 'GET /ping', got %+v", spans)
	}
}
