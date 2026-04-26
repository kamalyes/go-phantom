/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\phantom.go
 * @Description: 幻影引擎核心 - 动态数据隔离引擎的入口和协调器，
 *   整合注册中心、路由器、影子流量管理器和健康检查管理器，
 *   提供声明式数据源切换、按存储类型解析等便捷API
 *   使用 syncx.Map 管理策略和默认分组映射，保证并发安全
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"io"
	"sync"

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Phantom 幻影引擎核心结构体，协调所有子模块
type Phantom struct {
	registry          *Registry                       // 注册中心
	router            *Router                         // 路由器
	shadowManager     *ShadowManager                  // 影子流量管理器
	healthCheckMgr    *HealthCheckManager             // 健康检查管理器
	strategies        *syncx.Map[string, Strategy]    // 策略注册表，使用 syncx.Map 保证并发安全
	defaultGroup      *syncx.Map[StorageType, string] // 存储类型到默认分组的映射
	logger            logger.ILogger
	mu                sync.RWMutex      // 保护初始化状态
	initialized       bool              // 是否已初始化
	healthCheckConfig HealthCheckConfig // 健康检查配置
}

// PhantomOption 幻影引擎配置选项函数
type PhantomOption func(*Phantom)

// WithLogger 设置日志记录器
func WithLogger(log logger.ILogger) PhantomOption {
	return func(p *Phantom) {
		p.logger = log
	}
}

// WithDefaultGroup 设置指定存储类型的默认分组
func WithDefaultGroup(storageType StorageType, groupName string) PhantomOption {
	return func(p *Phantom) {
		p.defaultGroup.Store(storageType, groupName)
	}
}

// WithStrategy 注册自定义策略
func WithStrategy(name string, strategy Strategy) PhantomOption {
	return func(p *Phantom) {
		p.strategies.Store(name, strategy)
	}
}

// WithHealthCheck 设置健康检查配置
func WithHealthCheck(config HealthCheckConfig) PhantomOption {
	return func(p *Phantom) {
		p.healthCheckConfig = config
	}
}

