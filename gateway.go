/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\gateway.go
 * @Description: 幻影引擎网关集成 - 提供 HTTP 中间件和 gRPC 拦截器，
 *   自动从请求头/gRPC 元数据中提取路由上下文信息（数据源、影子标记、租户等），
 *   并注入到请求上下文中供下游路由使用
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"net/http"
	"strings"
)

// GatewayMiddleware HTTP 网关中间件，从请求头中提取路由上下文
type GatewayMiddleware struct {
	phantom      *Phantom // 幻影引擎引用
	headerPrefix string   // 请求头前缀，默认为 "X-Phantom-"
}

// NewGatewayMiddleware 创建 HTTP 网关中间件
func NewGatewayMiddleware(p *Phantom, headerPrefix string) *GatewayMiddleware {
	if headerPrefix == "" {
		headerPrefix = "X-Phantom-"
	}
	return &GatewayMiddleware{
		phantom:      p,
		headerPrefix: headerPrefix,
	}
}

// Middleware 返回标准 http.Handler 中间件，自动提取路由上下文
func (gm *GatewayMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := gm.buildRouteContext(r)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// buildRouteContext 从 HTTP 请求头中构建路由上下文
func (gm *GatewayMiddleware) buildRouteContext(r *http.Request) context.Context {
	ctx := r.Context()
	routeCtx := extractRouteContext(ctx)
	if routeCtx != nil {
		return ctx
	}

	builder := NewRouteContextBuilder()

	if dsName := r.Header.Get(gm.headerPrefix + "DS"); dsName != "" {
		builder.DSName(dsName)
	}

	if shadow := r.Header.Get(gm.headerPrefix + "Shadow"); shadow != "" {
		builder.Shadow(strings.EqualFold(shadow, "true") || shadow == "1")
	}

	if tenantID := r.Header.Get(gm.headerPrefix + "Tenant"); tenantID != "" {
		builder.TenantID(tenantID)
	}

	if readOnly := r.Header.Get(gm.headerPrefix + "ReadOnly"); readOnly != "" {
		builder.ReadOnly(strings.EqualFold(readOnly, "true") || readOnly == "1")
	}

	if hint := r.Header.Get(gm.headerPrefix + "Hint"); hint != "" {
		builder.RouteHint(hint)
	}

	if builder != (&RouteContextBuilder{}) {
		routeCtx = builder.Build()
		ctx = WithRouteContext(ctx, routeCtx)
	}

	return ctx
}

// GRPCInterceptor gRPC 拦截器，从元数据中提取路由上下文
type GRPCInterceptor struct {
	phantom    *Phantom // 幻影引擎引用
	metaPrefix string   // 元数据键前缀，默认为 "phantom-"
}

// NewGRPCInterceptor 创建 gRPC 拦截器
func NewGRPCInterceptor(p *Phantom, metaPrefix string) *GRPCInterceptor {
	if metaPrefix == "" {
		metaPrefix = "phantom-"
	}
	return &GRPCInterceptor{
		phantom:    p,
		metaPrefix: metaPrefix,
	}
}

// ExtractRouteContext 从 gRPC 元数据中提取路由上下文并注入到 context 中
func (gi *GRPCInterceptor) ExtractRouteContext(ctx context.Context) context.Context {
	existing := extractRouteContext(ctx)
	if existing != nil {
		return ctx
	}

	builder := NewRouteContextBuilder()

	if md, ok := extractMetadata(ctx); ok {
		if dsName := getFirstMeta(md, gi.metaPrefix+"ds"); dsName != "" {
			builder.DSName(dsName)
		}
		if shadow := getFirstMeta(md, gi.metaPrefix+"shadow"); shadow != "" {
			builder.Shadow(strings.EqualFold(shadow, "true") || shadow == "1")
		}
		if tenantID := getFirstMeta(md, gi.metaPrefix+"tenant"); tenantID != "" {
			builder.TenantID(tenantID)
		}
		if readOnly := getFirstMeta(md, gi.metaPrefix+"readonly"); readOnly != "" {
			builder.ReadOnly(strings.EqualFold(readOnly, "true") || readOnly == "1")
		}
		if hint := getFirstMeta(md, gi.metaPrefix+"hint"); hint != "" {
			builder.RouteHint(hint)
		}
	}

	if builder != (&RouteContextBuilder{}) {
		routeCtx := builder.Build()
		ctx = WithRouteContext(ctx, routeCtx)
	}

	return ctx
}

// metadataMap 元数据映射类型
type metadataMap map[string][]string

var extractMetadataFunc = func(ctx context.Context) (metadataMap, bool) {
	return nil, false
}

// extractMetadata 从上下文中提取 gRPC 元数据（需配合 grpc-go 使用）
func extractMetadata(ctx context.Context) (metadataMap, bool) {
	return extractMetadataFunc(ctx)
}

// getFirstMeta 获取元数据中指定键的第一个值
func getFirstMeta(md metadataMap, key string) string {
	if vals, ok := md[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}
