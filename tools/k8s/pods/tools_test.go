// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package pods

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	ktesting "k8s.io/client-go/testing"
)

func TestNewDelegatesToMock(t *testing.T) {
	builder.EnableModeTest()
	defer builder.DisableModeTest()

	ctrl := gomock.NewController(t)
	mock := NewMockIToolsK8S(ctrl)

	tool := New(mock)
	ctx := context.Background()
	ctxLogger := builder.New(ctx)
	mock.EXPECT().GetPodHosts(ctx, ctxLogger, "demo").Return([]string{"10.0.0.1"}, nil)

	hosts, err := tool.GetPodHosts(ctx, ctxLogger, "demo")
	if err != nil {
		t.Fatalf("GetPodHosts() error = %v", err)
	}
	if !reflect.DeepEqual(hosts, []string{"10.0.0.1"}) {
		t.Fatalf("expected hosts %v, got %v", []string{"10.0.0.1"}, hosts)
	}
}

func TestGetPodHostsReturnsRunningReadyPodIPs(t *testing.T) {
	builder.EnableModeTest()
	defer builder.DisableModeTest()

	tool := &ToolsK8S{}
	client := fake.NewSimpleClientset(
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "broadcast-svc", Namespace: "demo"},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"app": "broadcast",
					"env": "test",
				},
			},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ready-pod",
				Namespace: "demo",
				Labels: map[string]string{
					"app": "broadcast",
					"env": "test",
				},
			},
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
				PodIP: "10.0.0.10",
				Conditions: []v1.PodCondition{
					{Type: v1.PodReady, Status: v1.ConditionTrue},
				},
			},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pending-pod",
				Namespace: "demo",
				Labels: map[string]string{
					"app": "broadcast",
					"env": "test",
				},
			},
			Status: v1.PodStatus{
				Phase: v1.PodPending,
				PodIP: "10.0.0.11",
			},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "not-ready-pod",
				Namespace: "demo",
				Labels: map[string]string{
					"app": "broadcast",
					"env": "test",
				},
			},
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
				PodIP: "10.0.0.12",
				Conditions: []v1.PodCondition{
					{Type: v1.PodReady, Status: v1.ConditionFalse},
				},
			},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-ip-pod",
				Namespace: "demo",
				Labels: map[string]string{
					"app": "broadcast",
					"env": "test",
				},
			},
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
				Conditions: []v1.PodCondition{
					{Type: v1.PodReady, Status: v1.ConditionTrue},
				},
			},
		},
	)

	restore := stubClusterDependencies(t, client, nil, "broadcast-svc")
	defer restore()

	ctx := context.Background()
	hosts, err := tool.GetPodHosts(ctx, builder.New(ctx), "demo")
	if err != nil {
		t.Fatalf("GetPodHosts() error = %v", err)
	}

	expected := []string{"10.0.0.10"}
	if !reflect.DeepEqual(hosts, expected) {
		t.Fatalf("expected hosts %v, got %v", expected, hosts)
	}
}

func TestGetPodHostsReturnsErrorWhenConfigFails(t *testing.T) {
	builder.EnableModeTest()
	defer builder.DisableModeTest()

	tool := &ToolsK8S{}
	restore := stubClusterDependencies(t, nil, errors.New("config boom"), "broadcast-svc")
	defer restore()

	ctx := context.Background()
	_, err := tool.GetPodHosts(ctx, builder.New(ctx), "demo")
	if err == nil || !strings.Contains(err.Error(), "failed to create in-cluster config") {
		t.Fatalf("expected config error, got %v", err)
	}
}

func TestGetPodHostsReturnsNoHostsWhenNotInCluster(t *testing.T) {
	builder.EnableModeTest()
	defer builder.DisableModeTest()

	tool := &ToolsK8S{}
	restore := stubClusterDependencies(t, nil, rest.ErrNotInCluster, "broadcast-svc")
	defer restore()

	ctx := context.Background()
	hosts, err := tool.GetPodHosts(ctx, builder.New(ctx), "demo")
	if err != nil {
		t.Fatalf("expected no error when not in cluster, got %v", err)
	}
	if hosts != nil {
		t.Fatalf("expected nil hosts when not in cluster, got %v", hosts)
	}
}

func TestGetPodHostsReturnsErrorWhenClientCreationFails(t *testing.T) {
	builder.EnableModeTest()
	defer builder.DisableModeTest()

	tool := &ToolsK8S{}

	prevConfigFn := inClusterConfigFn
	prevClientFn := newForConfigFn
	prevAppNameFn := appNameFn
	t.Cleanup(func() {
		inClusterConfigFn = prevConfigFn
		newForConfigFn = prevClientFn
		appNameFn = prevAppNameFn
	})

	inClusterConfigFn = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	newForConfigFn = func(*rest.Config) (kubernetes.Interface, error) {
		return nil, errors.New("client boom")
	}
	appNameFn = func() string {
		return "broadcast-svc"
	}

	ctx := context.Background()
	_, err := tool.GetPodHosts(ctx, builder.New(ctx), "demo")
	if err == nil || !strings.Contains(err.Error(), "failed to create kubernetes client") {
		t.Fatalf("expected client creation error, got %v", err)
	}
}

func TestGetPodHostsReturnsErrorWhenServiceHasNoSelector(t *testing.T) {
	builder.EnableModeTest()
	defer builder.DisableModeTest()

	tool := &ToolsK8S{}
	client := fake.NewSimpleClientset(
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "broadcast-svc", Namespace: "demo"},
		},
	)

	restore := stubClusterDependencies(t, client, nil, "broadcast-svc")
	defer restore()

	ctx := context.Background()
	_, err := tool.GetPodHosts(ctx, builder.New(ctx), "demo")
	if err == nil || !strings.Contains(err.Error(), "has no selector") {
		t.Fatalf("expected selector error, got %v", err)
	}
}

