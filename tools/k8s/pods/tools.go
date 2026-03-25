// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -source=tools.go -destination=./mocksTools.go -package=pods

package pods

import (
	"context"
	"errors"
	"fmt"
	"strings"

	serverGin "github.com/PointerByte/QuicksGo/config/server/gin"
	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/PointerByte/QuicksGo/logger/formatter"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	// inClusterConfigFn obtiene la configuracion del cluster actual.
	// Se define como variable para facilitar pruebas unitarias sin depender
	// de un entorno real de Kubernetes.
	inClusterConfigFn = rest.InClusterConfig

	// newForConfigFn construye el cliente de Kubernetes a partir de una configuracion.
	// Se inyecta en pruebas para simular respuestas del API server.
	newForConfigFn = func(config *rest.Config) (kubernetes.Interface, error) {
		return kubernetes.NewForConfig(config)
	}

	// appNameFn resuelve el nombre del servicio objetivo desde la configuracion.
	// Mantenerlo desacoplado permite validar escenarios sin tocar el estado global de Viper.
	appNameFn = func() string {
		return viper.GetString("app.name")
	}

	setHostsRefreshFn = serverGin.SetHostsRefresh
	builderNewFn      = builder.New
)

// IToolsK8S define las operaciones del paquete k8s que pueden ser usadas o simuladas
// por otros componentes.
type IToolsK8S interface {
	GetPodHosts(ctx context.Context, ctxLogger *builder.Context, namespace string) ([]string, error)
}

// ToolsK8S implementa el acceso a Kubernetes y opcionalmente delega en un doble de prueba.
type ToolsK8S struct {
	mocks IToolsK8S
}

// New crea una nueva herramienta para consultar recursos de Kubernetes.
// Si se proporciona un mock, las llamadas se delegan a ese objeto.
func New(mock IToolsK8S) IToolsK8S {
	return &ToolsK8S{
		mocks: mock,
	}
}

// GetPodHosts obtains the IPs of Running and Ready pods selected by the
// configured Kubernetes Service in the provided namespace.
//
// Example usage with a Gin server prepared for testing:
//
//	type PodHostDiscovery interface {
//		GetPodHosts(ctx context.Context, ctxLogger *builder.Context, namespace string) ([]string, error)
//	}
//
//	type RefreshHandler struct {
//		discovery PodHostDiscovery
//	}
//
//	func NewRefreshHandler(discovery PodHostDiscovery) *RefreshHandler {
//		return &RefreshHandler{discovery: discovery}
//	}
//
//	func (h *RefreshHandler) Register(api *gin.RouterGroup) {
//		api.GET("/refresh/hosts", h.GetHosts)
//	}
//
//	func (h *RefreshHandler) GetHosts(c *gin.Context) {
//		ctx := c.Request.Context()
//		ctxLogger := builder.New(ctx)
//		namespace := viper.GetString("app.namespace")
//
//		hosts, err := h.discovery.GetPodHosts(ctx, ctxLogger, namespace)
//		if err != nil {
//			c.JSON(500, gin.H{"error": err.Error()})
//			return
//		}
//
//		serverGin.SetHostsRefresh(hosts...)
//		c.JSON(200, gin.H{"hosts": hosts})
//	}
//
// Production setup:
//
//	srv, err := serverGin.CreateApp()
//	if err != nil {
//		panic(err)
//	}
//
//	api := serverGin.GetRoute("/api/v1")
//	handler := NewRefreshHandler(pods.New(nil))
//	handler.Register(api)
//
//	serverGin.Start(srv)
//
// Test setup:
//
//	type mockPodDiscovery struct{}
//
//	func (m *mockPodDiscovery) GetPodHosts(_ context.Context, _ *builder.Context, _ string) ([]string, error) {
//		return []string{"10.0.0.10", "10.0.0.11"}, nil
//	}
func (t *ToolsK8S) GetPodHosts(ctx context.Context, ctxLogger *builder.Context, namespace string) ([]string, error) {
	if t.mocks != nil {
		return t.mocks.GetPodHosts(ctx, ctxLogger, namespace)
	}
	process := &formatter.Service{
		System:  "K8S",
		Process: "We retrieve hosts from pods K8S",
		Path:    "K8S API",
		Status:  formatter.SUCCESS,
	}
	ctxLogger.TraceInit(process)
	defer ctxLogger.TraceEnd(process)

	config, err := inClusterConfigFn()
	if err != nil {
		if errors.Is(err, rest.ErrNotInCluster) {
			return nil, nil
		}
		process.Status = formatter.ERROR
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	clientset, err := newForConfigFn(config)
	if err != nil {
		process.Status = formatter.ERROR
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	serviceName := appNameFn()
	svc, err := clientset.CoreV1().Services(namespace).Get(
		context.TODO(),
		serviceName,
		metav1.GetOptions{},
	)
	if err != nil {
		process.Status = formatter.ERROR
		return nil, fmt.Errorf("failed to get service %s: %w", serviceName, err)
	}

	if len(svc.Spec.Selector) == 0 {
		process.Status = formatter.ERROR
		return nil, fmt.Errorf("service %s has no selector", serviceName)
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: buildLabelSelector(svc.Spec.Selector),
		},
	)
	if err != nil {
		process.Status = formatter.ERROR
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var hosts []string
	for _, pod := range pods.Items {
		if isPodHostAvailable(pod) {
			hosts = append(hosts, pod.Status.PodIP)
		}
	}
	return hosts, nil
}

// buildLabelSelector transforma el selector de un Service en el formato esperado
// por ListOptions.LabelSelector.
func buildLabelSelector(selector map[string]string) string {
	parts := make([]string, 0, len(selector))
	for key, value := range selector {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, ",")
}

// isPodHostAvailable valida si un pod expone una IP util para broadcast.
// Solo se consideran pods Running, con IP asignada y marcados como Ready.
func isPodHostAvailable(pod v1.Pod) bool {
	if pod.Status.Phase != v1.PodRunning || pod.Status.PodIP == "" {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

var _New = New

func GetHosts() {
	ctx := context.Background()
	ctxLogger := builderNewFn(ctx)
	namespace := viper.GetString("app.namespace")
	host, err := _New(nil).GetPodHosts(ctx, ctxLogger, namespace)
	if err != nil {
		ctxLogger.Error(fmt.Errorf("Error configuring the service task hosts in the refresh handler: %v", err))
		return
	}
	setHostsRefreshFn(host...)
	ctxLogger.Info("The service task hosts were configured in the refresh handler")
}
