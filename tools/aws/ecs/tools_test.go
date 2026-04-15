// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/PointerByte/QuicksGo/logger/builder"
	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	awsTools "github.com/PointerByte/QuicksGo/tools/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
)

type stubECSClient struct {
	listTasksOutput     *ecs.ListTasksOutput
	listTasksErr        error
	describeTasksOutput *ecs.DescribeTasksOutput
	describeTasksErr    error
	listClustersOutput  *ecs.ListClustersOutput
	listClustersErr     error
}

func (s *stubECSClient) ListTasks(_ context.Context, _ *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	if s.listTasksOutput == nil {
		return &ecs.ListTasksOutput{}, s.listTasksErr
	}
	return s.listTasksOutput, s.listTasksErr
}

func (s *stubECSClient) DescribeTasks(_ context.Context, _ *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	if s.describeTasksOutput == nil {
		return &ecs.DescribeTasksOutput{}, s.describeTasksErr
	}
	return s.describeTasksOutput, s.describeTasksErr
}

func (s *stubECSClient) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	if s.listClustersOutput == nil {
		return &ecs.ListClustersOutput{}, s.listClustersErr
	}
	return s.listClustersOutput, s.listClustersErr
}

type stubListClustersPager struct {
	pages []*ecs.ListClustersOutput
	errs  []error
	index int
}

func (s *stubListClustersPager) HasMorePages() bool {
	return s.index < len(s.pages) || s.index < len(s.errs)
}

func (s *stubListClustersPager) NextPage(_ context.Context, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	var page *ecs.ListClustersOutput
	var err error
	if s.index < len(s.pages) {
		page = s.pages[s.index]
	}
	if s.index < len(s.errs) {
		err = s.errs[s.index]
	}
	s.index++
	if page == nil {
		page = &ecs.ListClustersOutput{}
	}
	return page, err
}

type stubListTasksPager struct {
	pages []*ecs.ListTasksOutput
	errs  []error
	index int
}

func (s *stubListTasksPager) HasMorePages() bool {
	return s.index < len(s.pages) || s.index < len(s.errs)
}

func (s *stubListTasksPager) NextPage(_ context.Context, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	var page *ecs.ListTasksOutput
	var err error
	if s.index < len(s.pages) {
		page = s.pages[s.index]
	}
	if s.index < len(s.errs) {
		err = s.errs[s.index]
	}
	s.index++
	if page == nil {
		page = &ecs.ListTasksOutput{}
	}
	return page, err
}

func TestNewDelegatesToMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockIToolsECS(ctrl)

	tool := New(mock)
	ctx := context.Background()
	ctxLogger := testLoggerContext()
	mock.EXPECT().GetTaskECSHosts(ctx, ctxLogger).Return([]string{"10.0.0.1"}, nil)

	hosts, err := tool.GetTaskECSHosts(ctx, ctxLogger)
	if err != nil {
		t.Fatalf("GetPodHosts() error = %v", err)
	}
	if !reflect.DeepEqual(hosts, []string{"10.0.0.1"}) {
		t.Fatalf("expected hosts %v, got %v", []string{"10.0.0.1"}, hosts)
	}
}

func TestGetPodHostsFiltersSelfTargets(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	discoverSelfFn = func(context.Context) (selfInfo, bool, error) {
		return selfInfo{clusterArn: "cluster", service: "svc", taskArn: "self"}, true, nil
	}
	loadAWSConfigFn = func(context.Context) (aws.Config, error) {
		return aws.Config{}, nil
	}
	newECSClientFn = func(aws.Config) ecsAPI {
		return &stubECSClient{
			listTasksOutput: &ecs.ListTasksOutput{TaskArns: []string{"self", "peer-a", "peer-b"}},
			describeTasksOutput: &ecs.DescribeTasksOutput{
				Tasks: []ecsTypes.Task{
					taskWithContainerIP("peer-a", "10.0.0.2"),
					taskWithContainerIP("peer-b", "10.0.0.3"),
				},
			},
		}
	}

	tool := &ToolsECS{}
	hosts, err := tool.GetTaskECSHosts(context.Background(), testLoggerContext())
	if err != nil {
		t.Fatalf("GetPodHosts() error = %v", err)
	}

	expected := []string{"10.0.0.2", "10.0.0.3"}
	if !reflect.DeepEqual(hosts, expected) {
		t.Fatalf("expected hosts %v, got %v", expected, hosts)
	}
}

