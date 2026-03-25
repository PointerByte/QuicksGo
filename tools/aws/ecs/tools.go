// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -source=tools.go -destination=./mocksTools.go -package=ecs

package ecs

import (
	"context"
	"fmt"
	"net"
	"strings"

	serverGin "github.com/PointerByte/QuicksGo/config/server/gin"
	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/PointerByte/QuicksGo/logger/formatter"
	awsTools "github.com/PointerByte/QuicksGo/tools/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type ecsAPI interface {
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

type listClustersPager interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
}

type listTasksPager interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
}

var (
	loadAWSConfigFn   = awsTools.LoadAWSConfig
	discoverSelfFn    = discoverSelfInECS
	localPrivateIPsFn = localPrivateIPs
	setHostsRefreshFn = serverGin.SetHostsRefresh
	builderNewFn      = builder.New
	newECSClientFn    = func(cfg aws.Config) ecsAPI {
		return ecs.NewFromConfig(cfg)
	}
	newListClustersPagerFn = func(client ecsAPI, input *ecs.ListClustersInput) listClustersPager {
		return ecs.NewListClustersPaginator(client, input)
	}
	newListTasksPagerFn = func(client ecsAPI, input *ecs.ListTasksInput) listTasksPager {
		return ecs.NewListTasksPaginator(client, input)
	}
)

// IToolsECS describes the ECS host discovery operations that can be consumed
// or mocked by other components.
type IToolsECS interface {
	GetTaskECSHosts(ctx context.Context, ctxLogger *builder.Context) ([]string, error)
}

// ToolsECS resolves broadcast hosts from ECS tasks and can delegate to a mock.
type ToolsECS struct {
	mocks IToolsECS
}

// New creates a new ECS tool instance.
// When a mock is provided, all calls are delegated to it.
func New(mock IToolsECS) IToolsECS {
	return &ToolsECS{
		mocks: mock,
	}
}

// GetTaskECSHosts returns the private IPs of peer ECS tasks that belong to the
// same ECS service as the current task.
//
// Example usage with a Gin server prepared for testing:
//
//	type ECSHostDiscovery interface {
//		GetTaskECSHosts(ctx context.Context, ctxLogger *builder.Context) ([]string, error)
//	}
//
//	type RefreshHandler struct {
//		discovery ECSHostDiscovery
//	}
//
//	func NewRefreshHandler(discovery ECSHostDiscovery) *RefreshHandler {
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
//
//		hosts, err := h.discovery.GetTaskECSHosts(ctx, ctxLogger)
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
//	handler := NewRefreshHandler(ecs.New(nil))
//	handler.Register(api)
//
//	serverGin.Start(srv)
//
// Test setup:
//
//	type mockECSDiscovery struct{}
//
//	func (m *mockECSDiscovery) GetTaskECSHosts(_ context.Context, _ *builder.Context) ([]string, error) {
//		return []string{"10.1.0.20", "10.1.0.21"}, nil
//	}
func (t *ToolsECS) GetTaskECSHosts(ctx context.Context, ctxLogger *builder.Context) (result []string, _ error) {
	if t.mocks != nil {
		return t.mocks.GetTaskECSHosts(ctx, ctxLogger)
	}

	targets, isECS, err := t.getBroadcastTargets(ctx, ctxLogger)
	if err != nil {
		return nil, err
	}
	if !isECS {
		return
	}

	for _, tg := range targets {
		if tg.IsSelf {
			continue
		}
		result = append(result, tg.IP)
	}
	return
}

// Target represents an ECS task that can receive a broadcast request.
type Target struct {
	IP      string
	IsSelf  bool
	TaskArn string
}

// getBroadcastTargets returns all tasks from the same ECS service as the
// current task. If the process is not running in ECS, it returns isECS=false.
func (t *ToolsECS) getBroadcastTargets(ctx context.Context, ctxLogger *builder.Context) (targets []Target, isECS bool, err error) {
	process := &formatter.Service{
		System:  "AWS SDK ECS",
		Process: "We retrieve hosts from ECS tasks",
		Server:  "ECS",
		Path:    "AWS SDK",
		Status:  formatter.SUCCESS,
	}
	ctxLogger.TraceInit(process)
	defer ctxLogger.TraceEnd(process)

	me, ok, e := discoverSelfFn(ctx)
	if e != nil {
		return nil, false, fmt.Errorf("failed to discover current ECS task: %w", e)
	}
	if !ok {
		return []Target{}, false, nil
	}

	cfg, err := loadAWSConfigFn(ctx)
	if err != nil {
		process.Status = formatter.ERROR
		return nil, false, fmt.Errorf("failed to load AWS config: %w", err)
	}
	cli := newECSClientFn(cfg)

	ltOut, err := cli.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       aws.String(me.clusterArn),
		ServiceName:   aws.String(me.service),
		DesiredStatus: ecsTypes.DesiredStatusRunning,
	})
	if err != nil {
		return []Target{}, true, nil
	}
	if len(ltOut.TaskArns) == 0 {
		return []Target{}, true, nil
	}

	var taskArns []string
	for _, taskArn := range ltOut.TaskArns {
		if taskArn == me.taskArn {
			continue
		}
		taskArns = append(taskArns, taskArn)
	}

	desc, err := cli.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(me.clusterArn),
		Tasks:   taskArns,
	})
	if err != nil {
		return []Target{}, true, nil
	}

	var out []Target
	for _, task := range desc.Tasks {
		ip := extractPrivateIPv4(task)
		if ip == "" {
			continue
		}
		out = append(out, Target{
			IP:      ip,
			IsSelf:  aws.ToString(task.TaskArn) == me.taskArn,
			TaskArn: aws.ToString(task.TaskArn),
		})
	}

	return out, true, nil
}

