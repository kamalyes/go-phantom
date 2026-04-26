/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\context_test.go
 * @Description: 测试上下文
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRouteContext_Clone(t *testing.T) {
	rc := NewRouteContext().
		WithDSName("slave_db").
		WithTenantID("tenant_1").
		WithShadow(true).
		WithReadOnly(false).
		WithRouteHint("hint_1").
		WithExtra("key1", "value1")

	cloned := rc.Clone()

	assert.Equal(t, rc.DSName, cloned.DSName)
	assert.Equal(t, rc.TenantID, cloned.TenantID)
	assert.Equal(t, rc.Shadow, cloned.Shadow)
	assert.Equal(t, rc.ReadOnly, cloned.ReadOnly)
	assert.Equal(t, rc.RouteHint, cloned.RouteHint)
	assert.Equal(t, rc.Extra["key1"], cloned.Extra["key1"])

	cloned.TenantID = "tenant_2"
	assert.NotEqual(t, rc.TenantID, cloned.TenantID)
}

func TestRouteContext_WithMethods(t *testing.T) {
	rc := NewRouteContext()

	rc2 := rc.WithDSName("slave_db")
	assert.Equal(t, "slave_db", rc2.DSName)
	assert.Equal(t, "", rc.DSName)

	rc3 := rc2.WithShadow(true)
	assert.True(t, rc3.Shadow)
	assert.False(t, rc2.Shadow)

	rc4 := rc3.WithTenantID("t1")
	assert.Equal(t, "t1", rc4.TenantID)
	assert.Equal(t, "", rc3.TenantID)

	rc5 := rc4.WithReadOnly(true)
	assert.True(t, rc5.ReadOnly)
	assert.False(t, rc4.ReadOnly)

	rc6 := rc5.WithRouteHint("h1")
	assert.Equal(t, "h1", rc6.RouteHint)

	rc7 := rc6.WithExtra("k", "v")
	assert.Equal(t, "v", rc7.Extra["k"])
}

func TestRouteContextBuilder(t *testing.T) {
	rc := NewRouteContextBuilder().
		DSName("slave_db").
		Shadow(true).
		TenantID("tenant_1").
		ReadOnly(true).
		RouteHint("hint_1").
		Extra("key1", "value1").
		Extra("key2", 42).
		Build()

	assert.Equal(t, "slave_db", rc.DSName)
	assert.True(t, rc.Shadow)
	assert.Equal(t, "tenant_1", rc.TenantID)
	assert.True(t, rc.ReadOnly)
	assert.Equal(t, "hint_1", rc.RouteHint)
	assert.Equal(t, "value1", rc.Extra["key1"])
	assert.Equal(t, 42, rc.Extra["key2"])
}

func TestRouteContextBuilder_Empty(t *testing.T) {
	rc := NewRouteContextBuilder().Build()
	assert.NotNil(t, rc)
	assert.Equal(t, "", rc.DSName)
	assert.False(t, rc.Shadow)
	assert.Equal(t, "", rc.TenantID)
	assert.False(t, rc.ReadOnly)
	assert.Equal(t, "", rc.RouteHint)
	assert.Nil(t, rc.Extra)
}

func TestRouteContextBuilder_Chaining(t *testing.T) {
	builder := NewRouteContextBuilder()
	result := builder.DSName("db").Shadow(true).TenantID("t1").ReadOnly(true).RouteHint("h1")
	assert.Equal(t, builder, result)

	rc := result.Build()
	assert.Equal(t, "db", rc.DSName)
	assert.True(t, rc.Shadow)
	assert.Equal(t, "t1", rc.TenantID)
	assert.True(t, rc.ReadOnly)
	assert.Equal(t, "h1", rc.RouteHint)
}

func TestRouteContext_Clone_Nil(t *testing.T) {
	var rc *RouteContext
	cloned := rc.Clone()
	assert.Nil(t, cloned)
}

func TestRouteContext_Clone_DeepCopy(t *testing.T) {
	rc := NewRouteContext().WithExtra("key", "value")
	cloned := rc.Clone()

	cloned.Extra["key"] = "modified"
	assert.Equal(t, "value", rc.Extra["key"])
	assert.Equal(t, "modified", cloned.Extra["key"])
}

func TestNewRouteContext(t *testing.T) {
	rc := NewRouteContext()
	assert.NotNil(t, rc)
	assert.Equal(t, "", rc.DSName)
	assert.False(t, rc.Shadow)
	assert.Equal(t, "", rc.TenantID)
	assert.False(t, rc.ReadOnly)
	assert.Equal(t, "", rc.RouteHint)
	assert.NotNil(t, rc.Extra)
	assert.Empty(t, rc.Extra)
}

func TestWithRouteContext_NilContext(t *testing.T) {
	rc := GetRouteContext(context.TODO())
	assert.Nil(t, rc)
}

func TestCurrentDS_EmptyContext(t *testing.T) {
	assert.Equal(t, "", CurrentDS(context.Background()))
}

func TestPhantom_CurrentDS(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", CurrentDS(ctx))

	ctx = Use(ctx, "master")
	assert.Equal(t, "master", CurrentDS(ctx))

	ctx = Use(ctx, "slave")
	assert.Equal(t, "slave", CurrentDS(ctx))
}

