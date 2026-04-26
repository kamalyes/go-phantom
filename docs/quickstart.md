# 快速开始 - 5分钟上手 go-phantom

## 🎯 目标

本指南将通过一个具体的示例，一步一步教你如何安装、配置和使用 go-phantom，让你快速上手动态数据隔离引擎

## 📦 步骤 1：安装

```bash
go get -u github.com/kamalyes/go-phantom
```

## 📦 步骤 2：创建幻影引擎

```go
package main

import (
    "context"
    "fmt"

    "github.com/kamalyes/go-phantom"
)

func main() {
    // 创建幻影引擎
    p := phantom.NewPhantom()

    // 注册分组和数据源
    dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
    dbGroup.AddSource(&phantom.DataSource{
        Name:        "primary_db",
        StorageType: phantom.StorageDatabase,
        Weight:      1,
    })
    dbGroup.AddSource(&phantom.DataSource{
        Name:        "slave_db",
        StorageType: phantom.StorageDatabase,
        ReadOnly:    true,
        Weight:      1,
    })

    p.RegisterGroup(dbGroup)
    p.SetDefaultGroup(phantom.StorageDatabase, "db_group")
    p.SetGroupStrategy("db_group", "primary")
    p.Initialize(context.Background())

    // 默认路由到主库
    source, _ := p.Resolve(context.Background(), "db_group")
    fmt.Println("默认数据源:", source.Name) // primary_db

    // 声明式切换到从库
    ctx := phantom.Use(context.Background(), "slave_db")
    source, _ = p.Resolve(ctx, "db_group")
    fmt.Println("切换后数据源:", source.Name) // slave_db
}
```

## 📦 步骤 3：声明式切换

go-phantom 的核心特性是**声明式数据源切换**，通过上下文 API 在业务代码中声明要使用的数据源：

```go
// 方式一：直接指定数据源名称
ctx := phantom.Use(context.Background(), "slave_db")

// 方式二：设置影子流量标记
ctx = phantom.WithShadow(ctx, true)

// 方式三：设置租户ID
ctx = phantom.WithTenant(ctx, "tenant_1")

// 方式四：设置只读标记
ctx = phantom.WithReadOnly(ctx, true)

// 方式五：设置路由提示
ctx = phantom.WithRouteHint(ctx, "hint_ds")

// 链式调用
ctx = phantom.Use(context.Background(), "slave_db").
    WithReadOnly(true).
    WithTenant("tenant_1")
```

## 📦 步骤 4：使用 WithDS 函数

`WithDS` 提供了一种更优雅的方式来在函数作用域内切换数据源：

```go
err := phantom.WithDS("slave_db", func(ctx context.Context) error {
    // 在这个函数内，所有通过 ctx 进行的路由都会使用 slave_db
    source, _ := p.Resolve(ctx, "db_group")
    fmt.Println("当前数据源:", source.Name) // slave_db
    return nil
})
// 函数返回后，上下文自动恢复
```

## 📦 步骤 5：便捷 API

go-phantom 提供了按存储类型的便捷 API：

```go
// 获取默认数据库数据源
dbSource, err := p.GetDB(context.Background())

// 获取默认 Redis 数据源
redisSource, err := p.GetRedis(context.Background())

// 获取指定分组的数据库数据源
dbSource, err := p.GetDB(context.Background(), "db_group")

// 通过存储类型解析
source, err := p.ResolveByType(context.Background(), phantom.StorageDatabase)
```

## 🎉 完成

恭喜！你已经掌握了 go-phantom 的基本使用方式。接下来你可以：

- [路由策略详解](routing-strategies.md) - 了解各种路由策略的使用场景
- [影子流量隔离](shadow-traffic.md) - 学习如何隔离测试流量
- [健康检查](health-check.md) - 配置数据源健康检查
- [配置驱动](config-driven.md) - 通过配置文件初始化引擎
- [网关集成](gateway-integration.md) - 在 HTTP/gRPC 网关中使用