func TestGetPodHostsReturnsErrorWhenServiceDoesNotExist(t *testing.T) {
	builder.EnableModeTest()
	defer builder.DisableModeTest()

	tool := &ToolsK8S{}
	client := fake.NewSimpleClientset()
	restore := stubClusterDependencies(t, client, nil, "broadcast-svc")
	defer restore()

	ctx := context.Background()
	_, err := tool.GetPodHosts(ctx, builder.New(ctx), "demo")
	if err == nil || !strings.Contains(err.Error(), "failed to get service broadcast-svc") {
		t.Fatalf("expected service lookup error, got %v", err)
	}
}

func TestGetPodHostsReturnsErrorWhenPodListFails(t *testing.T) {
	builder.EnableModeTest()
	defer builder.DisableModeTest()

	tool := &ToolsK8S{}
	client := fake.NewSimpleClientset(
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "broadcast-svc", Namespace: "demo"},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{"app": "broadcast"},
			},
		},
	)

	client.PrependReactor("list", "pods", func(_ ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInternalError(errors.New("pods boom"))
	})

	restore := stubClusterDependencies(t, client, nil, "broadcast-svc")
	defer restore()

	ctx := context.Background()
	_, err := tool.GetPodHosts(ctx, builder.New(ctx), "demo")
	if err == nil || !strings.Contains(err.Error(), "failed to list pods") {
		t.Fatalf("expected pod list error, got %v", err)
	}
}

func TestBuildLabelSelector(t *testing.T) {
	selector := buildLabelSelector(map[string]string{
		"app": "broadcast",
		"env": "test",
	})

	parts := strings.Split(selector, ",")
	if len(parts) != 2 {
		t.Fatalf("expected 2 selector parts, got %v", parts)
	}

	expectedParts := map[string]bool{
		"app=broadcast": true,
		"env=test":      true,
	}
	for _, part := range parts {
		if !expectedParts[part] {
			t.Fatalf("unexpected selector part %s", part)
		}
	}
}

func TestIsPodHostAvailable(t *testing.T) {
	pod := v1.Pod{
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIP: "10.0.0.20",
			Conditions: []v1.PodCondition{
				{Type: v1.PodReady, Status: v1.ConditionTrue},
			},
		},
	}

	if !isPodHostAvailable(pod) {
		t.Fatal("expected pod to be available")
	}

	pod.Status.PodIP = ""
	if isPodHostAvailable(pod) {
		t.Fatal("expected pod without IP to be unavailable")
	}
}

func TestGetHostsStoresRefreshHosts(t *testing.T) {
	builder.EnableModeTest()
	defer builder.DisableModeTest()

	prevNew := _New
	prevSetHosts := setHostsRefreshFn
	prevBuilderNew := builderNewFn
	t.Cleanup(func() {
		_New = prevNew
		setHostsRefreshFn = prevSetHosts
		builderNewFn = prevBuilderNew
	})

	viper.Set("app.namespace", "demo")

	var captured []string
	ctrl := gomock.NewController(t)
	mock := NewMockIToolsK8S(ctrl)
	_New = func(IToolsK8S) IToolsK8S { return mock }
	setHostsRefreshFn = func(input ...string) {
		captured = append(captured, input...)
	}
	builderNewFn = func(context.Context) *builder.Context {
		return builder.New(context.Background())
	}
	mock.EXPECT().
		GetPodHosts(gomock.Any(), gomock.Any(), "demo").
		Return([]string{"10.0.0.10", "10.0.0.11"}, nil)

	GetHosts()

	expected := []string{"10.0.0.10", "10.0.0.11"}
	if !reflect.DeepEqual(captured, expected) {
		t.Fatalf("expected captured hosts %v, got %v", expected, captured)
	}
}

func TestGetHostsDoesNotStoreHostsWhenDiscoveryFails(t *testing.T) {
	builder.EnableModeTest()
	defer builder.DisableModeTest()

	prevNew := _New
	prevSetHosts := setHostsRefreshFn
	prevBuilderNew := builderNewFn
	t.Cleanup(func() {
		_New = prevNew
		setHostsRefreshFn = prevSetHosts
		builderNewFn = prevBuilderNew
	})

	viper.Set("app.namespace", "demo")

	called := false
	ctrl := gomock.NewController(t)
	mock := NewMockIToolsK8S(ctrl)
	_New = func(IToolsK8S) IToolsK8S { return mock }
	setHostsRefreshFn = func(...string) {
		called = true
	}
	builderNewFn = func(context.Context) *builder.Context {
		return builder.New(context.Background())
	}
	mock.EXPECT().
		GetPodHosts(gomock.Any(), gomock.Any(), "demo").
		Return(nil, errors.New("boom"))

	GetHosts()

	if called {
		t.Fatal("did not expect hosts to be stored on discovery error")
	}
}

func stubClusterDependencies(t *testing.T, client kubernetes.Interface, configErr error, appName string) func() {
	t.Helper()

	prevConfigFn := inClusterConfigFn
	prevClientFn := newForConfigFn
	prevAppNameFn := appNameFn

	inClusterConfigFn = func() (*rest.Config, error) {
		if configErr != nil {
			return nil, configErr
		}
		return &rest.Config{}, nil
	}
	newForConfigFn = func(*rest.Config) (kubernetes.Interface, error) {
		return client, nil
	}
	appNameFn = func() string {
		return appName
	}

	return func() {
		inClusterConfigFn = prevConfigFn
		newForConfigFn = prevClientFn
		appNameFn = prevAppNameFn
	}
}