type selfInfo struct {
	clusterArn string
	service    string
	taskArn    string
}

// discoverSelfInECS tries to identify the current task by matching local IPs
// against the IPs assigned to running ECS tasks. It returns ok=false when the
// process does not appear to be running inside ECS.
func discoverSelfInECS(ctx context.Context) (si selfInfo, ok bool, hardErr error) {
	localIPs, err := localPrivateIPsFn()
	if err != nil || len(localIPs) == 0 {
		return selfInfo{}, false, nil
	}

	cfg, err := loadAWSConfigFn(ctx)
	if err != nil {
		return selfInfo{}, false, fmt.Errorf("failed to load AWS config: %w", err)
	}
	cli := newECSClientFn(cfg)

	cluPager := newListClustersPagerFn(cli, &ecs.ListClustersInput{MaxResults: aws.Int32(100)})
	for cluPager.HasMorePages() {
		cluOut, err := cluPager.NextPage(ctx)
		if err != nil {
			return selfInfo{}, false, nil
		}
		for _, clu := range cluOut.ClusterArns {
			taskPager := newListTasksPagerFn(cli, &ecs.ListTasksInput{
				Cluster:       aws.String(clu),
				DesiredStatus: ecsTypes.DesiredStatusRunning,
				MaxResults:    aws.Int32(100),
			})

			var batch []string
			flush := func() (bool, selfInfo) {
				if len(batch) == 0 {
					return false, selfInfo{}
				}

				desc, err2 := cli.DescribeTasks(ctx, &ecs.DescribeTasksInput{
					Cluster: aws.String(clu),
					Tasks:   batch,
				})
				batch = batch[:0]
				if err2 != nil {
					return false, selfInfo{}
				}

				for _, task := range desc.Tasks {
					ip := extractPrivateIPv4(task)
					if ip != "" && localIPs[ip] {
						return true, selfInfo{
							clusterArn: clu,
							service:    strings.TrimPrefix(aws.ToString(task.Group), "service:"),
							taskArn:    aws.ToString(task.TaskArn),
						}
					}
				}

				return false, selfInfo{}
			}

			for taskPager.HasMorePages() {
				tOut, err3 := taskPager.NextPage(ctx)
				if err3 != nil {
					break
				}
				for _, taskArn := range tOut.TaskArns {
					batch = append(batch, taskArn)
					if len(batch) >= 100 {
						if ok, si := flush(); ok {
							return si, true, nil
						}
					}
				}
			}

			if ok, si := flush(); ok {
				return si, true, nil
			}
		}
	}

	return selfInfo{}, false, nil
}

// localPrivateIPs returns the set of non-loopback private IPv4 addresses
// assigned to the current host.
func localPrivateIPs() (map[string]bool, error) {
	out := make(map[string]bool)
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, ifc := range ifaces {
		addrs, _ := ifc.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			out[ip.String()] = true
		}
	}

	return out, nil
}

// extractPrivateIPv4 extracts the private IPv4 address from an ECS task,
// preferring ENI attachment details and falling back to container interfaces.
func extractPrivateIPv4(task ecsTypes.Task) string {
	for _, attachment := range task.Attachments {
		if attachment.Type != nil && *attachment.Type == "ElasticNetworkInterface" {
			for _, detail := range attachment.Details {
				if detail.Name != nil && *detail.Name == "privateIPv4Address" && detail.Value != nil {
					return *detail.Value
				}
			}
		}
	}

	for _, container := range task.Containers {
		for _, networkInterface := range container.NetworkInterfaces {
			if networkInterface.PrivateIpv4Address != nil {
				return *networkInterface.PrivateIpv4Address
			}
		}
	}
	return ""
}

var _New = New

func GetHosts() {
	ctx := context.Background()
	ctxLogger := builderNewFn(ctx)
	host, err := _New(nil).GetTaskECSHosts(ctx, ctxLogger)
	if err != nil {
		ctxLogger.Error(fmt.Errorf("Error configuring the service task hosts in the refresh handler: %v", err))
		return
	}
	setHostsRefreshFn(host...)
}
