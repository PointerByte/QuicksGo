package logger

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func Test_initLogger(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		setupViper func()
		wantErr    bool
		afterTest  func()
	}{
		{
			name: "success",
			setupViper: func() {
				viper.Set("logger.dir", tmpDir)
				viper.Set("service.name", "success_service")
			},
			wantErr: false,
			afterTest: func() {
				// Close file to allow Windows cleanup
				if f, ok := viper.Get("fileLog").(*os.File); ok {
					f.Close()
				}
			},
		},
		{
			name: "mkdir fails",
			setupViper: func() {
				viper.Set("logger.dir", string([]byte{0})) // Invalid dir
				viper.Set("service.name", "mkdir_fail")
			},
			wantErr:   true,
			afterTest: func() {},
		},
		{
			name: "file open fails",
			setupViper: func() {
				viper.Set("logger.dir", tmpDir)
				viper.Set("service.name", string([]byte{0})) // Invalid filename
			},
			wantErr:   true,
			afterTest: func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupViper()
			err := initLogger()
			if (err != nil) != tt.wantErr {
				t.Errorf("initLogger() error = %v, wantErr = %v", err, tt.wantErr)
			}
			tt.afterTest()
		})
	}
}
