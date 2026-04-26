# 路由策略详解

## 🎯 概述

go-phantom 提供了多种内置路由策略，每种策略适用于不同的业务场景。你可以为每个数据源分组设置不同的策略，也可以在运行时动态切换策略

## 📋 策略一览

| 策略 | 名称 | 适用场景 | 说明 |
|------|------|---------|------|
| PrimaryStrategy | primary | 默认写操作 | 始终选择主数据源 |
| ReadOnlyStrategy | readonly | 读操作 | 优先选择只读数据源，无则回退主库 |
| ReadWriteStrategy | readwrite | 读写分离 | 根据只读标记自动选择读库或写库 |
| TenantStrategy | tenant | 多租户 | 按租户ID路由到对应数据源 |
| RoundRobinStrategy | roundrobin | 负载均衡 | 轮询选择数据源 |
| WeightedStrategy | weighted | 加权负载 | 按权重随机选择数据源 |
| HintStrategy | hint | 定向路由 | 通过路由提示选择指定数据源 |
| FailoverStrategy | failover | 高可用 | 自动故障转移 |

## 🚀 PrimaryStrategy - 主库策略

始终选择分组中的主数据源，适用于所有写操作

```go
p := phantom.NewPhantom()

dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
dbGroup.AddSource(&phantom.DataSource{Name: "primary_db", StorageType: phantom.StorageDatabase})
p.RegisterGroup(dbGroup)

p.SetGroupStrategy("db_group", "primary")
p.Initialize(context.Background())

source, _ := p.Resolve(context.Background(), "db_group")
// source.Name == "primary_db"
```

## 📖 ReadOnlyStrategy - 只读策略

优先选择只读数据源，没有只读数据源时回退到主库

```go
dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
dbGroup.AddSource(&phantom.DataSource{Name: "primary_db", StorageType: phantom.StorageDatabase})
dbGroup.AddSource(&phantom.DataSource{
    Name:        "slave_db",
    StorageType: phantom.StorageDatabase,
    ReadOnly:    true,
})
p.RegisterGroup(dbGroup)

p.SetGroupStrategy("db_group", "readonly")
p.Initialize(context.Background())

source, _ := p.Resolve(context.Background(), "db_group")
// source.Name == "slave_db"（优先选择只读数据源）
```

## 🔄 ReadWriteStrategy - 读写分离策略

根据路由上下文中的 `ReadOnly` 标记自动选择读库或写库

```go
p.SetGroupStrategy("db_group", "readwrite")
p.Initialize(context.Background())

// 写操作 - 默认路由到主库
source, _ := p.Resolve(context.Background(), "db_group")
// source.Name == "primary_db"

// 读操作 - 设置只读标记后路由到从库
ctx := phantom.WithReadOnly(context.Background(), true)
source, _ = p.Resolve(ctx, "db_group")
// source.Name == "slave_db"
```

## 👥 TenantStrategy - 多租户策略

按租户ID路由到对应的数据源，未匹配时回退到主库

```go
dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
dbGroup.AddSource(&phantom.DataSource{Name: "primary_db", StorageType: phantom.StorageDatabase})
dbGroup.AddSource(&phantom.DataSource{
    Name:        "tenant_1_db",
    StorageType: phantom.StorageDatabase,
    TenantID:    "tenant_1",
})
dbGroup.AddSource(&phantom.DataSource{
    Name:        "tenant_2_db",
    StorageType: phantom.StorageDatabase,
    TenantID:    "tenant_2",
})
p.RegisterGroup(dbGroup)

p.SetGroupStrategy("db_group", "tenant")
p.Initialize(context.Background())

// 租户1的请求路由到 tenant_1_db
ctx := phantom.WithTenant(context.Background(), "tenant_1")
source, _ := p.Resolve(ctx, "db_group")
// source.Name == "tenant_1_db"

// 未知租户回退到主库
ctx2 := phantom.WithTenant(context.Background(), "unknown")
source2, _ := p.Resolve(ctx2, "db_group")
// source2.Name == "primary_db"
```

### 大规模多租户（1000+ 租户）

TenantStrategy 内部使用 `tenantMap`（map[string]*DataSource）实现 O(1) 租户查找，无论租户数量多少，查找性能恒定：

```go
// 1000 个租户 - 一次性注册
dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
dbGroup.AddSource(&phantom.DataSource{Name: "primary_db", StorageType: phantom.StorageDatabase})

for i := 0; i < 1000; i++ {
    tenantID := fmt.Sprintf("tenant_%d", i)
    dbGroup.AddSource(&phantom.DataSource{
        Name:        fmt.Sprintf("db_%s", tenantID),
        StorageType: phantom.StorageDatabase,
        TenantID:    tenantID,
        Instance:    createDBForTenant(tenantID), // 每个租户独立的数据库实例
    })
}

p.RegisterGroup(dbGroup)
p.SetGroupStrategy("db_group", "tenant")
p.Initialize(context.Background())

// O(1) 查找 - 无论 100 还是 10000 租户，性能一致
ctx := phantom.WithTenant(context.Background(), "tenant_500")
source, _ := p.Resolve(ctx, "db_group")
```