func TestGetPodHostsReturnsNilWhenNotInECS(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	discoverSelfFn = func(context.Context) (selfInfo, bool, error) {
		return selfInfo{}, false, nil
	}

	tool := &ToolsECS{}
	hosts, err := tool.GetTaskECSHosts(context.Background(), testLoggerContext())
	if err != nil {
		t.Fatalf("GetPodHosts() error = %v", err)
	}
	if hosts != nil {
		t.Fatalf("expected nil hosts, got %v", hosts)
	}
}

func TestGetPodHostsPropagatesDiscoveryError(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	discoverSelfFn = func(context.Context) (selfInfo, bool, error) {
		return selfInfo{}, false, errors.New("boom")
	}

	hosts, err := (&ToolsECS{}).GetTaskECSHosts(context.Background(), testLoggerContext())
	if err == nil || !strings.Contains(err.Error(), "failed to discover current ECS task") {
		t.Fatalf("expected discovery error, got hosts=%v err=%v", hosts, err)
	}
}

func TestGetBroadcastTargetsReturnsDiscoverError(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	discoverSelfFn = func(context.Context) (selfInfo, bool, error) {
		return selfInfo{}, false, errors.New("boom")
	}

	_, _, err := (&ToolsECS{}).getBroadcastTargets(context.Background(), testLoggerContext())
	if err == nil || !strings.Contains(err.Error(), "failed to discover current ECS task") {
		t.Fatalf("expected discover error, got %v", err)
	}
}

func TestGetBroadcastTargetsReturnsLoadConfigError(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	discoverSelfFn = func(context.Context) (selfInfo, bool, error) {
		return selfInfo{clusterArn: "cluster", service: "svc", taskArn: "self"}, true, nil
	}
	loadAWSConfigFn = func(context.Context) (aws.Config, error) {
		return aws.Config{}, errors.New("config boom")
	}

	_, _, err := (&ToolsECS{}).getBroadcastTargets(context.Background(), testLoggerContext())
	if err == nil || !strings.Contains(err.Error(), "failed to load AWS config") {
		t.Fatalf("expected config error, got %v", err)
	}
}

func TestGetBroadcastTargetsReturnsEmptyWhenListTasksFails(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	discoverSelfFn = func(context.Context) (selfInfo, bool, error) {
		return selfInfo{clusterArn: "cluster", service: "svc", taskArn: "self"}, true, nil
	}
	loadAWSConfigFn = func(context.Context) (aws.Config, error) {
		return aws.Config{}, nil
	}
	newECSClientFn = func(aws.Config) ecsAPI {
		return &stubECSClient{listTasksErr: errors.New("list boom")}
	}

	targets, isECS, err := (&ToolsECS{}).getBroadcastTargets(context.Background(), testLoggerContext())
	if err != nil {
		t.Fatalf("getBroadcastTargets() error = %v", err)
	}
	if !isECS {
		t.Fatal("expected isECS to be true")
	}
	if len(targets) != 0 {
		t.Fatalf("expected no targets, got %v", targets)
	}
}

func TestGetBroadcastTargetsReturnsEmptyWhenDescribeTasksFails(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	discoverSelfFn = func(context.Context) (selfInfo, bool, error) {
		return selfInfo{clusterArn: "cluster", service: "svc", taskArn: "self"}, true, nil
	}
	loadAWSConfigFn = func(context.Context) (aws.Config, error) {
		return aws.Config{}, nil
	}
	newECSClientFn = func(aws.Config) ecsAPI {
		return &stubECSClient{
			listTasksOutput:  &ecs.ListTasksOutput{TaskArns: []string{"self", "peer"}},
			describeTasksErr: errors.New("describe boom"),
		}
	}

	targets, isECS, err := (&ToolsECS{}).getBroadcastTargets(context.Background(), testLoggerContext())
	if err != nil {
		t.Fatalf("getBroadcastTargets() error = %v", err)
	}
	if !isECS {
		t.Fatal("expected isECS to be true")
	}
	if len(targets) != 0 {
		t.Fatalf("expected no targets, got %v", targets)
	}
}

func TestDiscoverSelfInECSReturnsFalseWhenNoLocalIPs(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	localPrivateIPsFn = func() (map[string]bool, error) {
		return map[string]bool{}, nil
	}

	got, ok, err := discoverSelfInECS(context.Background())
	if err != nil {
		t.Fatalf("discoverSelfInECS() error = %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false, got self=%v", got)
	}
}

