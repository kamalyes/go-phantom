/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\registry.go
 * @Description: 幻影引擎注册中心 - 管理数据源分组和数据源的注册、查询与移除
 *   使用 syncx.Map 实现分组级别的并发安全，使用 sourceMap 实现数据源 O(1) 查找，
 *   使用 healthyCache 缓存健康数据源列表以减少重复过滤开销
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// StorageType 存储类型标识
type StorageType string

const (
	StorageDatabase StorageType = "database" // 关系型数据库
	StorageRedis    StorageType = "redis"    // Redis 缓存
	StorageCustom   StorageType = "custom"   // 自定义存储
)

// DataSource 数据源定义，描述一个可路由的数据源实例
type DataSource struct {
	Name        string      // 数据源名称，在分组内唯一
	StorageType StorageType // 存储类型
	Instance    interface{} // 实际存储实例（如 *gorm.DB、*redis.Client）
	Shadow      bool        // 是否为影子数据源
	ReadOnly    bool        // 是否为只读数据源
	TenantID    string      // 所属租户ID
	Weight      int         // 权重，用于加权路由策略
	Healthy     atomic.Bool // 健康状态，使用原子操作保证并发安全
}

// IsHealthy 检查数据源是否健康
func (ds *DataSource) IsHealthy() bool {
	return ds.Healthy.Load()
}

// MarkHealthy 标记数据源为健康状态
func (ds *DataSource) MarkHealthy() {
	ds.Healthy.Store(true)
}

// MarkUnhealthy 标记数据源为不健康状态
func (ds *DataSource) MarkUnhealthy() {
	ds.Healthy.Store(false)
}

// Group 数据源分组，管理同一类型下的多个数据源
type Group struct {
	Name        string        // 分组名称
	StorageType StorageType   // 存储类型
	Sources     []*DataSource // 全部数据源列表
	Primary     *DataSource   // 主数据源（非只读、非影子的第一个数据源）
	Shadows     []*DataSource // 影子数据源列表

	sourceMap     map[string]*DataSource // 数据源名称到实例的映射，O(1) 查找
	tenantMap     map[string]*DataSource // 租户ID到数据源的映射，O(1) 租户查找
	healthyCache  []*DataSource          // 健康数据源缓存
	shadowCache   []*DataSource          // 健康影子数据源缓存
	readOnlyCache []*DataSource          // 只读数据源缓存
	dirty         atomic.Bool            // 缓存脏标记，数据源变更时置为 true
	mu            sync.RWMutex           // 读写锁，保护分组内部状态
}

// NewGroup 创建一个新的数据源分组
func NewGroup(name string, storageType StorageType) *Group {
	return &Group{
		Name:        name,
		StorageType: storageType,
		sourceMap:   make(map[string]*DataSource),
		tenantMap:   make(map[string]*DataSource),
	}
}

// AddSource 向分组中添加数据源，自动维护 sourceMap、tenantMap、Primary 和 Shadows
func (g *Group) AddSource(ds *DataSource) {
	syncx.WithLock(&g.mu, func() {
		if g.sourceMap == nil {
			g.sourceMap = make(map[string]*DataSource)
		}
		if g.tenantMap == nil {
			g.tenantMap = make(map[string]*DataSource)
		}

		ds.Healthy.Store(true)
		g.Sources = append(g.Sources, ds)
		g.sourceMap[ds.Name] = ds

		if ds.TenantID != "" {
			g.tenantMap[ds.TenantID] = ds
		}

		if ds.Shadow {
			g.Shadows = append(g.Shadows, ds)
		} else if g.Primary == nil && !ds.ReadOnly {
			g.Primary = ds
		}

		g.dirty.Store(true)
	})
}

// RemoveSource 从分组中移除指定名称的数据源，自动更新 Primary 和 tenantMap
func (g *Group) RemoveSource(name string) {
	syncx.WithLock(&g.mu, func() {
		var removed *DataSource
		for i, ds := range g.Sources {
			if ds.Name == name {
				removed = ds
				g.Sources = append(g.Sources[:i], g.Sources[i+1:]...)
				break
			}
		}

		for i, ds := range g.Shadows {
			if ds.Name == name {
				g.Shadows = append(g.Shadows[:i], g.Shadows[i+1:]...)
				break
			}
		}

		if g.sourceMap != nil {
			delete(g.sourceMap, name)
		}

		if removed != nil && removed.TenantID != "" && g.tenantMap != nil {
			delete(g.tenantMap, removed.TenantID)
		}

		if g.Primary != nil && g.Primary.Name == name {
			g.Primary = nil
			for _, ds := range g.Sources {
				if !ds.ReadOnly && !ds.Shadow {
					g.Primary = ds
					break
				}
			}
		}

		g.dirty.Store(true)
	})
}

// GetSource 按名称查找数据源，使用 sourceMap 实现 O(1) 查找
func (g *Group) GetSource(name string) *DataSource {
	return syncx.WithRLockReturnValue(&g.mu, func() *DataSource {
		if g.sourceMap == nil {
			return nil
		}
		return g.sourceMap[name]
	})
}

// GetSourceByTenant 按租户ID查找数据源，使用 tenantMap 实现 O(1) 查找
func (g *Group) GetSourceByTenant(tenantID string) *DataSource {
	return syncx.WithRLockReturnValue(&g.mu, func() *DataSource {
		if g.tenantMap == nil {
			return nil
		}
		return g.tenantMap[tenantID]
	})
}

// TenantCount 返回分组中租户数据源的数量
func (g *Group) TenantCount() int {
	return syncx.WithRLockReturnValue(&g.mu, func() int {
		return len(g.tenantMap)
	})
}

