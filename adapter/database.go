/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\adapter\database.go
 * @Description: 幻影引擎数据库适配器 - 封装 GORM 数据库操作，
 *   提供数据源解析、声明式切换、事务管理和双写支持
 *   双写模式支持同步/异步影子库写入，适用于数据迁移和灰度发布场景
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package adapter

import (
	"context"
	"fmt"

	"github.com/kamalyes/go-phantom"
	"gorm.io/gorm"
)

// DatabaseAdapter 数据库适配器，封装 GORM 与幻影引擎的集成
type DatabaseAdapter struct{}

// NewDatabaseAdapter 创建数据库适配器
func NewDatabaseAdapter() *DatabaseAdapter {
	return &DatabaseAdapter{}
}

// StorageType 返回适配器的存储类型
func (a *DatabaseAdapter) StorageType() phantom.StorageType {
	return phantom.StorageDatabase
}

// Resolve 解析数据源并返回 *gorm.DB 实例
func (a *DatabaseAdapter) Resolve(ctx context.Context, engine *phantom.Phantom, group string) (*gorm.DB, error) {
	source, err := engine.Resolve(ctx, group)
	if err != nil {
		return nil, err
	}

	db, ok := source.Instance.(*gorm.DB)
	if !ok {
		return nil, fmt.Errorf("source %q is not a *gorm.DB instance", source.Name)
	}

	return db, nil
}

// Use 返回一个声明式切换数据源的执行函数，在执行前自动切换到指定数据源
func (a *DatabaseAdapter) Use(dsName string, fn func(ctx context.Context, db *gorm.DB) error) func(ctx context.Context, engine *phantom.Phantom, group string) error {
	return func(ctx context.Context, engine *phantom.Phantom, group string) error {
		ctx = phantom.Use(ctx, dsName)
		db, err := a.Resolve(ctx, engine, group)
		if err != nil {
			return err
		}
		return fn(ctx, db)
	}
}

// DBCallbackFunc 数据库操作回调函数类型
type DBCallbackFunc func(ctx context.Context, db *gorm.DB) error

// Execute 在指定分组的数据源上执行数据库操作
func (a *DatabaseAdapter) Execute(ctx context.Context, engine *phantom.Phantom, group string, fn DBCallbackFunc) error {
	db, err := a.Resolve(ctx, engine, group)
	if err != nil {
		return err
	}
	return fn(ctx, db)
}

// ExecuteWithDS 在指定数据源上执行数据库操作（声明式切换）
func (a *DatabaseAdapter) ExecuteWithDS(ctx context.Context, engine *phantom.Phantom, group, dsName string, fn DBCallbackFunc) error {
	ctx = phantom.Use(ctx, dsName)
	return a.Execute(ctx, engine, group, fn)
}

// Transaction 在指定分组的数据源上执行事务操作
func (a *DatabaseAdapter) Transaction(ctx context.Context, engine *phantom.Phantom, group string, fn DBCallbackFunc) error {
	db, err := a.Resolve(ctx, engine, group)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		return fn(ctx, tx)
	})
}

// TransactionWithDS 在指定数据源上执行事务操作（声明式切换）
func (a *DatabaseAdapter) TransactionWithDS(ctx context.Context, engine *phantom.Phantom, group, dsName string, fn DBCallbackFunc) error {
	ctx = phantom.Use(ctx, dsName)
	return a.Transaction(ctx, engine, group, fn)
}

// DualWriteOption 双写选项配置
type DualWriteOption struct {
	PrimaryGroup string // 主库分组名称
	ShadowGroup  string // 影子库分组名称
	ShadowSilent bool   // 影子库写入失败时是否静默处理
	ShadowAsync  bool   // 影子库是否异步写入
}

// DualWrite 执行双写操作 - 同时写入主库和影子库
// 影子库写入失败不影响主库结果，支持同步和异步两种模式
func (a *DatabaseAdapter) DualWrite(ctx context.Context, engine *phantom.Phantom, opt DualWriteOption, fn DBCallbackFunc) error {
	if err := a.Execute(ctx, engine, opt.PrimaryGroup, fn); err != nil {
		return fmt.Errorf("primary write failed: %w", err)
	}

	shadowFn := func() error {
		shadowCtx := phantom.WithShadow(ctx, true)
		return a.Execute(shadowCtx, engine, opt.ShadowGroup, fn)
	}

	if opt.ShadowAsync {
		go func() {
			if err := shadowFn(); err != nil && !opt.ShadowSilent {
				_ = err
			}
		}()
		return nil
	}

	if err := shadowFn(); err != nil && !opt.ShadowSilent {
		return fmt.Errorf("shadow write failed: %w", err)
	}

	return nil
}

// DualWriteTransaction 执行双写事务操作 - 主库事务成功后执行影子库事务
// 影子库事务失败不影响主库结果，支持同步和异步两种模式
func (a *DatabaseAdapter) DualWriteTransaction(ctx context.Context, engine *phantom.Phantom, opt DualWriteOption, fn DBCallbackFunc) error {
	if err := a.Transaction(ctx, engine, opt.PrimaryGroup, fn); err != nil {
		return fmt.Errorf("primary transaction failed: %w", err)
	}

	shadowFn := func() error {
		shadowCtx := phantom.WithShadow(ctx, true)
		return a.Transaction(shadowCtx, engine, opt.ShadowGroup, fn)
	}

	if opt.ShadowAsync {
		go func() {
			_ = shadowFn()
		}()
		return nil
	}

	if err := shadowFn(); err != nil && !opt.ShadowSilent {
		return fmt.Errorf("shadow transaction failed: %w", err)
	}

	return nil
}