func TestDiscoverSelfInECSReturnsLoadConfigError(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	localPrivateIPsFn = func() (map[string]bool, error) {
		return map[string]bool{"10.0.0.8": true}, nil
	}
	loadAWSConfigFn = func(context.Context) (aws.Config, error) {
		return aws.Config{}, errors.New("config boom")
	}

	_, _, err := discoverSelfInECS(context.Background())
	if err == nil || !strings.Contains(err.Error(), "failed to load AWS config") {
		t.Fatalf("expected config error, got %v", err)
	}
}

func TestDiscoverSelfInECSFindsMatchingTask(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	localPrivateIPsFn = func() (map[string]bool, error) {
		return map[string]bool{"10.0.0.8": true}, nil
	}
	loadAWSConfigFn = func(context.Context) (aws.Config, error) {
		return aws.Config{}, nil
	}

	client := &stubECSClient{
		describeTasksOutput: &ecs.DescribeTasksOutput{
			Tasks: []ecsTypes.Task{
				{
					TaskArn: aws.String("task-1"),
					Group:   aws.String("service:broadcast"),
					Containers: []ecsTypes.Container{
						{
							NetworkInterfaces: []ecsTypes.NetworkInterface{
								{PrivateIpv4Address: aws.String("10.0.0.8")},
							},
						},
					},
				},
			},
		},
	}
	newECSClientFn = func(aws.Config) ecsAPI {
		return client
	}
	newListClustersPagerFn = func(ecsAPI, *ecs.ListClustersInput) listClustersPager {
		return &stubListClustersPager{
			pages: []*ecs.ListClustersOutput{
				{ClusterArns: []string{"cluster-a"}},
			},
		}
	}
	newListTasksPagerFn = func(ecsAPI, *ecs.ListTasksInput) listTasksPager {
		return &stubListTasksPager{
			pages: []*ecs.ListTasksOutput{
				{TaskArns: []string{"task-1"}},
			},
		}
	}

	got, ok, err := discoverSelfInECS(context.Background())
	if err != nil {
		t.Fatalf("discoverSelfInECS() error = %v", err)
	}
	if !ok {
		t.Fatal("expected to detect ECS task")
	}

	expected := selfInfo{
		clusterArn: "cluster-a",
		service:    "broadcast",
		taskArn:    "task-1",
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected selfInfo %v, got %v", expected, got)
	}
}

func TestDiscoverSelfInECSReturnsFalseWhenClusterPageFails(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	localPrivateIPsFn = func() (map[string]bool, error) {
		return map[string]bool{"10.0.0.8": true}, nil
	}
	loadAWSConfigFn = func(context.Context) (aws.Config, error) {
		return aws.Config{}, nil
	}
	newECSClientFn = func(aws.Config) ecsAPI {
		return &stubECSClient{}
	}
	newListClustersPagerFn = func(ecsAPI, *ecs.ListClustersInput) listClustersPager {
		return &stubListClustersPager{
			errs: []error{errors.New("page boom")},
		}
	}

	_, ok, err := discoverSelfInECS(context.Background())
	if err != nil {
		t.Fatalf("discoverSelfInECS() error = %v", err)
	}
	if ok {
		t.Fatal("expected ok=false")
	}
}

func TestDiscoverSelfInECSReturnsFalseWhenDescribeFails(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	localPrivateIPsFn = func() (map[string]bool, error) {
		return map[string]bool{"10.0.0.8": true}, nil
	}
	loadAWSConfigFn = func(context.Context) (aws.Config, error) {
		return aws.Config{}, nil
	}
	newECSClientFn = func(aws.Config) ecsAPI {
		return &stubECSClient{
			describeTasksErr: errors.New("describe boom"),
		}
	}
	newListClustersPagerFn = func(ecsAPI, *ecs.ListClustersInput) listClustersPager {
		return &stubListClustersPager{
			pages: []*ecs.ListClustersOutput{
				{ClusterArns: []string{"cluster-a"}},
			},
		}
	}
	newListTasksPagerFn = func(ecsAPI, *ecs.ListTasksInput) listTasksPager {
		return &stubListTasksPager{
			pages: []*ecs.ListTasksOutput{
				{TaskArns: []string{"task-1"}},
			},
		}
	}

	_, ok, err := discoverSelfInECS(context.Background())
	if err != nil {
		t.Fatalf("discoverSelfInECS() error = %v", err)
	}
	if ok {
		t.Fatal("expected ok=false")
	}
}

