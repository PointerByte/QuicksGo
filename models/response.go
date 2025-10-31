package models

import "github.com/spf13/viper"

// StatusType defines the allowed values for API response status.
type StatusType string

const (
	StatusSuccess StatusType = "success"
	StatusError   StatusType = "error"
)

// Response defines a generic, standardized API response structure.
// It ensures consistency across all endpoints by including a status indicator,
// service metadata, and a typed payload for data or additional details.
//
// Example usage:
//   Response[any]{ Status: StatusSuccess, ServiceName: "viper.GetViper().GetString("service.name")", ApiName: "viper.GetViper().GetString("service.api.name")", Details: dettails }
type Response[T any] struct {
	Status  StatusType `json:"status"`            // "success" or "error"
	Service string     `json:"service"`           // Service name
	API     string     `json:"api"`               // API name
	Details T          `json:"details,omitempty"` // Optional payload (for success)
}

func (r *Response[T]) SetDefaultData() {
	r.Service = viper.GetString("service.name")
	r.API = viper.GetString("service.api.name")
}

// GenericResponse creates a standardized response object for any data type.
//
// It builds a Response[T] structure containing the operation status, service name,
// API name, and detailed payload. The function leverages generics to support any
// response data type while maintaining a consistent response format across the API.
//
// Configuration values such as service and API names are automatically loaded
// from Viper (service.name and service.api.name).
func GenericResponse[T any](status StatusType, Details T) *Response[T] {
	resp := &Response[T]{
		Status:  status,
		Details: Details,
	}
	resp.SetDefaultData()
	return resp
}