func TestPhantom_ContextPropagation(t *testing.T) {
	rc := NewRouteContext().WithTenantID("t1").WithShadow(true)
	ctx := WithRouteContext(context.Background(), rc)

	extracted := GetRouteContext(ctx)
	assert.NotNil(t, extracted)
	assert.Equal(t, "t1", extracted.TenantID)
	assert.True(t, extracted.Shadow)
}

func TestPhantom_WithGroup(t *testing.T) {
	ctx := WithGroup(context.Background(), "my_group")
	rc := GetRouteContext(ctx)
	assert.Equal(t, "my_group", rc.DSName)
}

func TestPhantom_WithShadow(t *testing.T) {
	ctx := WithShadow(context.Background(), true)
	rc := GetRouteContext(ctx)
	assert.True(t, rc.Shadow)
}

func TestPhantom_WithTenant(t *testing.T) {
	ctx := WithTenant(context.Background(), "tenant_1")
	rc := GetRouteContext(ctx)
	assert.Equal(t, "tenant_1", rc.TenantID)
}

func TestPhantom_WithReadOnly(t *testing.T) {
	ctx := WithReadOnly(context.Background(), true)
	rc := GetRouteContext(ctx)
	assert.True(t, rc.ReadOnly)
}

func TestPhantom_WithRouteHint(t *testing.T) {
	ctx := WithRouteHint(context.Background(), "hint_ds")
	rc := GetRouteContext(ctx)
	assert.Equal(t, "hint_ds", rc.RouteHint)
}

func TestPhantom_ContextChaining(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenant(ctx, "t1")
	ctx = WithReadOnly(ctx, true)
	ctx = WithShadow(ctx, false)
	ctx = Use(ctx, "slave_db")

	rc := GetRouteContext(ctx)
	assert.Equal(t, "t1", rc.TenantID)
	assert.True(t, rc.ReadOnly)
	assert.False(t, rc.Shadow)
	assert.Equal(t, "slave_db", rc.DSName)
}

func TestPhantom_NilContext(t *testing.T) {
	rc := GetRouteContext(nil)
	assert.Nil(t, rc)
}

func TestWithDS_Func(t *testing.T) {
	var capturedName string
	fn := WithDS("slave_db", func(ctx context.Context) error {
		capturedName = CurrentDS(ctx)
		return nil
	})

	err := fn(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "slave_db", capturedName)
}

func TestStorageType_Constants(t *testing.T) {
	assert.Equal(t, StorageType("database"), StorageDatabase)
	assert.Equal(t, StorageType("redis"), StorageRedis)
	assert.Equal(t, StorageType("custom"), StorageCustom)
}

func TestRouteContext_WithExtra_NilMap(t *testing.T) {
	rc := &RouteContext{}
	rc2 := rc.WithExtra("key", "value")
	assert.NotNil(t, rc2.Extra)
	assert.Equal(t, "value", rc2.Extra["key"])
}

func TestWithGroup_ExistingContext(t *testing.T) {
	ctx := WithTenant(context.Background(), "t1")
	ctx = WithGroup(ctx, "my_group")
	rc := GetRouteContext(ctx)
	assert.Equal(t, "my_group", rc.DSName)
	assert.Equal(t, "t1", rc.TenantID)
}

func TestWithShadow_ExistingContext(t *testing.T) {
	ctx := WithTenant(context.Background(), "t1")
	ctx = WithShadow(ctx, true)
	rc := GetRouteContext(ctx)
	assert.True(t, rc.Shadow)
	assert.Equal(t, "t1", rc.TenantID)
}

func TestWithTenant_ExistingContext(t *testing.T) {
	ctx := WithShadow(context.Background(), true)
	ctx = WithTenant(ctx, "t1")
	rc := GetRouteContext(ctx)
	assert.Equal(t, "t1", rc.TenantID)
	assert.True(t, rc.Shadow)
}

func TestWithReadOnly_ExistingContext(t *testing.T) {
	ctx := WithTenant(context.Background(), "t1")
	ctx = WithReadOnly(ctx, true)
	rc := GetRouteContext(ctx)
	assert.True(t, rc.ReadOnly)
	assert.Equal(t, "t1", rc.TenantID)
}

func TestWithRouteHint_ExistingContext(t *testing.T) {
	ctx := WithTenant(context.Background(), "t1")
	ctx = WithRouteHint(ctx, "hint_ds")
	rc := GetRouteContext(ctx)
	assert.Equal(t, "hint_ds", rc.RouteHint)
	assert.Equal(t, "t1", rc.TenantID)
}

func TestGetRouteContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextKey, "not_a_route_context")
	rc := GetRouteContext(ctx)
	assert.Nil(t, rc)
}

func TestWithRouteContext_NilCtx(t *testing.T) {
	rc := NewRouteContext().WithDSName("test")
	ctx := WithRouteContext(nil, rc)
	assert.NotNil(t, ctx)
	result := GetRouteContext(ctx)
	assert.Equal(t, "test", result.DSName)
}