func TestExtractPrivateIPv4FromAttachment(t *testing.T) {
	task := ecsTypes.Task{
		Attachments: []ecsTypes.Attachment{
			{
				Type: aws.String("ElasticNetworkInterface"),
				Details: []ecsTypes.KeyValuePair{
					{Name: aws.String("privateIPv4Address"), Value: aws.String("10.0.0.5")},
				},
			},
		},
	}

	if got := extractPrivateIPv4(task); got != "10.0.0.5" {
		t.Fatalf("expected attachment IP, got %s", got)
	}
}

func TestExtractPrivateIPv4FallsBackToContainerInterface(t *testing.T) {
	task := taskWithContainerIP("task-2", "10.0.0.9")

	if got := extractPrivateIPv4(task); got != "10.0.0.9" {
		t.Fatalf("expected container IP, got %s", got)
	}
}

func TestExtractPrivateIPv4ReturnsEmptyWhenMissing(t *testing.T) {
	if got := extractPrivateIPv4(ecsTypes.Task{}); got != "" {
		t.Fatalf("expected empty IP, got %s", got)
	}
}

func TestLocalPrivateIPsReturnsOnlyIPv4NonLoopback(t *testing.T) {
	got, err := localPrivateIPs()
	if err != nil {
		t.Fatalf("localPrivateIPs() error = %v", err)
	}

	if got["127.0.0.1"] {
		t.Fatal("did not expect loopback address in result")
	}

	for value := range got {
		ip := net.ParseIP(value)
		if ip == nil {
			t.Fatalf("expected valid IP, got %q", value)
		}
		if ip.IsLoopback() {
			t.Fatalf("did not expect loopback IP, got %q", value)
		}
		if ip.To4() == nil {
			t.Fatalf("expected IPv4 address, got %q", value)
		}
	}
}

func TestGetHostsStoresRefreshHosts(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	prevNew := _New
	prevSetHosts := setHostsRefreshFn
	prevBuilderNew := builderNewFn
	t.Cleanup(func() {
		_New = prevNew
		setHostsRefreshFn = prevSetHosts
		builderNewFn = prevBuilderNew
	})

	var captured []string
	ctrl := gomock.NewController(t)
	mock := NewMockIToolsECS(ctrl)
	_New = func(IToolsECS) IToolsECS { return mock }
	setHostsRefreshFn = func(input ...string) {
		captured = append(captured, input...)
	}
	builderNewFn = func(context.Context) *builder.Context {
		return testLoggerContext()
	}
	mock.EXPECT().
		GetTaskECSHosts(gomock.Any(), gomock.Any()).
		Return([]string{"10.1.0.20", "10.1.0.21"}, nil)

	GetHosts()

	expected := []string{"10.1.0.20", "10.1.0.21"}
	if !reflect.DeepEqual(captured, expected) {
		t.Fatalf("expected captured hosts %v, got %v", expected, captured)
	}
}

func TestGetHostsDoesNotStoreHostsWhenDiscoveryFails(t *testing.T) {
	restore := stubECSDeps(t)
	defer restore()

	prevNew := _New
	prevSetHosts := setHostsRefreshFn
	prevBuilderNew := builderNewFn
	t.Cleanup(func() {
		_New = prevNew
		setHostsRefreshFn = prevSetHosts
		builderNewFn = prevBuilderNew
	})

	called := false
	ctrl := gomock.NewController(t)
	mock := NewMockIToolsECS(ctrl)
	_New = func(IToolsECS) IToolsECS { return mock }
	setHostsRefreshFn = func(...string) {
		called = true
	}
	builderNewFn = func(context.Context) *builder.Context {
		return testLoggerContext()
	}
	mock.EXPECT().
		GetTaskECSHosts(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("boom"))

	GetHosts()

	if called {
		t.Fatal("did not expect hosts to be stored on discovery error")
	}
}

func TestGeneratedMockecsAPI(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockecsAPI(ctrl)

	mock.EXPECT().
		ListClusters(gomock.Any(), gomock.Any()).
		Return(&ecs.ListClustersOutput{ClusterArns: []string{"cluster-a"}}, nil)
	mock.EXPECT().
		ListTasks(gomock.Any(), gomock.Any()).
		Return(&ecs.ListTasksOutput{TaskArns: []string{"task-a"}}, nil)
	mock.EXPECT().
		DescribeTasks(gomock.Any(), gomock.Any()).
		Return(&ecs.DescribeTasksOutput{}, nil)

	ctx := context.Background()

	clusters, err := mock.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		t.Fatalf("ListClusters() error = %v", err)
	}
	if !reflect.DeepEqual(clusters.ClusterArns, []string{"cluster-a"}) {
		t.Fatalf("unexpected clusters: %v", clusters.ClusterArns)
	}

	tasks, err := mock.ListTasks(ctx, &ecs.ListTasksInput{})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if !reflect.DeepEqual(tasks.TaskArns, []string{"task-a"}) {
		t.Fatalf("unexpected tasks: %v", tasks.TaskArns)
	}

	if _, err := mock.DescribeTasks(ctx, &ecs.DescribeTasksInput{}); err != nil {
		t.Fatalf("DescribeTasks() error = %v", err)
	}
}