// NewPhantom 创建幻影引擎实例，支持通过选项函数进行配置
func NewPhantom(opts ...PhantomOption) *Phantom {
	p := &Phantom{
		registry:          NewRegistry(nil),
		strategies:        syncx.NewMap[string, Strategy](),
		defaultGroup:      syncx.NewMap[StorageType, string](),
		healthCheckConfig: DefaultHealthCheckConfig(),
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.logger == nil {
		p.logger = logger.New()
	}

	p.registry.logger = p.logger
	p.router = NewRouter(p.registry, p.logger)
	p.shadowManager = NewShadowManager(p.registry, p.logger)
	p.healthCheckMgr = NewHealthCheckManager(p.registry, p.healthCheckConfig, p.logger)

	p.registerBuiltinStrategies()

	return p
}

// registerBuiltinStrategies 注册内置策略
func (p *Phantom) registerBuiltinStrategies() {
	p.strategies.Store("primary", &PrimaryStrategy{})
	p.strategies.Store("readonly", NewReadOnlyStrategy())
	p.strategies.Store("readwrite", NewReadWriteStrategy())
	p.strategies.Store("tenant", NewTenantStrategy(nil))
	p.strategies.Store("roundrobin", NewRoundRobinStrategy())
	p.strategies.Store("weighted", NewWeightedStrategy())
	p.strategies.Store("hint", NewHintStrategy(nil))
	p.strategies.Store("failover", NewFailoverStrategy(3, nil))
}

// Initialize 初始化幻影引擎，启动健康检查等后台任务
func (p *Phantom) Initialize(ctx context.Context) error {
	syncx.WithLock(&p.mu, func() {
		if p.initialized {
			return
		}
		p.initialized = true
	})

	p.healthCheckMgr.Start(ctx)

	p.logger.InfoContext(ctx, "[Phantom] 引擎已初始化")
	return nil
}

// RegisterGroup 注册一个数据源分组
func (p *Phantom) RegisterGroup(group *Group) error {
	return p.registry.RegisterGroup(group)
}

// AddSource 向指定分组添加数据源
func (p *Phantom) AddSource(groupName string, ds *DataSource) error {
	return p.registry.AddSourceToGroup(groupName, ds)
}

// RemoveSource 从指定分组移除数据源
func (p *Phantom) RemoveSource(groupName, sourceName string) error {
	return p.registry.RemoveSourceFromGroup(groupName, sourceName)
}

// SetDefaultGroup 设置指定存储类型的默认分组
func (p *Phantom) SetDefaultGroup(storageType StorageType, groupName string) {
	p.defaultGroup.Store(storageType, groupName)
}

// GetDefaultGroup 获取指定存储类型的默认分组名称
func (p *Phantom) GetDefaultGroup(storageType StorageType) string {
	if v, ok := p.defaultGroup.Load(storageType); ok {
		return v
	}
	return ""
}

// SetGroupStrategy 为指定分组设置路由策略
func (p *Phantom) SetGroupStrategy(groupName, strategyName string) error {
	strategy, ok := p.strategies.Load(strategyName)
	if !ok {
		return ErrStrategyNotFound
	}
	p.router.SetStrategy(groupName, strategy)
	return nil
}

// Resolve 解析数据源 - 根据上下文中的路由信息选择合适的数据源
func (p *Phantom) Resolve(ctx context.Context, groupName string) (*DataSource, error) {
	routeCtx := extractRouteContext(ctx)

	if routeCtx != nil && routeCtx.DSName != "" {
		return p.resolveByDSName(groupName, routeCtx.DSName)
	}

	if routeCtx != nil && routeCtx.Shadow {
		return p.resolveShadow(ctx, groupName, routeCtx)
	}

	return p.router.Route(ctx, groupName, routeCtx)
}

// resolveByDSName 按数据源名称精确解析
func (p *Phantom) resolveByDSName(groupName, dsName string) (*DataSource, error) {
	group, ok := p.registry.GetGroup(groupName)
	if !ok {
		return nil, NewGroupError(groupName, ErrGroupNotFound)
	}

	ds := group.GetSource(dsName)
	if ds == nil {
		return nil, NewSourceError(groupName, dsName, ErrSourceNotFound)
	}

	if !ds.IsHealthy() {
		return nil, NewSourceError(groupName, dsName, ErrSourceUnhealthy)
	}

	return ds, nil
}

// ResolveWithRoute 使用指定的路由上下文解析数据源
func (p *Phantom) ResolveWithRoute(ctx context.Context, groupName string, routeCtx *RouteContext) (*DataSource, error) {
	if routeCtx != nil && routeCtx.DSName != "" {
		return p.resolveByDSName(groupName, routeCtx.DSName)
	}

	if routeCtx != nil && routeCtx.Shadow {
		return p.resolveShadow(ctx, groupName, routeCtx)
	}
	return p.router.Route(ctx, groupName, routeCtx)
}

// ResolveByType 按存储类型解析数据源，使用该类型的默认分组
func (p *Phantom) ResolveByType(ctx context.Context, storageType StorageType) (*DataSource, error) {
	groupName, ok := p.defaultGroup.Load(storageType)
	if !ok || groupName == "" {
		return nil, ErrNoDefaultGroup
	}

	return p.Resolve(ctx, groupName)
}

// resolveShadow 解析影子数据源，无可用影子源时回退到普通路由
func (p *Phantom) resolveShadow(ctx context.Context, groupName string, routeCtx *RouteContext) (*DataSource, error) {
	group, ok := p.registry.GetGroup(groupName)
	if !ok {
		return nil, NewGroupError(groupName, ErrGroupNotFound)
	}

	shadows := group.GetHealthyShadows()
	if len(shadows) == 0 {
		p.logger.WarnContext(ctx, "[Phantom] 分组 %q 中无健康的影子数据源，回退到主库", groupName)
		return p.router.Route(ctx, groupName, routeCtx)
	}

	return shadows[0], nil
}

// GetDB 便捷方法 - 获取数据库类型的数据源
func (p *Phantom) GetDB(ctx context.Context, groupNames ...string) (*DataSource, error) {
	groupName := p.getDefaultGroupName(StorageDatabase, groupNames...)
	return p.Resolve(ctx, groupName)
}

// GetRedis 便捷方法 - 获取Redis类型的数据源
func (p *Phantom) GetRedis(ctx context.Context, groupNames ...string) (*DataSource, error) {
	groupName := p.getDefaultGroupName(StorageRedis, groupNames...)
	return p.Resolve(ctx, groupName)
}

// GetCustom 便捷方法 - 获取自定义类型的数据源
func (p *Phantom) GetCustom(ctx context.Context, storageType StorageType, groupNames ...string) (*DataSource, error) {
	groupName := p.getDefaultGroupName(storageType, groupNames...)
	return p.Resolve(ctx, groupName)
}

// getDefaultGroupName 获取默认分组名称，优先使用传入的分组名
func (p *Phantom) getDefaultGroupName(storageType StorageType, groupNames ...string) string {
	if len(groupNames) > 0 && groupNames[0] != "" {
		return groupNames[0]
	}
	return p.GetDefaultGroup(storageType)
}

// RegisterShadowRule 注册影子流量规则
func (p *Phantom) RegisterShadowRule(groupName string, rule *ShadowRule) {
	p.shadowManager.RegisterRule(groupName, rule)
}

// IsShadowTraffic 判断当前请求是否为影子流量
func (p *Phantom) IsShadowTraffic(ctx context.Context, groupName string) bool {
	return p.shadowManager.IsShadowTraffic(ctx, groupName)
}

// IsShadowEnabled 检查指定分组是否启用了影子流量
func (p *Phantom) IsShadowEnabled(groupName string) bool {
	return p.shadowManager.IsShadowEnabled(groupName)
}

// GetRegistry 获取注册中心引用
func (p *Phantom) GetRegistry() *Registry {
	return p.registry
}

// GetRouter 获取路由器引用
func (p *Phantom) GetRouter() *Router {
	return p.router
}

// GetShadowManager 获取影子流量管理器引用
func (p *Phantom) GetShadowManager() *ShadowManager {
	return p.shadowManager
}

// GetHealthCheckManager 获取健康检查管理器引用
func (p *Phantom) GetHealthCheckManager() *HealthCheckManager {
	return p.healthCheckMgr
}

// HealthCheck 执行全量健康检查，返回每个分组中每个数据源的健康状态
func (p *Phantom) HealthCheck() map[string]map[string]bool {
	return p.registry.HealthCheckAll()
}

// RegisterHealthChecker 注册自定义健康检查器
func (p *Phantom) RegisterHealthChecker(storageType StorageType, checker HealthChecker) {
	p.healthCheckMgr.RegisterChecker(storageType, checker)
}

// Close 优雅关闭幻影引擎，停止健康检查并关闭所有数据源连接
func (p *Phantom) Close() error {
	syncx.WithLock(&p.mu, func() {
		if p.healthCheckMgr != nil {
			p.healthCheckMgr.Stop()
		}

		p.registry.ForEach(func(name string, group *Group) {
			syncx.WithRLock(&group.mu, func() {
				for _, ds := range group.Sources {
					if ds.Instance != nil {
						if closer, ok := ds.Instance.(io.Closer); ok {
							if err := closer.Close(); err != nil {
								p.logger.WarnContext(context.Background(), "[Phantom] 关闭数据源 %s 失败: %v", ds.Name, err)
							}
						}
					}
				}
			})
		})

		p.logger.InfoContext(context.Background(), "[Phantom] 引擎已关闭")
		p.initialized = false
	})
	return nil
}
