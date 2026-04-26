/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\gateway_test.go
 * @Description: 测试网关中间件
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGatewayMiddleware_DefaultPrefix(t *testing.T) {
	gm := NewGatewayMiddleware(nil, "")
	assert.Equal(t, "X-Phantom-", gm.headerPrefix)
}

func TestGatewayMiddleware_CustomPrefix(t *testing.T) {
	gm := NewGatewayMiddleware(nil, "X-Custom-")
	assert.Equal(t, "X-Custom-", gm.headerPrefix)
}

func TestGatewayMiddleware_ExtractHeaders(t *testing.T) {
	p := NewPhantom()
	gm := NewGatewayMiddleware(p, "X-Phantom-")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Phantom-DS", "slave_db")
	req.Header.Set("X-Phantom-Shadow", "true")
	req.Header.Set("X-Phantom-Tenant", "tenant_1")
	req.Header.Set("X-Phantom-ReadOnly", "1")
	req.Header.Set("X-Phantom-Hint", "hint_ds")

	ctx := gm.buildRouteContext(req)
	rc := GetRouteContext(ctx)
	assert.NotNil(t, rc)
	assert.Equal(t, "slave_db", rc.DSName)
	assert.True(t, rc.Shadow)
	assert.Equal(t, "tenant_1", rc.TenantID)
	assert.True(t, rc.ReadOnly)
	assert.Equal(t, "hint_ds", rc.RouteHint)
}

func TestGatewayMiddleware_NoHeaders(t *testing.T) {
	p := NewPhantom()
	gm := NewGatewayMiddleware(p, "X-Phantom-")

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := gm.buildRouteContext(req)
	rc := GetRouteContext(ctx)
	assert.NotNil(t, rc)
	assert.Equal(t, "", rc.DSName)
	assert.False(t, rc.Shadow)
	assert.Equal(t, "", rc.TenantID)
	assert.False(t, rc.ReadOnly)
	assert.Equal(t, "", rc.RouteHint)
}

func TestGatewayMiddleware_ExistingRouteContext(t *testing.T) {
	p := NewPhantom()
	gm := NewGatewayMiddleware(p, "X-Phantom-")

	existingRC := NewRouteContext().WithDSName("existing_db")
	ctx := WithRouteContext(context.Background(), existingRC)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Phantom-DS", "slave_db")
	req = req.WithContext(ctx)

	resultCtx := gm.buildRouteContext(req)
	rc := GetRouteContext(resultCtx)
	assert.NotNil(t, rc)
	assert.Equal(t, "existing_db", rc.DSName)
}

func TestGatewayMiddleware_Middleware(t *testing.T) {
	p := NewPhantom()
	gm := NewGatewayMiddleware(p, "X-Phantom-")

	called := false
	handler := gm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		rc := GetRouteContext(r.Context())
		assert.NotNil(t, rc)
		assert.Equal(t, "slave_db", rc.DSName)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Phantom-DS", "slave_db")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.True(t, called)
}

func TestGRPCInterceptor_DefaultPrefix(t *testing.T) {
	gi := NewGRPCInterceptor(nil, "")
	assert.Equal(t, "phantom-", gi.metaPrefix)
}

func TestGRPCInterceptor_CustomPrefix(t *testing.T) {
	gi := NewGRPCInterceptor(nil, "X-Custom-")
	assert.Equal(t, "X-Custom-", gi.metaPrefix)
}

func TestGRPCInterceptor_ExtractRouteContext(t *testing.T) {
	p := NewPhantom()
	gi := NewGRPCInterceptor(p, "phantom-")

	ctx := context.Background()
	resultCtx := gi.ExtractRouteContext(ctx)
	assert.NotNil(t, resultCtx)
}

