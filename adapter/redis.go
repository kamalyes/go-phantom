/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\adapter\redis.go
 * @Description: 幻影引擎 Redis 适配器 - 封装 go-redis 客户端操作，
 *   提供数据源解析、声明式切换和便捷执行功能
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package adapter

import (
	"context"
	"fmt"

	"github.com/kamalyes/go-phantom"
	"github.com/redis/go-redis/v9"
)

// RedisAdapter Redis 适配器，封装 go-redis 与幻影引擎的集成
type RedisAdapter struct{}

// NewRedisAdapter 创建 Redis 适配器
func NewRedisAdapter() *RedisAdapter {
	return &RedisAdapter{}
}

// StorageType 返回适配器的存储类型
func (a *RedisAdapter) StorageType() phantom.StorageType {
	return phantom.StorageRedis
}

// Resolve 解析数据源并返回 *redis.Client 实例
func (a *RedisAdapter) Resolve(ctx context.Context, engine *phantom.Phantom, group string) (*redis.Client, error) {
	source, err := engine.Resolve(ctx, group)
	if err != nil {
		return nil, err
	}

	client, ok := source.Instance.(*redis.Client)
	if !ok {
		return nil, fmt.Errorf("source %q is not a *redis.Client instance", source.Name)
	}

	return client, nil
}

// Use 返回一个声明式切换数据源的执行函数，在执行前自动切换到指定数据源
func (a *RedisAdapter) Use(dsName string, fn func(ctx context.Context, client *redis.Client) error) func(ctx context.Context, engine *phantom.Phantom, group string) error {
	return func(ctx context.Context, engine *phantom.Phantom, group string) error {
		ctx = phantom.Use(ctx, dsName)
		client, err := a.Resolve(ctx, engine, group)
		if err != nil {
			return err
		}
		return fn(ctx, client)
	}
}

// RedisCallbackFunc Redis 操作回调函数类型
type RedisCallbackFunc func(ctx context.Context, client *redis.Client) error

// Execute 在指定分组的数据源上执行 Redis 操作
func (a *RedisAdapter) Execute(ctx context.Context, engine *phantom.Phantom, group string, fn RedisCallbackFunc) error {
	client, err := a.Resolve(ctx, engine, group)
	if err != nil {
		return err
	}
	return fn(ctx, client)
}

// ExecuteWithDS 在指定数据源上执行 Redis 操作（声明式切换）
func (a *RedisAdapter) ExecuteWithDS(ctx context.Context, engine *phantom.Phantom, group, dsName string, fn RedisCallbackFunc) error {
	ctx = phantom.Use(ctx, dsName)
	return a.Execute(ctx, engine, group, fn)
}
