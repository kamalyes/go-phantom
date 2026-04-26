/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\router.go
 * @Description: 幻影引擎路由策略 - 提供多种数据源选择策略，
 *   包括主库、只读、读写分离、租户、轮询、加权、提示和故障转移策略
 *   使用 random.RandInt 替代 math/rand 实现协程安全的随机数生成，
 *   使用 atomic 计数器实现无锁轮询
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"sync/atomic"

	"github.com/kamalyes/go-toolbox/pkg/random"
)

var weightedRandInt = random.RandInt

// RouteResult 路由结果，包含选中的数据源和所属分组信息
type RouteResult struct {
	Source    *DataSource // 选中的数据源
	IsShadow  bool        // 是否为影子数据源
	GroupName string      // 所属分组名称
}

// Strategy 路由策略接口，定义数据源选择的核心抽象
type Strategy interface {
	Resolve(ctx context.Context, group *Group, routeCtx *RouteContext) (*RouteResult, error)
	Name() string
}

// PrimaryStrategy 主库策略 - 始终选择分组中的主数据源
type PrimaryStrategy struct{}

// Resolve 选择主数据源，主库不存在或不健康时返回错误
func (s *PrimaryStrategy) Resolve(_ context.Context, group *Group, _ *RouteContext) (*RouteResult, error) {
	if group.Primary == nil {
		return nil, NewGroupError(group.Name, ErrNoPrimary)
	}
	if !group.Primary.IsHealthy() {
		return nil, NewSourceError(group.Name, group.Primary.Name, ErrSourceUnhealthy)
	}
	return &RouteResult{Source: group.Primary, GroupName: group.Name}, nil
}

// Name 返回策略名称
func (s *PrimaryStrategy) Name() string { return "primary" }

// ReadOnlyStrategy 只读策略 - 从只读数据源中随机选择，无只读源时回退到主库
type ReadOnlyStrategy struct{}

// NewReadOnlyStrategy 创建只读策略
func NewReadOnlyStrategy() *ReadOnlyStrategy {
	return &ReadOnlyStrategy{}
}

// Resolve 从只读数据源中随机选择一个，无可用只读源时回退到主库
func (s *ReadOnlyStrategy) Resolve(_ context.Context, group *Group, _ *RouteContext) (*RouteResult, error) {
	readOnlySources := group.GetReadOnlySources()
	if len(readOnlySources) > 0 {
		idx := random.RandInt(0, len(readOnlySources))
		return &RouteResult{Source: readOnlySources[idx], GroupName: group.Name}, nil
	}

	if group.Primary != nil && group.Primary.IsHealthy() {
		return &RouteResult{Source: group.Primary, GroupName: group.Name}, nil
	}

	return nil, NewGroupError(group.Name, ErrNoReadSource)
}

// Name 返回策略名称
func (s *ReadOnlyStrategy) Name() string { return "readonly" }

// ReadWriteStrategy 读写分离策略 - 读请求路由到只读源，写请求路由到主库
type ReadWriteStrategy struct {
	writeStrategy Strategy
	readStrategy  Strategy
}

// NewReadWriteStrategy 创建读写分离策略
func NewReadWriteStrategy() *ReadWriteStrategy {
	return &ReadWriteStrategy{
		writeStrategy: &PrimaryStrategy{},
		readStrategy:  NewReadOnlyStrategy(),
	}
}

// Resolve 根据路由上下文中的只读标记选择读策略或写策略
func (s *ReadWriteStrategy) Resolve(ctx context.Context, group *Group, routeCtx *RouteContext) (*RouteResult, error) {
	if routeCtx != nil && routeCtx.ReadOnly {
		return s.readStrategy.Resolve(ctx, group, routeCtx)
	}
	return s.writeStrategy.Resolve(ctx, group, routeCtx)
}

// Name 返回策略名称
func (s *ReadWriteStrategy) Name() string { return "readwrite" }

// TenantStrategy 租户策略 - 优先匹配租户专属数据源，未匹配则回退
type TenantStrategy struct {
	fallback Strategy
}

// NewTenantStrategy 创建租户策略，fallback 为空时默认使用主库策略
func NewTenantStrategy(fallback Strategy) *TenantStrategy {
	if fallback == nil {
		fallback = &PrimaryStrategy{}
	}
	return &TenantStrategy{fallback: fallback}
}

// Resolve 根据租户ID匹配专属数据源，未匹配则使用回退策略
// 使用 Group.tenantMap 实现 O(1) 查找，替代 O(n) 线性遍历
func (s *TenantStrategy) Resolve(ctx context.Context, group *Group, routeCtx *RouteContext) (*RouteResult, error) {
	if routeCtx != nil && routeCtx.TenantID != "" {
		ds := group.GetSourceByTenant(routeCtx.TenantID)
		if ds != nil && ds.IsHealthy() {
			return &RouteResult{Source: ds, GroupName: group.Name}, nil
		}
	}
	return s.fallback.Resolve(ctx, group, routeCtx)
}