// GetHealthySources 获取分组中所有健康的数据源，使用缓存减少重复过滤
func (g *Group) GetHealthySources() []*DataSource {
	if !g.dirty.Load() {
		cache := syncx.WithRLockReturnValue(&g.mu, func() []*DataSource {
			return g.healthyCache
		})
		if cache != nil {
			return cache
		}
	}

	return syncx.WithLockReturnValue(&g.mu, func() []*DataSource {
		if g.healthyCache != nil && !g.dirty.Load() {
			return g.healthyCache
		}

		result := make([]*DataSource, 0, len(g.Sources))
		for _, ds := range g.Sources {
			if ds.IsHealthy() {
				result = append(result, ds)
			}
		}
		g.healthyCache = result
		g.dirty.Store(false)
		return result
	})
}

// GetHealthyShadows 获取分组中所有健康的影子数据源
func (g *Group) GetHealthyShadows() []*DataSource {
	if !g.dirty.Load() {
		cache := syncx.WithRLockReturnValue(&g.mu, func() []*DataSource {
			return g.shadowCache
		})
		if cache != nil {
			return cache
		}
	}

	return syncx.WithLockReturnValue(&g.mu, func() []*DataSource {
		if g.shadowCache != nil && !g.dirty.Load() {
			return g.shadowCache
		}

		result := make([]*DataSource, 0, len(g.Shadows))
		for _, ds := range g.Shadows {
			if ds.IsHealthy() {
				result = append(result, ds)
			}
		}
		g.shadowCache = result
		g.dirty.Store(false)
		return result
	})
}

// GetReadOnlySources 获取分组中所有健康的只读数据源
func (g *Group) GetReadOnlySources() []*DataSource {
	if !g.dirty.Load() {
		cache := syncx.WithRLockReturnValue(&g.mu, func() []*DataSource {
			return g.readOnlyCache
		})
		if cache != nil {
			return cache
		}
	}

	return syncx.WithLockReturnValue(&g.mu, func() []*DataSource {
		if g.readOnlyCache != nil && !g.dirty.Load() {
			return g.readOnlyCache
		}

		result := make([]*DataSource, 0)
		for _, ds := range g.Sources {
			if ds.ReadOnly && ds.IsHealthy() {
				result = append(result, ds)
			}
		}
		g.readOnlyCache = result
		g.dirty.Store(false)
		return result
	})
}

// InvalidateCache 使缓存失效，在数据源健康状态变更时调用
func (g *Group) InvalidateCache() {
	g.dirty.Store(true)
}

// Registry 注册中心，管理所有数据源分组
type Registry struct {
	groups *syncx.Map[string, *Group] // 使用 syncx.Map 实现并发安全的分组存储
	logger logger.ILogger
}

// NewRegistry 创建一个新的注册中心
func NewRegistry(log logger.ILogger) *Registry {
	return &Registry{
		groups: syncx.NewMap[string, *Group](),
		logger: log,
	}
}

// RegisterGroup 注册一个数据源分组，重复注册返回 ErrGroupExists
func (r *Registry) RegisterGroup(group *Group) error {
	if _, loaded := r.groups.LoadOrStore(group.Name, group); loaded {
		return NewGroupError(group.Name, ErrGroupExists)
	}

	if r.logger != nil {
		r.logger.InfoContext(context.Background(), "[Phantom] 分组已注册: %s, 类型: %s, 数据源数: %d", group.Name, group.StorageType, len(group.Sources))
	}
	return nil
}

// GetGroup 按名称获取数据源分组
func (r *Registry) GetGroup(name string) (*Group, bool) {
	return r.groups.Load(name)
}

// RemoveGroup 按名称移除数据源分组
func (r *Registry) RemoveGroup(name string) {
	r.groups.Delete(name)
}

// ListGroups 列出所有已注册的分组名称
func (r *Registry) ListGroups() []string {
	return r.groups.Keys()
}

// ListGroupsByType 列出指定存储类型的所有分组名称
func (r *Registry) ListGroupsByType(storageType StorageType) []string {
	return r.groups.FilterKeys(func(_ string, group *Group) bool {
		return group.StorageType == storageType
	})
}

// AddSourceToGroup 向指定分组添加数据源
func (r *Registry) AddSourceToGroup(groupName string, ds *DataSource) error {
	g, ok := r.groups.Load(groupName)
	if !ok {
		return NewGroupError(groupName, ErrGroupNotFound)
	}

	g.AddSource(ds)

	if r.logger != nil {
		r.logger.InfoContext(context.Background(), "[Phantom] 数据源已添加: %s 到分组 %s (影子=%v, 只读=%v)", ds.Name, groupName, ds.Shadow, ds.ReadOnly)
	}
	return nil
}

// RemoveSourceFromGroup 从指定分组移除数据源
func (r *Registry) RemoveSourceFromGroup(groupName, sourceName string) error {
	g, ok := r.groups.Load(groupName)
	if !ok {
		return NewGroupError(groupName, ErrGroupNotFound)
	}

	g.RemoveSource(sourceName)
	return nil
}

// HealthCheckAll 执行全量健康检查，返回每个分组中每个数据源的健康状态
func (r *Registry) HealthCheckAll() map[string]map[string]bool {
	result := make(map[string]map[string]bool)
	r.groups.ForEach(func(name string, group *Group) {
		syncx.WithRLock(&group.mu, func() {
			sources := make(map[string]bool, len(group.Sources))
			for _, ds := range group.Sources {
				sources[ds.Name] = ds.IsHealthy()
			}
			result[name] = sources
		})
	})
	return result
}

// ForEach 遍历所有分组执行指定函数
func (r *Registry) ForEach(fn func(name string, group *Group)) {
	r.groups.ForEach(fn)
}