func TestGRPCInterceptor_ExtractRouteContext_ExistingContext(t *testing.T) {
	p := NewPhantom()
	gi := NewGRPCInterceptor(p, "phantom-")

	existingRC := NewRouteContext().WithDSName("existing_db")
	ctx := WithRouteContext(context.Background(), existingRC)

	resultCtx := gi.ExtractRouteContext(ctx)
	rc := GetRouteContext(resultCtx)
	assert.Equal(t, "existing_db", rc.DSName)
}

func TestGetFirstMeta(t *testing.T) {
	md := metadataMap{
		"key1": {"value1", "value2"},
		"key2": {},
	}

	assert.Equal(t, "value1", getFirstMeta(md, "key1"))
	assert.Equal(t, "", getFirstMeta(md, "key2"))
	assert.Equal(t, "", getFirstMeta(md, "nonexistent"))
}

func TestGatewayMiddleware_ShadowHeader(t *testing.T) {
	p := NewPhantom()
	gm := NewGatewayMiddleware(p, "X-Phantom-")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Phantom-Shadow", "1")
	ctx := gm.buildRouteContext(req)
	rc := GetRouteContext(ctx)
	assert.True(t, rc.Shadow)
}

func TestGatewayMiddleware_ReadOnlyHeader(t *testing.T) {
	p := NewPhantom()
	gm := NewGatewayMiddleware(p, "X-Phantom-")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Phantom-ReadOnly", "1")
	ctx := gm.buildRouteContext(req)
	rc := GetRouteContext(ctx)
	assert.True(t, rc.ReadOnly)
}

func TestGRPCInterceptor_ExtractRouteContext_WithMetadata(t *testing.T) {
	original := extractMetadataFunc
	defer func() { extractMetadataFunc = original }()

	extractMetadataFunc = func(ctx context.Context) (metadataMap, bool) {
		return metadataMap{
			"phantom-ds":       {"slave_db"},
			"phantom-shadow":   {"true"},
			"phantom-tenant":   {"tenant_1"},
			"phantom-readonly": {"1"},
			"phantom-hint":     {"hint_ds"},
		}, true
	}

	p := NewPhantom()
	gi := NewGRPCInterceptor(p, "phantom-")

	ctx := context.Background()
	resultCtx := gi.ExtractRouteContext(ctx)
	rc := GetRouteContext(resultCtx)
	assert.NotNil(t, rc)
	assert.Equal(t, "slave_db", rc.DSName)
	assert.True(t, rc.Shadow)
	assert.Equal(t, "tenant_1", rc.TenantID)
	assert.True(t, rc.ReadOnly)
	assert.Equal(t, "hint_ds", rc.RouteHint)
}

func TestGRPCInterceptor_ExtractRouteContext_MetadataShadowVariant(t *testing.T) {
	original := extractMetadataFunc
	defer func() { extractMetadataFunc = original }()

	extractMetadataFunc = func(ctx context.Context) (metadataMap, bool) {
		return metadataMap{
			"phantom-shadow":   {"1"},
			"phantom-readonly": {"true"},
		}, true
	}

	p := NewPhantom()
	gi := NewGRPCInterceptor(p, "phantom-")

	ctx := context.Background()
	resultCtx := gi.ExtractRouteContext(ctx)
	rc := GetRouteContext(resultCtx)
	assert.True(t, rc.Shadow)
	assert.True(t, rc.ReadOnly)
}

func TestGRPCInterceptor_ExtractRouteContext_EmptyMetadata(t *testing.T) {
	original := extractMetadataFunc
	defer func() { extractMetadataFunc = original }()

	extractMetadataFunc = func(ctx context.Context) (metadataMap, bool) {
		return metadataMap{}, true
	}

	p := NewPhantom()
	gi := NewGRPCInterceptor(p, "phantom-")

	ctx := context.Background()
	resultCtx := gi.ExtractRouteContext(ctx)
	rc := GetRouteContext(resultCtx)
	assert.NotNil(t, rc)
	assert.Empty(t, rc.DSName)
}
