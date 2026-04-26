/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\strategy\strategy.go
 * @Description: 幻影引擎扩展策略 - 提供组合策略和函数策略两种扩展方式
 *   组合策略按顺序尝试多个策略，返回第一个成功的结果；
 *   函数策略允许通过闭包快速定义自定义路由逻辑
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package strategy

import (
	"context"

	"github.com/kamalyes/go-phantom"
)

// CompositeStrategy 组合策略 - 按顺序尝试多个策略，返回第一个成功的结果
// 适用于需要多级回退的场景，如：先尝试租户策略，失败则回退到主库策略
type CompositeStrategy struct {
	strategies []phantom.Strategy // 策略列表，按顺序尝试
}

// NewCompositeStrategy 创建组合策略，传入多个策略按顺序尝试
func NewCompositeStrategy(strategies ...phantom.Strategy) *CompositeStrategy {
	return &CompositeStrategy{strategies: strategies}
}

// Resolve 按顺序尝试所有策略，返回第一个成功解析的结果
func (s *CompositeStrategy) Resolve(ctx context.Context, group *phantom.Group, routeCtx *phantom.RouteContext) (*phantom.RouteResult, error) {
	for _, strategy := range s.strategies {
		result, err := strategy.Resolve(ctx, group, routeCtx)
		if err == nil && result != nil {
			return result, nil
		}
	}
	return nil, phantom.ErrNoAvailableSource
}

// Name 返回策略名称
func (s *CompositeStrategy) Name() string { return "composite" }

// FuncStrategy 函数策略 - 通过闭包快速定义自定义路由逻辑
// 适用于简单的自定义路由场景，无需创建完整的策略结构体
type FuncStrategy struct {
	name string                                                                                                        // 策略名称
	fn   func(ctx context.Context, group *phantom.Group, routeCtx *phantom.RouteContext) (*phantom.RouteResult, error) // 路由解析函数
}

// NewFuncStrategy 创建函数策略，传入策略名称和路由解析函数
func NewFuncStrategy(name string, fn func(ctx context.Context, group *phantom.Group, routeCtx *phantom.RouteContext) (*phantom.RouteResult, error)) *FuncStrategy {
	return &FuncStrategy{name: name, fn: fn}
}

// Resolve 调用自定义路由解析函数
func (s *FuncStrategy) Resolve(ctx context.Context, group *phantom.Group, routeCtx *phantom.RouteContext) (*phantom.RouteResult, error) {
	return s.fn(ctx, group, routeCtx)
}

// Name 返回策略名称
func (s *FuncStrategy) Name() string { return s.name }
