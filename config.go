/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\config.go
 * @Description: 幻影引擎配置驱动构建 - 支持通过配置文件（YAML/JSON）驱动引擎初始化，
 *   包括分组、数据源、影子规则和健康检查等完整配置
 *   提供 ConfigDrivenBuilder 构建器模式，支持自定义数据源工厂
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"time"

	"github.com/kamalyes/go-logger"
)

// PhantomConfig 幻影引擎顶层配置
type PhantomConfig struct {
	Enabled       bool                     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                     // 是否启用幻影引擎
	HealthCheck   PhantomHealthCheckConfig `mapstructure:"health-check" yaml:"health-check" json:"healthCheck"`       // 健康检查配置
	Groups        []PhantomGroupConfig     `mapstructure:"groups" yaml:"groups" json:"groups"`                        // 分组配置列表
	ShadowRules   []PhantomShadowConfig    `mapstructure:"shadow-rules" yaml:"shadow-rules" json:"shadowRules"`       // 影子规则配置列表
	DefaultGroups map[StorageType]string   `mapstructure:"default-groups" yaml:"default-groups" json:"defaultGroups"` // 存储类型到默认分组的映射
}

// PhantomHealthCheckConfig 健康检查配置
type PhantomHealthCheckConfig struct {
	Enabled  bool          `mapstructure:"enabled" yaml:"enabled" json:"enabled"`    // 是否启用
	Interval time.Duration `mapstructure:"interval" yaml:"interval" json:"interval"` // 检查间隔
	Timeout  time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`    // 单次检查超时
}

// PhantomGroupConfig 分组配置
type PhantomGroupConfig struct {
	Name        string                `mapstructure:"name" yaml:"name" json:"name"`                        // 分组名称
	StorageType StorageType           `mapstructure:"storage-type" yaml:"storage-type" json:"storageType"` // 存储类型
	Strategy    string                `mapstructure:"strategy" yaml:"strategy" json:"strategy"`            // 路由策略名称
	Sources     []PhantomSourceConfig `mapstructure:"sources" yaml:"sources" json:"sources"`               // 数据源配置列表
}

// PhantomSourceConfig 数据源配置
type PhantomSourceConfig struct {
	Name     string `mapstructure:"name" yaml:"name" json:"name"`               // 数据源名称
	Shadow   bool   `mapstructure:"shadow" yaml:"shadow" json:"shadow"`         // 是否为影子数据源
	ReadOnly bool   `mapstructure:"readonly" yaml:"readonly" json:"readonly"`   // 是否为只读数据源
	TenantID string `mapstructure:"tenant-id" yaml:"tenant-id" json:"tenantId"` // 租户ID
	Weight   int    `mapstructure:"weight" yaml:"weight" json:"weight"`         // 权重
	DSN      string `mapstructure:"dsn" yaml:"dsn" json:"dsn"`                  // 数据源连接字符串
}

// PhantomShadowConfig 影子规则配置
type PhantomShadowConfig struct {
	GroupName   string                   `mapstructure:"group-name" yaml:"group-name" json:"groupName"`       // 所属分组名称
	Enabled     bool                     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`               // 是否启用
	Logic       string                   `mapstructure:"logic" yaml:"logic" json:"logic"`                     // 组合逻辑（and/or）
	ShadowDS    string                   `mapstructure:"shadow-ds" yaml:"shadow-ds" json:"shadowDs"`          // 影子数据源名称
	ShadowTable string                   `mapstructure:"shadow-table" yaml:"shadow-table" json:"shadowTable"` // 影子表名
	FailSilent  bool                     `mapstructure:"fail-silent" yaml:"fail-silent" json:"failSilent"`    // 失败时是否静默
	MatchRules  []PhantomMatchRuleConfig `mapstructure:"match-rules" yaml:"match-rules" json:"matchRules"`    // 匹配规则列表
}

// PhantomMatchRuleConfig 匹配规则配置
type PhantomMatchRuleConfig struct {
	Type    ShadowMatchType `mapstructure:"type" yaml:"type" json:"type"`          // 匹配类型
	Key     string          `mapstructure:"key" yaml:"key" json:"key"`             // 匹配键
	Values  []string        `mapstructure:"values" yaml:"values" json:"values"`    // 匹配值列表
	Percent int             `mapstructure:"percent" yaml:"percent" json:"percent"` // 百分比
}

// DefaultPhantomConfig 返回默认幻影引擎配置
func DefaultPhantomConfig() *PhantomConfig {
	return &PhantomConfig{
		Enabled: false,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups:        []PhantomGroupConfig{},
		ShadowRules:   []PhantomShadowConfig{},
		DefaultGroups: map[StorageType]string{},
	}
}