**性能基准**（Intel i5-9300H）：

| 租户数量 | 查找耗时 | 内存分配 | 分配次数 |
|---------|---------|---------|---------|
| 10 | 122.8 ns/op | 32 B | 1 |
| 100 | 87.3 ns/op | 32 B | 1 |
| 1,000 | 103.9 ns/op | 32 B | 1 |
| 10,000 | 116.5 ns/op | 32 B | 1 |

> 可以看到从 10 到 10000 租户，查找耗时几乎不变，这就是 O(1) 的威力

**并发性能**（1000 租户，多协程并行）：

| 场景 | 耗时 | 内存分配 |
|------|------|---------|
| 单协程 | 103.9 ns/op | 32 B/op |
| 多协程并行 | 362.1 ns/op | 223 B/op |

**直接使用 tenantMap**：

```go
// 跳过策略层，直接通过 Group 的 tenantMap 查找（零分配）
ds := group.GetSourceByTenant("tenant_500")  // 34-54 ns/op, 0 B/op

// 查询租户数量
count := group.TenantCount()
```

## 🔁 RoundRobinStrategy - 轮询策略

使用原子计数器实现无锁轮询选择，适用于无状态负载均衡

```go
dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
dbGroup.AddSource(&phantom.DataSource{Name: "db1", StorageType: phantom.StorageDatabase})
dbGroup.AddSource(&phantom.DataSource{Name: "db2", StorageType: phantom.StorageDatabase})
dbGroup.AddSource(&phantom.DataSource{Name: "db3", StorageType: phantom.StorageDatabase})
p.RegisterGroup(dbGroup)

p.SetGroupStrategy("db_group", "roundrobin")
p.Initialize(context.Background())

// 依次轮询: db1 → db2 → db3 → db1 → ...
```

## ⚖️ WeightedStrategy - 加权策略

按权重随机选择数据源，使用 go-toolbox 的 random.RandInt 实现协程安全

```go
dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
dbGroup.AddSource(&phantom.DataSource{Name: "db1", StorageType: phantom.StorageDatabase, Weight: 3})
dbGroup.AddSource(&phantom.DataSource{Name: "db2", StorageType: phantom.StorageDatabase, Weight: 1})
p.RegisterGroup(dbGroup)

p.SetGroupStrategy("db_group", "weighted")
p.Initialize(context.Background())

// db1 被选中的概率约为 75%，db2 约为 25%
```

## 🎯 HintStrategy - 提示路由策略

通过路由提示定向选择指定数据源，未匹配时回退到主库

```go
dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
dbGroup.AddSource(&phantom.DataSource{Name: "primary_db", StorageType: phantom.StorageDatabase})
dbGroup.AddSource(&phantom.DataSource{Name: "special_db", StorageType: phantom.StorageDatabase})
p.RegisterGroup(dbGroup)

p.SetGroupStrategy("db_group", "hint")
p.Initialize(context.Background())

ctx := phantom.WithRouteHint(context.Background(), "special_db")
source, _ := p.Resolve(ctx, "db_group")
// source.Name == "special_db"
```

## 🛡️ FailoverStrategy - 故障转移策略

先尝试回退策略，失败后遍历健康数据源寻找可用源

```go
dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
dbGroup.AddSource(&phantom.DataSource{Name: "db1", StorageType: phantom.StorageDatabase})
dbGroup.AddSource(&phantom.DataSource{Name: "db2", StorageType: phantom.StorageDatabase})
p.RegisterGroup(dbGroup)

// 参数：最大重试次数，回退策略（nil 则使用 PrimaryStrategy）
p.SetGroupStrategy("db_group", "failover")
p.Initialize(context.Background())
```

## 🔄 运行时切换策略

可以在运行时动态切换分组策略：

```go
// 初始使用主库策略
p.SetGroupStrategy("db_group", "primary")

// 运行时切换为读写分离
p.SetGroupStrategy("db_group", "readwrite")

// 切换为轮询
p.SetGroupStrategy("db_group", "roundrobin")
```

## 🧩 自定义策略

实现 `Strategy` 接口即可创建自定义策略：

```go
type MyStrategy struct{}

func (s *MyStrategy) Resolve(ctx context.Context, group *phantom.Group, routeCtx *phantom.RouteContext) (*phantom.RouteResult, error) {
    // 自定义路由逻辑
    return &phantom.RouteResult{Source: group.Primary, GroupName: group.Name}, nil
}

func (s *MyStrategy) Name() string {
    return "my_strategy"
}

// 注册自定义策略
p.SetGroupStrategy("db_group", &MyStrategy{})
```
