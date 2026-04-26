/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\adapter\custom.go
 * @Description: 幻影引擎自定义适配器 - 泛型适配器，支持任意类型的存储实例
 *   通过泛型参数 T 适配不同的存储类型，提供数据源解析、声明式切换和便捷执行功能
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package adapter

import (
	"context"
	"fmt"

	"github.com/kamalyes/go-phantom"
)

// CustomAdapter 自定义泛型适配器，支持任意类型的存储实例
type CustomAdapter[T any] struct {
	storageType phantom.StorageType // 存储类型
}

// NewCustomAdapter 创建自定义泛型适配器
func NewCustomAdapter[T any](storageType phantom.StorageType) *CustomAdapter[T] {
	return &CustomAdapter[T]{storageType: storageType}
}

// StorageType 返回适配器的存储类型
func (a *CustomAdapter[T]) StorageType() phantom.StorageType {
	return a.storageType
}

// Resolve 解析数据源并返回类型为 T 的实例
func (a *CustomAdapter[T]) Resolve(ctx context.Context, engine *phantom.Phantom, group string) (T, error) {
	var zero T
	source, err := engine.Resolve(ctx, group)
	if err != nil {
		return zero, err
	}

	instance, ok := source.Instance.(T)
	if !ok {
		return zero, fmt.Errorf("source %q is not expected type", source.Name)
	}

	return instance, nil
}

// Use 返回一个声明式切换数据源的执行函数，在执行前自动切换到指定数据源
func (a *CustomAdapter[T]) Use(dsName string, fn func(ctx context.Context, instance T) error) func(ctx context.Context, engine *phantom.Phantom, group string) error {
	return func(ctx context.Context, engine *phantom.Phantom, group string) error {
		ctx = phantom.Use(ctx, dsName)
		instance, err := a.Resolve(ctx, engine, group)
		if err != nil {
			return err
		}
		return fn(ctx, instance)
	}
}

// CustomCallbackFunc 自定义操作回调函数类型
type CustomCallbackFunc[T any] func(ctx context.Context, instance T) error

// Execute 在指定分组的数据源上执行自定义操作
func (a *CustomAdapter[T]) Execute(ctx context.Context, engine *phantom.Phantom, group string, fn CustomCallbackFunc[T]) error {
	instance, err := a.Resolve(ctx, engine, group)
	if err != nil {
		return err
	}
	return fn(ctx, instance)
}

// ExecuteWithDS 在指定数据源上执行自定义操作（声明式切换）
func (a *CustomAdapter[T]) ExecuteWithDS(ctx context.Context, engine *phantom.Phantom, group, dsName string, fn CustomCallbackFunc[T]) error {
	ctx = phantom.Use(ctx, dsName)
	return a.Execute(ctx, engine, group, fn)
}
