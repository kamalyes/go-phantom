/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\router_group.go
 * @Description: 幻影引擎路由器 - 管理分组与策略的映射关系，
 *   根据分组名称选择对应策略执行数据源路由
 *   使用 syncx.Map 实现策略映射的并发安全
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Router 路由器，管理分组策略映射并执行数据源路由
type Router struct {
	registry   *Registry                    // 注册中心引用
	strategies *syncx.Map[string, Strategy] // 分组到策略的映射，使用 syncx.Map 保证并发安全
	logger     logger.ILogger
}

// NewRouter 创建一个新的路由器
func NewRouter(registry *Registry, log logger.ILogger) *Router {
	return &Router{
		registry:   registry,
		strategies: syncx.NewMap[string, Strategy](),
		logger:     log,
	}
}

// SetStrategy 为指定分组设置路由策略
func (r *Router) SetStrategy(groupName string, strategy Strategy) {
	r.strategies.Store(groupName, strategy)
}

// GetStrategy 获取指定分组的路由策略
func (r *Router) GetStrategy(groupName string) Strategy {
	if s, ok := r.strategies.Load(groupName); ok {
		return s
	}
	return nil
}

// Route 执行数据源路由 - 根据分组名称查找策略并解析数据源
func (r *Router) Route(ctx context.Context, groupName string, routeCtx *RouteContext) (*DataSource, error) {
	group, ok := r.registry.GetGroup(groupName)
	if !ok {
		return nil, NewGroupError(groupName, ErrGroupNotFound)
	}

	strategy := r.GetStrategy(groupName)
	if strategy == nil {
		strategy = &PrimaryStrategy{}
	}

	result, err := strategy.Resolve(ctx, group, routeCtx)
	if err != nil {
		return nil, err
	}

	if result.Source == nil {
		return nil, NewGroupError(groupName, ErrNilRouteResult)
	}

	return result.Source, nil
}
