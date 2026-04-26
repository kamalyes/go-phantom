/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\errors_test.go
 * @Description: 测试错误定义
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroupError_Error(t *testing.T) {
	err := NewGroupError("db_group", ErrNoPrimary)
	assert.Equal(t, "db_group: 分组中没有主数据源", err.Error())
}

func TestGroupError_Unwrap(t *testing.T) {
	err := NewGroupError("db_group", ErrNoPrimary)
	assert.True(t, errors.Is(err, ErrNoPrimary))
}

func TestSourceError_Error(t *testing.T) {
	err := NewSourceError("db_group", "primary_db", ErrSourceUnhealthy)
	assert.Equal(t, "db_group/primary_db: 数据源不健康", err.Error())
}

func TestSourceError_Unwrap(t *testing.T) {
	err := NewSourceError("db_group", "primary_db", ErrSourceUnhealthy)
	assert.True(t, errors.Is(err, ErrSourceUnhealthy))
}

func TestSentinelErrors(t *testing.T) {
	sentinelErrs := []error{
		ErrNoAvailableSource,
		ErrGroupNotFound,
		ErrSourceNotFound,
		ErrSourceUnhealthy,
		ErrNoPrimary,
		ErrNoReadSource,
		ErrAlreadyInitialized,
		ErrNotInitialized,
		ErrStrategyNotFound,
		ErrNoDefaultGroup,
		ErrGroupExists,
		ErrNilRouteResult,
	}

	for _, e := range sentinelErrs {
		assert.NotNil(t, e)
		assert.NotEmpty(t, e.Error())
	}
}
