/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\errors.go
 * @Description: 幻影引擎哨兵错误定义 - 使用预定义错误值避免 fmt.Errorf 的运行时开销
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import "errors"

// 哨兵错误定义 - 所有核心错误均以预定义变量形式提供，
// 调用方可直接使用 errors.Is 进行判断，无需字符串匹配
var (
	ErrNoAvailableSource = errors.New("没有可用的数据源")
	ErrGroupNotFound     = errors.New("分组未找到")
	ErrSourceNotFound    = errors.New("数据源未找到")
	ErrSourceUnhealthy   = errors.New("数据源不健康")
	ErrNoPrimary         = errors.New("分组中没有主数据源")
	ErrNoReadSource      = errors.New("分组中没有可用的读数据源")
	ErrAlreadyInitialized = errors.New("幻影引擎已初始化")
	ErrNotInitialized    = errors.New("幻影引擎未初始化")
	ErrStrategyNotFound  = errors.New("策略未找到")
	ErrNoDefaultGroup    = errors.New("该存储类型没有默认分组")
	ErrGroupExists       = errors.New("分组已注册")
	ErrNilRouteResult    = errors.New("策略返回了空数据源")
)

// GroupError 分组级别的错误，携带分组名称和内部错误
type GroupError struct {
	Group string
	Err   error
}

// Error 返回格式为 "分组名: 内部错误信息" 的错误字符串
func (e *GroupError) Error() string {
	return e.Group + ": " + e.Err.Error()
}

// Unwrap 支持errors.Is/As解包到内部错误
func (e *GroupError) Unwrap() error {
	return e.Err
}

// NewGroupError 创建一个分组级别的错误
func NewGroupError(group string, err error) *GroupError {
	return &GroupError{Group: group, Err: err}
}

// SourceError 数据源级别的错误，携带分组名、数据源名和内部错误
type SourceError struct {
	Group  string
	Source string
	Err    error
}

// Error 返回格式为 "分组名/数据源名: 内部错误信息" 的错误字符串
func (e *SourceError) Error() string {
	return e.Group + "/" + e.Source + ": " + e.Err.Error()
}

// Unwrap 支持errors.Is/As解包到内部错误
func (e *SourceError) Unwrap() error {
	return e.Err
}

// NewSourceError 创建一个数据源级别的错误
func NewSourceError(group, source string, err error) *SourceError {
	return &SourceError{Group: group, Source: source, Err: err}
}