func TestGeneratedMocklistClustersPager(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMocklistClustersPager(ctrl)

	gomock.InOrder(
		mock.EXPECT().HasMorePages().Return(true),
		mock.EXPECT().NextPage(gomock.Any()).Return(&ecs.ListClustersOutput{ClusterArns: []string{"cluster-a"}}, nil),
		mock.EXPECT().HasMorePages().Return(false),
	)

	if !mock.HasMorePages() {
		t.Fatal("expected HasMorePages() to return true")
	}

	page, err := mock.NextPage(context.Background())
	if err != nil {
		t.Fatalf("NextPage() error = %v", err)
	}
	if !reflect.DeepEqual(page.ClusterArns, []string{"cluster-a"}) {
		t.Fatalf("unexpected cluster page: %v", page.ClusterArns)
	}

	if mock.HasMorePages() {
		t.Fatal("expected HasMorePages() to return false")
	}
}

func TestGeneratedMocklistTasksPager(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMocklistTasksPager(ctrl)

	gomock.InOrder(
		mock.EXPECT().HasMorePages().Return(true),
		mock.EXPECT().NextPage(gomock.Any()).Return(&ecs.ListTasksOutput{TaskArns: []string{"task-a"}}, nil),
		mock.EXPECT().HasMorePages().Return(false),
	)

	if !mock.HasMorePages() {
		t.Fatal("expected HasMorePages() to return true")
	}

	page, err := mock.NextPage(context.Background())
	if err != nil {
		t.Fatalf("NextPage() error = %v", err)
	}
	if !reflect.DeepEqual(page.TaskArns, []string{"task-a"}) {
		t.Fatalf("unexpected task page: %v", page.TaskArns)
	}

	if mock.HasMorePages() {
		t.Fatal("expected HasMorePages() to return false")
	}
}

func taskWithContainerIP(taskArn, ip string) ecsTypes.Task {
	return ecsTypes.Task{
		TaskArn: aws.String(taskArn),
		Containers: []ecsTypes.Container{
			{
				NetworkInterfaces: []ecsTypes.NetworkInterface{
					{PrivateIpv4Address: aws.String(ip)},
				},
			},
		},
	}
}

func testLoggerContext() *builder.Context {
	viper.Set(string(viperdata.AppAtribute), "quicksgo-test")
	viper.Set(string(viperdata.LoggerModeTestAtribute), true)
	viper.Set(string(viperdata.LoggerIgnoredHeadersAtribute), []string{})
	viperdata.ResetViperDataSingleton()
	return builder.New(context.Background())
}

func stubECSDeps(t *testing.T) func() {
	t.Helper()

	prevLoad := loadAWSConfigFn
	prevDiscover := discoverSelfFn
	prevLocalIPs := localPrivateIPsFn
	prevNewClient := newECSClientFn
	prevClustersPager := newListClustersPagerFn
	prevTasksPager := newListTasksPagerFn

	loadAWSConfigFn = awsTools.LoadAWSConfig
	discoverSelfFn = discoverSelfInECS
	localPrivateIPsFn = localPrivateIPs
	newECSClientFn = func(cfg aws.Config) ecsAPI {
		return ecs.NewFromConfig(cfg)
	}
	newListClustersPagerFn = func(client ecsAPI, input *ecs.ListClustersInput) listClustersPager {
		return ecs.NewListClustersPaginator(client, input)
	}
	newListTasksPagerFn = func(client ecsAPI, input *ecs.ListTasksInput) listTasksPager {
		return ecs.NewListTasksPaginator(client, input)
	}

	return func() {
		loadAWSConfigFn = prevLoad
		discoverSelfFn = prevDiscover
		localPrivateIPsFn = prevLocalIPs
		newECSClientFn = prevNewClient
		newListClustersPagerFn = prevClustersPager
		newListTasksPagerFn = prevTasksPager
	}
}