// SourceFactory 数据源工厂函数，根据配置创建实际存储实例
type SourceFactory func(ctx context.Context, cfg PhantomSourceConfig) (interface{}, error)

// ConfigDrivenBuilder 配置驱动的幻影引擎构建器
type ConfigDrivenBuilder struct {
	config        *PhantomConfig // 引擎配置
	sourceFactory SourceFactory  // 数据源工厂
	phantom       *Phantom       // 构建完成的引擎实例
	logger        interface{}    // 日志记录器（延迟类型断言）
}

// NewConfigDrivenBuilder 创建配置驱动的构建器
func NewConfigDrivenBuilder(config *PhantomConfig) *ConfigDrivenBuilder {
	return &ConfigDrivenBuilder{
		config: config,
	}
}

// WithSourceFactory 设置数据源工厂
func (b *ConfigDrivenBuilder) WithSourceFactory(factory SourceFactory) *ConfigDrivenBuilder {
	b.sourceFactory = factory
	return b
}

// WithLogger 设置日志记录器
func (b *ConfigDrivenBuilder) WithLogger(log interface{}) *ConfigDrivenBuilder {
	b.logger = log
	return b
}

// Build 根据配置构建幻影引擎实例
func (b *ConfigDrivenBuilder) Build(ctx context.Context) (*Phantom, error) {
	if !b.config.Enabled {
		return NewPhantom(), nil
	}

	var opts []PhantomOption

	if b.logger != nil {
		if log, ok := b.logger.(logger.ILogger); ok {
			opts = append(opts, WithLogger(log))
		}
	}

	hcConfig := HealthCheckConfig{
		Enabled:  b.config.HealthCheck.Enabled,
		Interval: b.config.HealthCheck.Interval,
		Timeout:  b.config.HealthCheck.Timeout,
	}
	if hcConfig.Interval == 0 {
		hcConfig.Interval = 10 * time.Second
	}
	if hcConfig.Timeout == 0 {
		hcConfig.Timeout = 3 * time.Second
	}
	opts = append(opts, WithHealthCheck(hcConfig))

	for storageType, groupName := range b.config.DefaultGroups {
		opts = append(opts, WithDefaultGroup(storageType, groupName))
	}

	p := NewPhantom(opts...)

	for _, groupCfg := range b.config.Groups {
		group := NewGroup(groupCfg.Name, groupCfg.StorageType)

		for _, srcCfg := range groupCfg.Sources {
			ds := &DataSource{
				Name:        srcCfg.Name,
				StorageType: groupCfg.StorageType,
				Shadow:      srcCfg.Shadow,
				ReadOnly:    srcCfg.ReadOnly,
				TenantID:    srcCfg.TenantID,
				Weight:      srcCfg.Weight,
			}
			ds.Healthy.Store(true)

			if b.sourceFactory != nil {
				instance, err := b.sourceFactory(ctx, srcCfg)
				if err != nil {
					return nil, NewSourceError(groupCfg.Name, srcCfg.Name, err)
				}
				ds.Instance = instance
			}

			group.AddSource(ds)
		}

		if err := p.RegisterGroup(group); err != nil {
			return nil, err
		}

		if groupCfg.Strategy != "" {
			_ = p.SetGroupStrategy(groupCfg.Name, groupCfg.Strategy)
		}
	}

	for _, shadowCfg := range b.config.ShadowRules {
		logic := ShadowLogic(shadowCfg.Logic)
		matchRules := make([]*ShadowMatchRule, 0, len(shadowCfg.MatchRules))
		for _, ruleCfg := range shadowCfg.MatchRules {
			matchRules = append(matchRules, &ShadowMatchRule{
				Type:    ruleCfg.Type,
				Key:     ruleCfg.Key,
				Values:  ruleCfg.Values,
				Percent: ruleCfg.Percent,
			})
		}

		rule := &ShadowRule{
			Enabled:     shadowCfg.Enabled,
			Logic:       logic,
			GroupName:   shadowCfg.GroupName,
			ShadowDS:    shadowCfg.ShadowDS,
			ShadowTable: shadowCfg.ShadowTable,
			FailSilent:  shadowCfg.FailSilent,
			MatchRules:  matchRules,
		}
		p.RegisterShadowRule(shadowCfg.GroupName, rule)
	}

	p.Initialize(ctx)

	b.phantom = p
	return p, nil
}

// GetPhantom 获取构建完成的幻影引擎实例
func (b *ConfigDrivenBuilder) GetPhantom() *Phantom {
	return b.phantom
}
