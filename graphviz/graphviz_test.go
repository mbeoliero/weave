package graphviz_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mbeoliero/weave"
	"github.com/mbeoliero/weave/graphviz"
)

func TestGraphvizMiddleware(t *testing.T) {
	ctx := context.Background()

	// 创建 graphviz 实例
	ctx, graph := graphviz.NewBuilder("TestGraphvizMW").WithTimeDetail().Build(ctx)
	defer graph.Log(ctx)

	// 创建测试节点
	nodeA := weave.NewNode("A", func(ctx context.Context, c int) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	nodeB := weave.NewNode("B", func(ctx context.Context, c int) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}, "A")

	nodeC := weave.NewNode("C", func(ctx context.Context, c int) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}, "A")

	nodeD := weave.NewNode("D", func(ctx context.Context, c int) error {
		time.Sleep(300 * time.Millisecond)
		return nil
	}, "B")

	nodeE := weave.NewNode("E", func(ctx context.Context, c int) error {
		time.Sleep(30 * time.Millisecond)
		return nil
	}, "B", "C")

	// 创建 Group 并添加 graphviz 中间件
	group := weave.NewGroup[int]("test_group")
	group.AddMiddleware(graphviz.Middleware())

	// 添加节点到组
	group.AddNode(nodeA)
	group.AddNode(nodeB)
	group.AddNode(nodeC)
	group.AddNode(nodeD)
	group.AddNode(nodeE)

	err := group.AsNode().Exec(ctx, 42)
	if err != nil {
		t.Fatalf("Failed to execute group: %v", err)
	}

	// 输出图信息
	info := graph.GetInfo()
	if info != "" {
		fmt.Printf("Graph info: %s\n", info)
	}
}

