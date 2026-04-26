# 配置驱动

## 🎯 概述

go-phantom 支持通过配置文件或配置结构体驱动引擎初始化，无需手动创建分组和数据源。这种方式特别适合从 YAML/JSON 配置文件加载的场景

## 🚀 基本使用

### 通过配置结构体初始化

```go
config := &phantom.PhantomConfig{
    Groups: []phantom.GroupConfig{
        {
            Name:        "db_group",
            StorageType: phantom.StorageDatabase,
            Strategy:    "readwrite",
            Default:     true,
            Sources: []phantom.SourceConfig{
                {
                    Name:     "primary_db",
                    Weight:   1,
                    ReadOnly: false,
                },
                {
                    Name:     "slave_db",
                    Weight:   1,
                    ReadOnly: true,
                },
            },
        },
        {
            Name:        "redis_group",
            StorageType: phantom.StorageCache,
            Strategy:    "primary",
            Default:     true,
            Sources: []phantom.SourceConfig{
                {
                    Name:   "primary_redis",
                    Weight: 1,
                },
            },
        },
    },
}

builder := phantom.NewConfigDrivenBuilder(config)
p, err := builder.Build()
if err != nil {
    panic(err)
}
p.Initialize(context.Background())
```

### 通过 SourceFactory 注入实例

配置驱动模式下，数据源的 `Instance` 字段需要通过 `SourceFactory` 注入：

```go
factory := func(groupName, sourceName string, storageType phantom.StorageType) (interface{}, error) {
    switch storageType {
    case phantom.StorageDatabase:
        db, err := sql.Open("mysql", "dsn...")
        if err != nil {
            return nil, err
        }
        return db, nil
    case phantom.StorageCache:
        rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
        return rdb, nil
    default:
        return nil, fmt.Errorf("unsupported storage type: %s", storageType)
    }
}

builder := phantom.NewConfigDrivenBuilder(config, phantom.WithSourceFactory(factory))
p, err := builder.Build()
```

### 禁用引擎

```go
config := &phantom.PhantomConfig{
    Disabled: true,
}

builder := phantom.NewConfigDrivenBuilder(config)
p, _ := builder.Build()
// p 为一个空的 Phantom 实例，所有路由操作将返回错误
```

## 📋 配置结构体参考

### PhantomConfig

```go
type PhantomConfig struct {
    Disabled     bool           // 是否禁用引擎
    Groups       []GroupConfig  // 数据源分组配置
    ShadowRules  []ShadowRuleConfig // 影子规则配置
}
```

### GroupConfig

```go
type GroupConfig struct {
    Name        string         // 分组名称
    StorageType StorageType    // 存储类型
    Strategy    string         // 路由策略名称
    Default     bool           // 是否为该存储类型的默认分组
    Sources     []SourceConfig // 数据源配置
}
```

### SourceConfig

```go
type SourceConfig struct {
    Name     string // 数据源名称
    Weight   int    // 权重（用于加权策略）
    ReadOnly bool   // 是否只读
    TenantID string // 租户ID（用于多租户策略）
    Shadow   bool   // 是否影子数据源
}
```

### ShadowRuleConfig

```go
type ShadowRuleConfig struct {
    Enabled     bool                // 是否启用
    GroupName   string              // 分组名称
    ShadowDS    string              // 影子数据源名称
    ShadowTable string              // 影子表前缀
    FailSilent  bool                // 静默回退
    Logic       ShadowLogic         // 组合逻辑 (AND/OR)
    MatchRules  []ShadowMatchConfig // 匹配规则
}
```

### ShadowMatchConfig

```go
type ShadowMatchConfig struct {
    Type    ShadowMatchType // 匹配类型
    Key     string          // 匹配键（请求头匹配时使用）
    Values  []string        // 匹配值列表
    Percent int             // 百分比（百分比分流时使用）
}
```

## 🔄 从 YAML 加载示例

```yaml
phantom:
  disabled: false
  groups:
    - name: db_group
      storage_type: database
      strategy: readwrite
      default: true
      sources:
        - name: primary_db
          weight: 1
          read_only: false
        - name: slave_db
          weight: 1
          read_only: true
    - name: redis_group
      storage_type: cache
      strategy: primary
      default: true
      sources:
        - name: primary_redis
          weight: 1
  shadow_rules:
    - enabled: true
      group_name: db_group
      shadow_ds: shadow_db
      shadow_table: shadow_
      fail_silent: true
      logic: or
      match_rules:
        - type: tag
          values: ["pressure_test"]
        - type: percent
          percent: 10
```

```go
// 从 YAML 文件加载（使用你喜欢的 YAML 库）
data, _ := os.ReadFile("phantom.yaml")
var cfg struct {
    Phantom phantom.PhantomConfig `yaml:"phantom"`
}
yaml.Unmarshal(data, &cfg)

builder := phantom.NewConfigDrivenBuilder(&cfg.Phantom)
p, _ := builder.Build()
p.Initialize(context.Background())
```

## ⚠️ 错误处理

| 场景 | 行为 |
|------|------|
| 配置中分组名称重复 | 返回错误 |
| SourceFactory 返回错误 | 跳过该数据源并记录警告 |
| 策略名称无效 | 使用默认策略 (primary) |
| 影子数据源不存在 | 跳过该影子规则 |
