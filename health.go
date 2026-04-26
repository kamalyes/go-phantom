/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\health.go
 * @Description: 幻影引擎健康检查管理 - 定期检测数据源健康状态，
 *   自动标记不健康的数据源并刷新缓存
 *   内置数据库、Redis和通用健康检查器，支持自定义扩展
 *   使用 syncx.Map 管理检查器映射，使用 syncx.WithLock 保护运行状态
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"sync"
	"time"

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// HealthChecker 健康检查器接口，定义数据源健康检查的抽象
type HealthChecker interface {
	Check(ctx context.Context, ds *DataSource) bool
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled  bool          // 是否启用健康检查
	Interval time.Duration // 检查间隔
	Timeout  time.Duration // 单次检查超时时间
}

// DefaultHealthCheckConfig 返回默认健康检查配置
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		Enabled:  true,
		Interval: 10 * time.Second,
		Timeout:  3 * time.Second,
	}
}

// HealthCheckManager 健康检查管理器
type HealthCheckManager struct {
	registry *Registry                              // 注册中心引用
	checkers *syncx.Map[StorageType, HealthChecker] // 存储类型到检查器的映射
	config   HealthCheckConfig                      // 健康检查配置
	logger   logger.ILogger
	cancel   context.CancelFunc // 取消函数
	mu       sync.RWMutex       // 保护运行状态
	running  bool               // 是否正在运行
}

// NewHealthCheckManager 创建健康检查管理器
func NewHealthCheckManager(registry *Registry, config HealthCheckConfig, log logger.ILogger) *HealthCheckManager {
	hcm := &HealthCheckManager{
		registry: registry,
		checkers: syncx.NewMap[StorageType, HealthChecker](),
		config:   config,
		logger:   log,
	}
	hcm.registerBuiltinCheckers()
	return hcm
}

// registerBuiltinCheckers 注册内置健康检查器
func (hcm *HealthCheckManager) registerBuiltinCheckers() {
	hcm.checkers.Store(StorageDatabase, &DBHealthChecker{})
	hcm.checkers.Store(StorageRedis, &RedisHealthChecker{})
	hcm.checkers.Store(StorageCustom, &GenericHealthChecker{})
}

// RegisterChecker 注册自定义健康检查器
func (hcm *HealthCheckManager) RegisterChecker(storageType StorageType, checker HealthChecker) {
	hcm.checkers.Store(storageType, checker)
}

// Start 启动健康检查后台任务
func (hcm *HealthCheckManager) Start(ctx context.Context) {
	if !hcm.config.Enabled {
		return
	}

	syncx.WithLock(&hcm.mu, func() {
		if hcm.running {
			return
		}
		hcm.running = true
		checkCtx, cancel := context.WithCancel(ctx)
		hcm.cancel = cancel

		go hcm.run(checkCtx)
	})
}

// Stop 停止健康检查后台任务
func (hcm *HealthCheckManager) Stop() {
	syncx.WithLock(&hcm.mu, func() {
		if hcm.cancel != nil {
			hcm.cancel()
		}
		hcm.running = false
	})
}

// run 健康检查主循环
func (hcm *HealthCheckManager) run(ctx context.Context) {
	ticker := time.NewTicker(hcm.config.Interval)
	defer ticker.Stop()

	hcm.checkAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hcm.checkAll(ctx)
		}
	}
}

// checkAll 对所有分组中的所有数据源执行健康检查
func (hcm *HealthCheckManager) checkAll(ctx context.Context) {
	hcm.registry.ForEach(func(name string, group *Group) {
		syncx.WithRLock(&group.mu, func() {
			sources := make([]*DataSource, len(group.Sources))
			copy(sources, group.Sources)

			for _, ds := range sources {
				hcm.checkOne(ctx, name, ds)
			}
		})
	})
}

// checkOne 对单个数据源执行健康检查
func (hcm *HealthCheckManager) checkOne(ctx context.Context, groupName string, ds *DataSource) {
	checker, ok := hcm.checkers.Load(ds.StorageType)
	if !ok {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, hcm.config.Timeout)
	defer cancel()

	healthy := checker.Check(checkCtx, ds)

	wasHealthy := ds.IsHealthy()
	if healthy {
		ds.MarkHealthy()
	} else {
		ds.MarkUnhealthy()
	}

	if wasHealthy != healthy {
		ds.Healthy.Store(healthy)
		group, _ := hcm.registry.GetGroup(groupName)
		if group != nil {
			group.InvalidateCache()
		}
		if hcm.logger != nil {
			if healthy {
				hcm.logger.InfoContext(ctx, "[Phantom] 数据源 %s 在分组 %s 中已恢复", ds.Name, groupName)
			} else {
				hcm.logger.WarnContext(ctx, "[Phantom] 数据源 %s 在分组 %s 中变为不健康", ds.Name, groupName)
			}
		}
	}
}

// DBHealthChecker 数据库健康检查器，通过 PingContext 检查连接可用性
type DBHealthChecker struct{}

// Check 检查数据库连接是否可用
// Instance 为 nil 时返回当前健康标记，有实例时通过 PingContext 检查连接可用性
func (c *DBHealthChecker) Check(ctx context.Context, ds *DataSource) bool {
	if ds.Instance == nil {
		return ds.IsHealthy()
	}

	type pinger interface {
		PingContext(ctx context.Context) error
	}

	type dbGetter interface {
		DB() (interface{}, error)
	}

	if getter, ok := ds.Instance.(dbGetter); ok {
		sqlDB, err := getter.DB()
		if err != nil {
			return false
		}
		if p, ok := sqlDB.(pinger); ok {
			return p.PingContext(ctx) == nil
		}
	}

	if p, ok := ds.Instance.(pinger); ok {
		return p.PingContext(ctx) == nil
	}

	return ds.IsHealthy()
}

// RedisHealthChecker Redis 健康检查器，通过 Ping 命令检查连接可用性
type RedisHealthChecker struct{}

// Check 检查 Redis 连接是否可用
// Instance 为 nil 时返回当前健康标记，有实例时通过 Ping 命令检查连接可用性
func (c *RedisHealthChecker) Check(ctx context.Context, ds *DataSource) bool {
	if ds.Instance == nil {
		return ds.IsHealthy()
	}

	type pinger interface {
		Ping(ctx context.Context) interface{ Err() error }
	}

	if p, ok := ds.Instance.(pinger); ok {
		result := p.Ping(ctx)
		return result.Err() == nil
	}

	return ds.IsHealthy()
}

// GenericHealthChecker 通用健康检查器，仅检查实例是否存在
type GenericHealthChecker struct{}

// Check 检查通用数据源健康状态
// Instance 为 nil 时默认认为健康（可能仅注册未设置实例），有实例时返回当前健康标记
func (c *GenericHealthChecker) Check(ctx context.Context, ds *DataSource) bool {
	if ds.Instance == nil {
		return ds.IsHealthy()
	}

	return ds.IsHealthy()
}