func TestGraphvizWithMultipleGroups(t *testing.T) {
	ctx := context.Background()

	// 创建 graphviz 实例
	ctx, graph := graphviz.NewBuilder("MultiGroupTest").WithMinCost(50).Build(ctx)
	defer graph.Log(ctx)

	// 第一个组
	group1 := weave.NewGroup[string]("group1")
	group1.AddMiddleware(graphviz.Middleware())

	nodeA := weave.NewNode("GroupA_NodeA", func(ctx context.Context, c string) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	nodeB := weave.NewNode("GroupA_NodeB", func(ctx context.Context, c string) error {
		time.Sleep(150 * time.Millisecond)
		return nil
	}, "GroupA_NodeA")

	group1.AddNode(nodeA)
	group1.AddNode(nodeB)

	// 第二个组
	group2 := weave.NewGroup[string]("group2", "group1")
	group2.AddMiddleware(graphviz.Middleware())

	nodeC := weave.NewNode("GroupB_NodeC", func(ctx context.Context, c string) error {
		time.Sleep(80 * time.Millisecond)
		return nil
	})

	nodeD := weave.NewNode("GroupB_NodeD", func(ctx context.Context, c string) error {
		time.Sleep(120 * time.Millisecond)
		return nil
	}, "GroupB_NodeC")

	group2.AddNode(nodeC)
	group2.AddNode(nodeD)

	err := group1.AsNode().Exec(ctx, "test")
	if err != nil {
		t.Fatalf("Failed to execute group1: %v", err)
	}

	err = group2.AsNode().Exec(ctx, "test")
	if err != nil {
		t.Fatalf("Failed to execute group2: %v", err)
	}

	// 输出图信息
	info := graph.GetInfo()
	if info != "" {
		fmt.Printf("Multi-group graph info: %s\n", info)
	}
}

func TestGraphvizWithErrors(t *testing.T) {
	ctx := context.Background()

	// 创建 graphviz 实例
	ctx, graph := graphviz.NewBuilder("ErrorTest").Build(ctx)
	defer graph.Log(ctx)

	// 创建会出错的节点
	errorNode := weave.NewNode("ErrorNode", func(ctx context.Context, c int) error {
		time.Sleep(50 * time.Millisecond)
		return fmt.Errorf("test error")
	})

	successNode := weave.NewNode("SuccessNode", func(ctx context.Context, c int) error {
		time.Sleep(30 * time.Millisecond)
		return nil
	}, "ErrorNode")

	group := weave.NewGroup[int]("error_group")
	group.AddMiddleware(graphviz.Middleware())

	group.AddNode(errorNode)
	group.AddNode(successNode)

	// 执行会有错误，但仍然会记录到 graphviz
	_ = group.AsNode().Exec(ctx, 42)

	// 输出图信息
	info := graph.GetInfo()
	if info != "" {
		fmt.Printf("Error graph info: %s\n", info)
	}
}

func TestGraphvizWithConcurrencyLimit(t *testing.T) {
	ctx := context.Background()

	// 创建 graphviz 实例
	ctx, graph := graphviz.NewBuilder("ConcurrencyLimitTest").WithTimeDetail().Build(ctx)
	defer graph.Log(ctx)

	// 创建测试节点 - 设计一个场景：同时有3个节点可以执行，但MaxGoNum=2
	// A (无依赖，50ms)
	// B (无依赖，100ms)
	// C (无依赖，80ms)
	// D (依赖A，60ms)
	// E (依赖B和C，40ms)
	//
	// 期望的执行序列：
	// 1. A和B同时开始 (2个goroutine)
	// 2. A完成后，C开始 (因为还有C在等待)
	// 3. B和C完成后，D和E可以开始

	start := time.Now()
	nodeA := weave.NewNode("A", func(ctx context.Context, c int) error {
		defer func() { t.Log("end exec A in ", time.Since(start).Milliseconds()) }()
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	nodeB := weave.NewNode("B", func(ctx context.Context, c int) error {
		defer func() { t.Log("end exec B in ", time.Since(start).Milliseconds()) }()
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	nodeC := weave.NewNode("C", func(ctx context.Context, c int) error {
		defer func() { t.Log("end exec C in ", time.Since(start).Milliseconds()) }()
		time.Sleep(80 * time.Millisecond)
		return nil
	})

	nodeD := weave.NewNode("D", func(ctx context.Context, c int) error {
		defer func() { t.Log("end exec D in ", time.Since(start).Milliseconds()) }()
		time.Sleep(60 * time.Millisecond)
		return nil
	}, "A")

	nodeE := weave.NewNode("E", func(ctx context.Context, c int) error {
		defer func() { t.Log("end exec E in ", time.Since(start).Milliseconds()) }()
		time.Sleep(40 * time.Millisecond)
		return nil
	}, "B", "C")

	// 创建 Group 并设置并发限制为2
	group := weave.NewGroup[int]("concurrency_test")
	group.SetMaxGoNum(2) // 关键：限制并发度为2
	group.AddMiddleware(graphviz.Middleware())

	// 添加节点到组
	group.AddNode(nodeA)
	group.AddNode(nodeB)
	group.AddNode(nodeC)
	group.AddNode(nodeD)
	group.AddNode(nodeE)

	start = time.Now()
	err := group.AsNode().Exec(ctx, 42)
	totalTime := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to execute group: %v", err)
	}

	// 验证总执行时间
	// 理论上的执行序列：A(0-50) + B(0-100) 并行，然后 C(50-130) + D(50-110)，最后 E(130-170)
	// 总时间应该约为170ms
	expectedMin := 170 * time.Millisecond
	expectedMax := 200 * time.Millisecond

	if totalTime < expectedMin {
		t.Errorf("Total execution time %v is less than expected minimum %v", totalTime, expectedMin)
	}
	if totalTime > expectedMax {
		t.Errorf("Total execution time %v is greater than expected maximum %v", totalTime, expectedMax)
	}

	// 输出图信息
	info := graph.GetInfo()
	if info != "" {
		fmt.Printf("Concurrency limit test graph info: %s\n", info)
	}

	t.Logf("Total execution time: %v", totalTime)
}

// TestGroupExec was removed because it depends on weave package's private test helpers
// (Tuple2, NewCalcNodes, Param). The test is kept in weave/graph_test.go instead.

func TestGroupToGroupDependency(t *testing.T) {
	ctx := context.Background()

	// 创建 graphviz 实例
	ctx, graph := graphviz.NewBuilder("GroupToGroupTest").WithTimeDetail().Build(ctx)
	defer graph.Log(ctx)

	// 创建第一个组 - 数据准备组
	prepGroup := weave.NewGroup[string]("prep")
	prepGroup.AddMiddleware(graphviz.Middleware())

	nodePrep1 := weave.NewNode("PrepData", func(ctx context.Context, c string) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	nodePrep2 := weave.NewNode("ValidateData", func(ctx context.Context, c string) error {
		time.Sleep(30 * time.Millisecond)
		return nil
	}, "PrepData")

	prepGroup.AddNode(nodePrep1)
	prepGroup.AddNode(nodePrep2)

	// 创建第二个组 - 处理组，依赖于prep组
	processGroup := weave.NewGroup[string]("process", "prep")
	processGroup.AddMiddleware(graphviz.Middleware())

	nodeProcess1 := weave.NewNode("ProcessA", func(ctx context.Context, c string) error {
		time.Sleep(80 * time.Millisecond)
		return nil
	})

	nodeProcess2 := weave.NewNode("ProcessB", func(ctx context.Context, c string) error {
		time.Sleep(60 * time.Millisecond)
		return nil
	}, "ProcessA")

	processGroup.AddNode(nodeProcess1)
	processGroup.AddNode(nodeProcess2)

	// 创建第三个组 - 输出组，依赖于process组
	outputGroup := weave.NewGroup[string]("output", "process")
	outputGroup.AddMiddleware(graphviz.Middleware())

	nodeOutput := weave.NewNode("GenerateReport", func(ctx context.Context, c string) error {
		time.Sleep(40 * time.Millisecond)
		return nil
	})

	outputGroup.AddNode(nodeOutput)

	// 创建图并添加组
	mainGraph := weave.NewGraph[string]()
	mainGraph.AddGlobalMW(graphviz.Middleware())
	mainGraph.AddNode(prepGroup.AsNode())
	mainGraph.AddNode(processGroup.AsNode())
	mainGraph.AddNode(outputGroup.AsNode())

	err := mainGraph.Exec(ctx, "test data")
	if err != nil {
		t.Fatalf("Failed to execute graph: %v", err)
	}

	// 输出图信息
	info := graph.GetInfo()
	if info != "" {
		fmt.Printf("Group-to-Group dependency graph info: %s\n", info)
	}
}

func TestNodeToGroupDependency(t *testing.T) {
	ctx := context.Background()

	// 创建 graphviz 实例
	ctx, graph := graphviz.NewBuilder("NodeToGroupTest").WithTimeDetail().Build(ctx)
	defer graph.Log(ctx)

	// 创建服务启动组
	serviceGroup := weave.NewGroup[int]("services")
	serviceGroup.AddMiddleware(graphviz.Middleware())

	dbNode := weave.NewNode("StartDB", func(ctx context.Context, c int) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	cacheNode := weave.NewNode("StartCache", func(ctx context.Context, c int) error {
		time.Sleep(60 * time.Millisecond)
		return nil
	})

	apiNode := weave.NewNode("StartAPI", func(ctx context.Context, c int) error {
		time.Sleep(40 * time.Millisecond)
		return nil
	}, "StartDB", "StartCache")

	serviceGroup.AddNode(dbNode)
	serviceGroup.AddNode(cacheNode)
	serviceGroup.AddNode(apiNode)

	// 创建依赖于整个服务组的独立节点
	healthCheckNode := weave.NewNode("HealthCheck", func(ctx context.Context, c int) error {
		time.Sleep(30 * time.Millisecond)
		return nil
	}, "services") // 依赖整个services组

	monitorNode := weave.NewNode("StartMonitoring", func(ctx context.Context, c int) error {
		time.Sleep(25 * time.Millisecond)
		return nil
	}, "services") // 依赖整个services组

	// 创建依赖于监控的节点
	alertNode := weave.NewNode("SetupAlerts", func(ctx context.Context, c int) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}, "StartMonitoring")

	// 创建图并添加节点
	mainGraph := weave.NewGraph[int]()
	mainGraph.AddGlobalMW(graphviz.Middleware())
	mainGraph.AddNode(serviceGroup.AsNode())
	mainGraph.AddNode(healthCheckNode)
	mainGraph.AddNode(monitorNode)
	mainGraph.AddNode(alertNode)

	err := mainGraph.Exec(ctx, 42)
	if err != nil {
		t.Fatalf("Failed to execute graph: %v", err)
	}

	// 输出图信息
	info := graph.GetInfo()
	if info != "" {
		fmt.Printf("Node-to-Group dependency graph info: %s\n", info)
	}
}

func TestComplexMixedDependencies(t *testing.T) {
	ctx := context.Background()

	// 创建 graphviz 实例
	ctx, graph := graphviz.NewBuilder("ComplexMixedTest").WithTimeDetail().Build(ctx)
	defer graph.Log(ctx)

	// 创建基础设施组
	infraGroup := weave.NewGroup[string]("infrastructure")
	infraGroup.AddMiddleware(graphviz.Middleware())

	networkNode := weave.NewNode("SetupNetwork", func(ctx context.Context, c string) error {
		time.Sleep(80 * time.Millisecond)
		return nil
	})

	storageNode := weave.NewNode("SetupStorage", func(ctx context.Context, c string) error {
		time.Sleep(90 * time.Millisecond)
		return nil
	})

	infraGroup.AddNode(networkNode)
	infraGroup.AddNode(storageNode)

	// 创建应用组，依赖于基础设施组
	appGroup := weave.NewGroup[string]("application", "infrastructure")
	appGroup.AddMiddleware(graphviz.Middleware())

	webServerNode := weave.NewNode("StartWebServer", func(ctx context.Context, c string) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	appGroup.AddNode(webServerNode)

	// 创建独立的配置节点，依赖于基础设施组
	configNode := weave.NewNode("LoadConfig", func(ctx context.Context, c string) error {
		time.Sleep(30 * time.Millisecond)
		return nil
	}, "infrastructure")

	// 创建部署节点，依赖于应用组和配置节点
	deployNode := weave.NewNode("Deploy", func(ctx context.Context, c string) error {
		time.Sleep(40 * time.Millisecond)
		return nil
	}, "application", "LoadConfig")

	// 创建独立的测试节点，依赖于部署节点
	integrationTestNode := weave.NewNode("IntegrationTest", func(ctx context.Context, c string) error {
		time.Sleep(60 * time.Millisecond)
		return nil
	}, "Deploy")

	loadTestNode := weave.NewNode("LoadTest", func(ctx context.Context, c string) error {
		time.Sleep(70 * time.Millisecond)
		return nil
	}, "Deploy")

	// 创建图并添加所有节点
	mainGraph := weave.NewGraph[string]()
	mainGraph.AddGlobalMW(graphviz.Middleware())
	mainGraph.AddNode(infraGroup.AsNode())
	mainGraph.AddNode(appGroup.AsNode())
	mainGraph.AddNode(configNode)
	mainGraph.AddNode(deployNode)
	mainGraph.AddNode(integrationTestNode)
	mainGraph.AddNode(loadTestNode)

	err := mainGraph.Exec(ctx, "deployment")
	if err != nil {
		t.Fatalf("Failed to execute graph: %v", err)
	}

	// 输出图信息
	info := graph.GetInfo()
	if info != "" {
		fmt.Printf("Complex mixed dependencies graph info: %s\n", info)
	}
}

func TestNestedGroupDependencies(t *testing.T) {
	ctx := context.Background()

	// 创建 graphviz 实例
	ctx, graph := graphviz.NewBuilder("NestedGroupTest").WithTimeDetail().Build(ctx)
	defer graph.Log(ctx)

	// 创建顶层组 - 数据层
	dataGroup := weave.NewGroup[int]("data")
	dataGroup.AddMiddleware(graphviz.Middleware())

	dbInitNode := weave.NewNode("InitDB", func(ctx context.Context, c int) error {
		time.Sleep(60 * time.Millisecond)
		return nil
	})

	dataGroup.AddNode(dbInitNode)

	// 创建嵌套的数据访问子组
	dataAccessGroup := weave.NewGroup[int]("access", "InitDB")
	dataAccessGroup.AddMiddleware(graphviz.Middleware())

	repoNode := weave.NewNode("SetupRepo", func(ctx context.Context, c int) error {
		time.Sleep(40 * time.Millisecond)
		return nil
	})

	cacheLayerNode := weave.NewNode("SetupCache", func(ctx context.Context, c int) error {
		time.Sleep(30 * time.Millisecond)
		return nil
	}, "SetupRepo")

	dataAccessGroup.AddNode(repoNode)
	dataAccessGroup.AddNode(cacheLayerNode)

	// 将子组添加到父组
	dataGroup.AddNode(dataAccessGroup.AsNode())

	// 创建业务逻辑组，依赖于数据访问组的最后一个节点
	businessGroup := weave.NewGroup[int]("business", "data")
	businessGroup.AddMiddleware(graphviz.Middleware())

	serviceNode := weave.NewNode("BusinessService", func(ctx context.Context, c int) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	validatorNode := weave.NewNode("Validator", func(ctx context.Context, c int) error {
		time.Sleep(35 * time.Millisecond)
		return nil
	}, "BusinessService")

	businessGroup.AddNode(serviceNode)
	businessGroup.AddNode(validatorNode)

	// 创建API层节点，依赖于业务组的最后一个节点
	apiNode := weave.NewNode("StartAPI", func(ctx context.Context, c int) error {
		time.Sleep(45 * time.Millisecond)
		return nil
	}, "business")

	// 创建嵌套的监控组
	monitoringGroup := weave.NewGroup[int]("monitoring", "StartAPI")
	monitoringGroup.AddMiddleware(graphviz.Middleware())

	startupNode := weave.NewNode("Startup", func(ctx context.Context, c int) error {
		time.Sleep(30 * time.Millisecond)
		return nil
	})

	monitoringGroup.AddNode(startupNode)

	// 监控子组中的健康检查组
	healthGroup := weave.NewGroup[int]("health", "Startup")
	healthGroup.AddMiddleware(graphviz.Middleware())

	healthCheckNode := weave.NewNode("HealthCheck", func(ctx context.Context, c int) error {
		time.Sleep(25 * time.Millisecond)
		return nil
	})

	healthGroup.AddNode(healthCheckNode)

	// 监控子组中的指标组
	metricsGroup := weave.NewGroup[int]("metrics", "Startup")
	metricsGroup.AddMiddleware(graphviz.Middleware())

	metricsCollectorNode := weave.NewNode("MetricsCollector", func(ctx context.Context, c int) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	})

	metricsGroup.AddNode(metricsCollectorNode)

	// 将子组添加到监控组
	monitoringGroup.AddNode(healthGroup.AsNode())
	monitoringGroup.AddNode(metricsGroup.AsNode())

	// 创建告警节点，依赖于监控组的节点
	alertNode := weave.NewNode("AlertSystem", func(ctx context.Context, c int) error {
		time.Sleep(15 * time.Millisecond)
		return nil
	}, "monitoring")

	// 创建主图
	mainGraph := weave.NewGraph[int]()
	mainGraph.AddGlobalMW(graphviz.Middleware())
	mainGraph.AddNode(dataGroup.AsNode())
	mainGraph.AddNode(businessGroup.AsNode())
	mainGraph.AddNode(apiNode)
	mainGraph.AddNode(monitoringGroup.AsNode())
	mainGraph.AddNode(alertNode)

	err := mainGraph.Exec(ctx, 100)
	if err != nil {
		t.Fatalf("Failed to execute nested group graph: %v", err)
	}

	// 输出图信息
	info := graph.GetInfo()
	if info != "" {
		fmt.Printf("Nested group dependencies graph info: %s\n", info)
	}

	// 验证执行结果
	t.Log("Nested group dependency test completed successfully")
}

func TestSimpleGroupDependencyChain(t *testing.T) {
	ctx := context.Background()

	// 创建 graphviz 实例
	ctx, graph := graphviz.NewBuilder("SimpleGroupChain").WithTimeDetail().Build(ctx)
	defer graph.Log(ctx)

	// 创建第一个组 - 没有依赖
	groupA := weave.NewGroup[string]("groupA")
	groupA.AddMiddleware(graphviz.Middleware())
	nodeA := weave.NewNode("TaskA", func(ctx context.Context, c string) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	groupA.AddNode(nodeA)

	// 创建第二个组 - 依赖groupA
	groupB := weave.NewGroup[string]("groupB", "groupA")
	groupB.AddMiddleware(graphviz.Middleware())
	nodeB := weave.NewNode("TaskB", func(ctx context.Context, c string) error {
		time.Sleep(60 * time.Millisecond)
		return nil
	})
	groupB.AddNode(nodeB)

	// 创建第三个组 - 依赖groupB
	groupC := weave.NewGroup[string]("groupC", "groupB")
	groupC.AddMiddleware(graphviz.Middleware())
	nodeC := weave.NewNode("TaskC", func(ctx context.Context, c string) error {
		time.Sleep(40 * time.Millisecond)
		return nil
	})
	groupC.AddNode(nodeC)

	// 创建独立节点 - 依赖groupA
	independentNode := weave.NewNode("IndependentTask", func(ctx context.Context, c string) error {
		time.Sleep(30 * time.Millisecond)
		return nil
	}, "groupA")

	// 创建最终节点 - 依赖groupC和独立节点
	finalNode := weave.NewNode("FinalTask", func(ctx context.Context, c string) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}, "groupC", "IndependentTask")

	// 创建主图
	mainGraph := weave.NewGraph[string]()
	mainGraph.AddGlobalMW(graphviz.Middleware())
	mainGraph.AddNode(groupA.AsNode())
	mainGraph.AddNode(groupB.AsNode())
	mainGraph.AddNode(groupC.AsNode())
	mainGraph.AddNode(independentNode)
	mainGraph.AddNode(finalNode)

	start := time.Now()
	err := mainGraph.Exec(ctx, "test")
	totalTime := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to execute simple group chain: %v", err)
	}

	// 输出图信息
	info := graph.GetInfo()
	if info != "" {
		fmt.Printf("Simple group dependency chain graph info: %s\n", info)
	}

	t.Logf("Simple group chain test completed in %v", totalTime)
}

func TestWellDesignedComplexDependencies(t *testing.T) {
	ctx := context.Background()

	// 创建 graphviz 实例
	ctx, graph := graphviz.NewBuilder("WellDesignedComplexTest").WithTimeDetail().Build(ctx)
	defer graph.Log(ctx)

	// 第一层：基础设施组 - 没有依赖
	infraGroup := weave.NewGroup[int]("infrastructure")
	infraGroup.AddMiddleware(graphviz.Middleware())

	networkSetupNode := weave.NewNode("NetworkSetup", func(ctx context.Context, c int) error {
		time.Sleep(60 * time.Millisecond)
		return nil
	})

	storageSetupNode := weave.NewNode("StorageSetup", func(ctx context.Context, c int) error {
		time.Sleep(70 * time.Millisecond)
		return nil
	}, "NetworkSetup")

	infraGroup.AddNode(networkSetupNode)
	infraGroup.AddNode(storageSetupNode)

	// 第二层：数据库组 - 依赖于基础设施组
	databaseGroup := weave.NewGroup[int]("database", "infrastructure")
	databaseGroup.AddMiddleware(graphviz.Middleware())

	dbInstallNode := weave.NewNode("DBInstall", func(ctx context.Context, c int) error {
		time.Sleep(80 * time.Millisecond)
		return nil
	})

	dbConfigNode := weave.NewNode("DBConfig", func(ctx context.Context, c int) error {
		time.Sleep(40 * time.Millisecond)
		return nil
	}, "DBInstall")

	databaseGroup.AddNode(dbInstallNode)
	databaseGroup.AddNode(dbConfigNode)

	// 第三层：应用服务组 - 依赖于数据库组
	appGroup := weave.NewGroup[int]("application", "database")
	appGroup.AddMiddleware(graphviz.Middleware())

	appServerNode := weave.NewNode("AppServer", func(ctx context.Context, c int) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	apiGatewayNode := weave.NewNode("APIGateway", func(ctx context.Context, c int) error {
		time.Sleep(45 * time.Millisecond)
		return nil
	}, "AppServer")

	appGroup.AddNode(appServerNode)
	appGroup.AddNode(apiGatewayNode)

	// 独立节点：配置加载器 - 依赖于基础设施组的最后一个节点
	configLoaderNode := weave.NewNode("ConfigLoader", func(ctx context.Context, c int) error {
		time.Sleep(30 * time.Millisecond)
		return nil
	}, "infrastructure")

	// 独立节点：健康检查 - 依赖于应用组的最后一个节点
	healthCheckNode := weave.NewNode("HealthCheck", func(ctx context.Context, c int) error {
		time.Sleep(25 * time.Millisecond)
		return nil
	}, "application")

	// 第四层：监控组 - 依赖于应用组和健康检查节点
	monitorGroup := weave.NewGroup[int]("monitoring", "HealthCheck")
	monitorGroup.AddMiddleware(graphviz.Middleware())

	metricsCollectorNode := weave.NewNode("MetricsCollector", func(ctx context.Context, c int) error {
		time.Sleep(35 * time.Millisecond)
		return nil
	})

	logAggregatorNode := weave.NewNode("LogAggregator", func(ctx context.Context, c int) error {
		time.Sleep(40 * time.Millisecond)
		return nil
	})

	monitorGroup.AddNode(metricsCollectorNode)
	monitorGroup.AddNode(logAggregatorNode)

	// 最终节点：部署完成通知 - 依赖于监控组的节点和配置加载器
	deploymentCompleteNode := weave.NewNode("DeploymentComplete", func(ctx context.Context, c int) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}, "monitoring", "ConfigLoader")

	// 创建主图并添加所有节点和组
	mainGraph := weave.NewGraph[int]()
	mainGraph.AddGlobalMW(graphviz.Middleware())

	// 添加组和独立节点（按依赖顺序）
	mainGraph.AddNode(infraGroup.AsNode())
	mainGraph.AddNode(databaseGroup.AsNode())
	mainGraph.AddNode(appGroup.AsNode())
	mainGraph.AddNode(configLoaderNode)
	mainGraph.AddNode(healthCheckNode)
	mainGraph.AddNode(monitorGroup.AsNode())
	mainGraph.AddNode(deploymentCompleteNode)

	start := time.Now()
	err := mainGraph.Exec(ctx, 42)
	totalTime := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to execute well-designed complex graph: %v", err)
	}

	// 验证执行时间：应该是层次化执行
	// 估算：基础设施(130ms) -> 数据库(120ms) -> 应用(95ms) -> 配置和健康检查并行 -> 监控(75ms) -> 完成(20ms)
	// 大约 440ms
	expectedMin := 400 * time.Millisecond
	expectedMax := 500 * time.Millisecond

	if totalTime < expectedMin {
		t.Errorf("Total execution time %v is less than expected minimum %v", totalTime, expectedMin)
	}
	if totalTime > expectedMax {
		t.Errorf("Total execution time %v is greater than expected maximum %v", totalTime, expectedMax)
	}

	// 输出图信息
	info := graph.GetInfo()
	if info != "" {
		fmt.Printf("Well-designed complex dependencies graph info: %s\n", info)
	}

	t.Logf("Well-designed complex test completed in %v", totalTime)
}
