# 健康检查

## 🎯 概述

go-phantom 内置了数据源健康检查机制，定期检测数据源的可用性，自动标记不健康的数据源并刷新缓存，确保路由请求始终发送到可用的数据源

## 🚀 基本使用

### 默认配置

```go
p := phantom.NewPhantom()
// 默认启用健康检查，间隔10秒，超时3秒
p.Initialize(context.Background())
```

### 自定义配置

```go
p := phantom.NewPhantom(
    phantom.WithHealthCheck(phantom.HealthCheckConfig{
        Enabled:  true,
        Interval: 30 * time.Second,  // 检查间隔
        Timeout:  5 * time.Second,   // 单次检查超时
    }),
)
p.Initialize(context.Background())
```

### 禁用健康检查

```go
p := phantom.NewPhantom(
    phantom.WithHealthCheck(phantom.HealthCheckConfig{
        Enabled: false,
    }),
)
```

## 📋 内置检查器

### DBHealthChecker - 数据库检查器

使用 `PingContext` 检查数据库连接是否可用：

```go
// 自动注册，当 DataSource.Instance 实现了 PingContext 方法时生效
// 如果 Instance 为 nil，则返回当前健康状态
```

### RedisHealthChecker - Redis 检查器

使用 `Ping` 方法检查 Redis 连接是否可用：

```go
// 自动注册，当 DataSource.Instance 实现了 Ping 方法时生效
```

### GenericHealthChecker - 通用检查器

当 Instance 为 nil 时，直接返回当前健康状态：

```go
// 适用于没有实际连接实例的数据源
```

## 🧩 自定义检查器

实现 `HealthChecker` 接口即可创建自定义检查器：

```go
type MyHealthChecker struct{}

func (c *MyHealthChecker) Check(ctx context.Context, ds *phantom.DataSource) bool {
    // 自定义检查逻辑
    // 返回 true 表示健康，false 表示不健康
    conn := ds.Instance.(net.Conn)
    return conn != nil
}

// 注册自定义检查器
p.RegisterHealthChecker(phantom.StorageCustom, &MyHealthChecker{})
```

## 📊 手动健康检查

### 查询所有数据源健康状态

```go
health := p.GetRegistry().HealthCheckAll()
// 返回 map[string]map[string]bool
// {
//   "db_group": {
//     "primary_db": true,
//     "slave_db": false,
//   }
// }
```

### 手动标记数据源状态

```go
// 获取数据源
group, _ := p.GetRegistry().GetGroup("db_group")
ds := group.GetSource("primary_db")

// 标记为不健康
ds.MarkUnhealthy()

// 标记为健康
ds.MarkHealthy()

// 检查健康状态
isHealthy := ds.IsHealthy()
```

## 🔄 缓存刷新

当数据源健康状态变化时，需要刷新分组的健康数据源缓存：

```go
// 手动刷新缓存
group.InvalidateCache()

// 刷新后重新获取健康数据源列表
healthy := group.GetHealthySources()
```

健康检查管理器在检测到状态变化时会自动刷新缓存

## ⚙️ 配置参考

```go
phantom.HealthCheckConfig{
    Enabled:  true,             // 是否启用健康检查
    Interval: 10 * time.Second, // 检查间隔
    Timeout:  3 * time.Second,  // 单次检查超时
}
```

### 默认值

| 参数 | 默认值 | 说明 |
|------|--------|------|
| Enabled | true | 默认启用 |
| Interval | 10s | 每10秒检查一次 |
| Timeout | 3s | 单次检查超时3秒 |
