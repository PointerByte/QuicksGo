package models_test

import (
	"reflect"
	"testing"

	"github.com/PointerByte/QuicksGo/models"

	"github.com/spf13/viper"
)

func TestGenericResponse(t *testing.T) {
	type samplePayload struct {
		Message string `json:"message"`
	}

	viper.Set("service.name", "TestService")
	viper.Set("server.api.name", "TestAPI")

	tests := []struct {
		name   string
		status models.StatusType
		input  samplePayload
		want   *models.Response[samplePayload]
	}{
		{
			name:   "success payload",
			status: models.StatusSuccess,
			input:  samplePayload{Message: "OK"},
			want: &models.Response[samplePayload]{
				Status:      models.StatusSuccess,
				ServiceName: "TestService",
				ApiName:     "TestAPI",
				Details:     samplePayload{Message: "OK"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.GenericResponse(tt.status, tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenericResponse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
