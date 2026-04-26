# 影子流量隔离

## 🎯 概述

影子流量隔离是 go-phantom 的核心特性之一，它允许你将特定的请求流量路由到影子数据源（如测试数据库），实现生产环境与测试环境的数据隔离。这在压测、灰度发布、功能验证等场景下非常有用

## 🔥 核心概念

### 什么是影子流量？

影子流量是指被标记为需要路由到影子数据源的请求。通过匹配规则识别影子流量后，go-phantom 会自动将请求路由到对应的影子数据源，确保测试数据不会污染生产数据

### 匹配规则类型

| 类型 | 常量 | 说明 | 匹配字段 |
|------|------|------|---------|
| 请求头匹配 | ShadowMatchHeader | 匹配 HTTP 请求头 | Key + Values |
| 用户ID匹配 | ShadowMatchUserID | 匹配用户标识 | Extra["user_id"] |
| 租户ID匹配 | ShadowMatchTenantID | 匹配租户标识 | TenantID |
| IP范围匹配 | ShadowMatchIPRange | 匹配IP范围 | Extra["ip"] |
| 百分比分流 | ShadowMatchPercent | 按百分比稳定分流 | TenantID/UserID/RequestID |
| 标签匹配 | ShadowMatchTag | 匹配自定义标签 | Extra["shadow_tag"] |
| 自定义匹配 | ShadowMatchCustom | 自定义匹配函数 | Matcher 函数 |

## 🚀 基本使用

### 1. 注册影子数据源

```go
dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
dbGroup.AddSource(&phantom.DataSource{
    Name:        "primary_db",
    StorageType: phantom.StorageDatabase,
})
dbGroup.AddSource(&phantom.DataSource{
    Name:        "shadow_db",
    StorageType: phantom.StorageDatabase,
    Shadow:      true,  // 标记为影子数据源
})
p.RegisterGroup(dbGroup)
```

### 2. 注册影子规则

```go
p.RegisterShadowRule("db_group", &phantom.ShadowRule{
    Enabled:     true,
    GroupName:   "db_group",
    ShadowDS:    "shadow_db",
    ShadowTable: "shadow_",  // 影子表前缀
    MatchRules: []*phantom.ShadowMatchRule{
        {
            Type:   phantom.ShadowMatchTag,
            Values: []string{"pressure_test"},
        },
    },
    FailSilent: true,  // 影子源不可用时静默回退
})
```

### 3. 识别影子流量

```go
// 普通请求 - 不是影子流量
normalCtx := context.Background()
isShadow := p.IsShadowTraffic(normalCtx, "db_group") // false

// 带标签的请求 - 是影子流量
shadowCtx := phantom.NewRouteContext().WithExtra("shadow_tag", "pressure_test")
ctx := phantom.WithRouteContext(context.Background(), shadowCtx)
isShadow = p.IsShadowTraffic(ctx, "db_group") // true
```

## 📋 匹配规则详解

### 请求头匹配

匹配 HTTP 请求头中的指定字段值：

```go
&phantom.ShadowMatchRule{
    Type:   phantom.ShadowMatchHeader,
    Key:    "X-Shadow",        // 请求头键名
    Values: []string{"true", "1"},  // 匹配值列表
}
```

### 用户ID匹配

匹配路由上下文中的 `user_id` 字段：

```go
&phantom.ShadowMatchRule{
    Type:   phantom.ShadowMatchUserID,
    Values: []string{"user_1", "user_2"},
}

// 设置用户ID
ctx := phantom.NewRouteContext().WithExtra("user_id", "user_1")
```

### 租户ID匹配

匹配路由上下文中的 `TenantID` 字段：

```go
&phantom.ShadowMatchRule{
    Type:   phantom.ShadowMatchTenantID,
    Values: []string{"test_tenant"},
}

// 设置租户ID
ctx := phantom.WithTenant(context.Background(), "test_tenant")
```

### 百分比分流

使用 FNV-1a 哈希实现稳定的百分比分流，确保同一标识的请求始终路由到相同环境：

```go
&phantom.ShadowMatchRule{
    Type:    phantom.ShadowMatchPercent,
    Percent: 10,  // 10% 的流量路由到影子环境
}
```

**稳定分流原理**：基于 TenantID、UserID 或 RequestID 计算 FNV-1a 哈希值，然后对 100 取模。同一标识的请求始终产生相同的哈希值，确保分流结果稳定

**种子优先级**：TenantID > UserID (Extra["user_id"]) > RequestID (Extra["request_id"])

### 标签匹配

匹配路由上下文中的 `shadow_tag` 字段，不区分大小写：

```go
&phantom.ShadowMatchRule{
    Type:   phantom.ShadowMatchTag,
    Values: []string{"pressure_test"},
}

// 设置标签
ctx := phantom.NewRouteContext().WithExtra("shadow_tag", "pressure_test")
```

### 自定义匹配

通过自定义函数实现任意匹配逻辑：

```go
&phantom.ShadowMatchRule{
    Type: phantom.ShadowMatchCustom,
    Matcher: func(ctx context.Context) bool {
        rc := phantom.GetRouteContext(ctx)
        if rc == nil {
            return false
        }
        if v, ok := rc.Extra["force_shadow"]; ok {
            return v.(bool)
        }
        return false
    },
}
```

## 🔗 组合逻辑

多条规则可以通过 AND/OR 逻辑组合：

### OR 逻辑（默认）

任一规则匹配即为影子流量：

```go
&phantom.ShadowRule{
    Enabled: true,
    Logic:   phantom.ShadowLogicOR,  // 默认值
    MatchRules: []*phantom.ShadowMatchRule{
        {Type: phantom.ShadowMatchTenantID, Values: []string{"test_tenant"}},
        {Type: phantom.ShadowMatchTag, Values: []string{"pressure_test"}},
    },
}
// 租户为 test_tenant 或标签为 pressure_test 的请求都是影子流量
```

### AND 逻辑

所有规则匹配才为影子流量：

```go
&phantom.ShadowRule{
    Enabled: true,
    Logic:   phantom.ShadowLogicAND,
    MatchRules: []*phantom.ShadowMatchRule{
        {Type: phantom.ShadowMatchTenantID, Values: []string{"test_tenant"}},
        {Type: phantom.ShadowMatchTag, Values: []string{"pressure_test"}},
    },
}
// 只有同时满足租户为 test_tenant 且标签为 pressure_test 的请求才是影子流量
```

## 🛡️ 自动回退

当影子数据源不可用时，go-phantom 会自动回退到主数据源，确保请求不会因为影子环境故障而失败：

```go
&phantom.ShadowRule{
    Enabled:     true,
    FailSilent:  true,  // 静默回退，不报错
    GroupName:   "db_group",
    ShadowDS:    "shadow_db",
    MatchRules:  []*phantom.ShadowMatchRule{...},
}
```

## 📊 影子表管理

通过 ShadowManager 获取影子表和影子数据源信息：

```go
sm := p.GetShadowManager()

// 获取影子表前缀
prefix := sm.GetShadowTable("db_group")  // "shadow_"

// 获取影子数据源名称
dsName := sm.GetShadowDS("db_group")  // "shadow_db"

// 检查影子功能是否启用
enabled := p.IsShadowEnabled("db_group")  // true
```