// Name 返回策略名称
func (s *TenantStrategy) Name() string { return "tenant" }

// RoundRobinStrategy 轮询策略 - 使用原子计数器实现无锁轮询选择
type RoundRobinStrategy struct {
	counter  uint64 // 原子计数器，无需互斥锁
	fallback Strategy
}

// NewRoundRobinStrategy 创建轮询策略
func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{fallback: &PrimaryStrategy{}}
}

// Resolve 以轮询方式选择健康数据源
func (s *RoundRobinStrategy) Resolve(_ context.Context, group *Group, _ *RouteContext) (*RouteResult, error) {
	healthy := group.GetHealthySources()
	if len(healthy) == 0 {
		return nil, NewGroupError(group.Name, ErrNoAvailableSource)
	}

	idx := atomic.AddUint64(&s.counter, 1) % uint64(len(healthy))
	return &RouteResult{Source: healthy[idx], GroupName: group.Name}, nil
}

// Name 返回策略名称
func (s *RoundRobinStrategy) Name() string { return "roundrobin" }

// WeightedStrategy 加权策略 - 按数据源权重进行加权随机选择
type WeightedStrategy struct{}

// NewWeightedStrategy 创建加权策略
func NewWeightedStrategy() *WeightedStrategy {
	return &WeightedStrategy{}
}

// Resolve 根据数据源权重进行加权随机选择，权重为0或负数时默认为1
func (s *WeightedStrategy) Resolve(_ context.Context, group *Group, _ *RouteContext) (*RouteResult, error) {
	healthy := group.GetHealthySources()
	if len(healthy) == 0 {
		return nil, NewGroupError(group.Name, ErrNoAvailableSource)
	}

	if len(healthy) == 1 {
		return &RouteResult{Source: healthy[0], GroupName: group.Name}, nil
	}

	totalWeight := 0
	for _, ds := range healthy {
		w := ds.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	r := weightedRandInt(0, totalWeight)

	cumulative := 0
	for _, ds := range healthy {
		w := ds.Weight
		if w <= 0 {
			w = 1
		}
		cumulative += w
		if r < cumulative {
			return &RouteResult{Source: ds, GroupName: group.Name}, nil
		}
	}

	return &RouteResult{Source: healthy[0], GroupName: group.Name}, nil
}

// Name 返回策略名称
func (s *WeightedStrategy) Name() string { return "weighted" }

// HintStrategy 提示策略 - 根据路由提示精确匹配数据源，未匹配则回退
type HintStrategy struct {
	fallback Strategy
}

// NewHintStrategy 创建提示策略，fallback 为空时默认使用主库策略
func NewHintStrategy(fallback Strategy) *HintStrategy {
	if fallback == nil {
		fallback = &PrimaryStrategy{}
	}
	return &HintStrategy{fallback: fallback}
}

// Resolve 根据路由提示匹配数据源，未匹配则使用回退策略
func (s *HintStrategy) Resolve(ctx context.Context, group *Group, routeCtx *RouteContext) (*RouteResult, error) {
	if routeCtx != nil && routeCtx.RouteHint != "" {
		if ds := group.GetSource(routeCtx.RouteHint); ds != nil && ds.IsHealthy() {
			return &RouteResult{Source: ds, GroupName: group.Name}, nil
		}
	}
	return s.fallback.Resolve(ctx, group, routeCtx)
}

// Name 返回策略名称
func (s *HintStrategy) Name() string { return "hint" }

// FailoverStrategy 故障转移策略 - 尝试回退策略失败后遍历健康数据源
type FailoverStrategy struct {
	maxRetries int
	fallback   Strategy
}

// NewFailoverStrategy 创建故障转移策略，maxRetries <= 0 时默认为3次
func NewFailoverStrategy(maxRetries int, fallback Strategy) *FailoverStrategy {
	if fallback == nil {
		fallback = &PrimaryStrategy{}
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &FailoverStrategy{maxRetries: maxRetries, fallback: fallback}
}

// Resolve 先尝试回退策略，失败后遍历健康数据源寻找可用源
func (s *FailoverStrategy) Resolve(ctx context.Context, group *Group, routeCtx *RouteContext) (*RouteResult, error) {
	healthy := group.GetHealthySources()
	if len(healthy) == 0 {
		return nil, NewGroupError(group.Name, ErrNoAvailableSource)
	}

	for i := 0; i < s.maxRetries && i < len(healthy); i++ {
		result, err := s.fallback.Resolve(ctx, group, routeCtx)
		if err == nil && result != nil {
			return result, nil
		}
	}

	for i := 0; i < len(healthy); i++ {
		ds := healthy[i]
		if ds.IsHealthy() {
			return &RouteResult{Source: ds, GroupName: group.Name}, nil
		}
	}

	return nil, NewGroupError(group.Name, ErrNoAvailableSource)
}

// Name 返回策略名称
func (s *FailoverStrategy) Name() string { return "failover" }
