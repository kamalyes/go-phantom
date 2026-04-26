/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\context.go
 * @Description: 幻影引擎路由上下文 - 管理数据源路由所需的上下文信息，
 *   包括数据源名称、影子标记、租户ID、只读标记、路由提示等
 *   提供 RouteContextBuilder 构建器模式以减少堆分配
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import "context"

// contextKeyType 用于 context.WithValue 的私有键类型，避免外部冲突
type contextKeyType struct{}

// contextKey 路由上下文在 context.Context 中的键
var contextKey = contextKeyType{}

// RouteContext 路由上下文，携带数据源路由所需的全部信息
type RouteContext struct {
	DSName    string                 // 指定数据源名称，用于声明式切换
	Shadow    bool                   // 是否为影子流量
	TenantID  string                 // 租户标识，用于多租户数据隔离
	ReadOnly  bool                   // 是否只读请求，影响读写分离策略
	RouteHint string                 // 路由提示，用于 HintStrategy 定向路由
	Extra     map[string]interface{} // 扩展字段，存储自定义路由参数
}

// Clone 深拷贝当前路由上下文，返回一个完全独立的新实例
func (rc *RouteContext) Clone() *RouteContext {
	if rc == nil {
		return nil
	}
	clone := &RouteContext{
		DSName:    rc.DSName,
		Shadow:    rc.Shadow,
		TenantID:  rc.TenantID,
		ReadOnly:  rc.ReadOnly,
		RouteHint: rc.RouteHint,
	}
	if rc.Extra != nil {
		clone.Extra = make(map[string]interface{}, len(rc.Extra))
		for k, v := range rc.Extra {
			clone.Extra[k] = v
		}
	}
	return clone
}

// WithDSName 返回一个设置了指定数据源名称的克隆上下文
func (rc *RouteContext) WithDSName(name string) *RouteContext {
	c := rc.Clone()
	c.DSName = name
	return c
}

// WithShadow 返回一个设置了影子标记的克隆上下文
func (rc *RouteContext) WithShadow(shadow bool) *RouteContext {
	c := rc.Clone()
	c.Shadow = shadow
	return c
}

// WithTenantID 返回一个设置了租户ID的克隆上下文
func (rc *RouteContext) WithTenantID(tenantID string) *RouteContext {
	c := rc.Clone()
	c.TenantID = tenantID
	return c
}

// WithReadOnly 返回一个设置了只读标记的克隆上下文
func (rc *RouteContext) WithReadOnly(readOnly bool) *RouteContext {
	c := rc.Clone()
	c.ReadOnly = readOnly
	return c
}

// WithRouteHint 返回一个设置了路由提示的克隆上下文
func (rc *RouteContext) WithRouteHint(hint string) *RouteContext {
	c := rc.Clone()
	c.RouteHint = hint
	return c
}

// WithExtra 返回一个添加了扩展字段的克隆上下文
func (rc *RouteContext) WithExtra(key string, value interface{}) *RouteContext {
	c := rc.Clone()
	if c.Extra == nil {
		c.Extra = make(map[string]interface{})
	}
	c.Extra[key] = value
	return c
}

// RouteContextBuilder 路由上下文构建器 - 使用构建器模式一次性构建 RouteContext，
// 避免多次 Clone 造成的堆分配开销
type RouteContextBuilder struct {
	dsName    string
	shadow    bool
	tenantID  string
	readOnly  bool
	routeHint string
	extra     [][2]interface{}
}

// NewRouteContextBuilder 创建一个新的路由上下文构建器
func NewRouteContextBuilder() *RouteContextBuilder {
	return &RouteContextBuilder{}
}

// DSName 设置数据源名称
func (b *RouteContextBuilder) DSName(name string) *RouteContextBuilder {
	b.dsName = name
	return b
}

// Shadow 设置影子标记
func (b *RouteContextBuilder) Shadow(shadow bool) *RouteContextBuilder {
	b.shadow = shadow
	return b
}

// TenantID 设置租户ID
func (b *RouteContextBuilder) TenantID(tenantID string) *RouteContextBuilder {
	b.tenantID = tenantID
	return b
}

// ReadOnly 设置只读标记
func (b *RouteContextBuilder) ReadOnly(readOnly bool) *RouteContextBuilder {
	b.readOnly = readOnly
	return b
}

// RouteHint 设置路由提示
func (b *RouteContextBuilder) RouteHint(hint string) *RouteContextBuilder {
	b.routeHint = hint
	return b
}

// Extra 添加扩展字段
func (b *RouteContextBuilder) Extra(key string, value interface{}) *RouteContextBuilder {
	b.extra = append(b.extra, [2]interface{}{key, value})
	return b
}

// Build 根据构建器中设置的所有字段构建 RouteContext 实例
func (b *RouteContextBuilder) Build() *RouteContext {
	rc := &RouteContext{
		DSName:    b.dsName,
		Shadow:    b.shadow,
		TenantID:  b.tenantID,
		ReadOnly:  b.readOnly,
		RouteHint: b.routeHint,
	}
	if len(b.extra) > 0 {
		rc.Extra = make(map[string]interface{}, len(b.extra))
		for _, kv := range b.extra {
			rc.Extra[kv[0].(string)] = kv[1]
		}
	}
	return rc
}

// NewRouteContext 创建一个带有初始化 Extra 映射的路由上下文
func NewRouteContext() *RouteContext {
	return &RouteContext{
		Extra: make(map[string]interface{}),
	}
}

// extractRouteContext 从 context.Context 中提取路由上下文，不存在则返回 nil
func extractRouteContext(ctx context.Context) *RouteContext {
	if ctx == nil {
		return nil
	}
	if val := ctx.Value(contextKey); val != nil {
		if rc, ok := val.(*RouteContext); ok {
			return rc
		}
	}
	return nil
}

// WithRouteContext 将路由上下文注入到 context.Context 中
func WithRouteContext(ctx context.Context, routeCtx *RouteContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey, routeCtx)
}

// GetRouteContext 获取当前上下文中的路由上下文（公开API）
func GetRouteContext(ctx context.Context) *RouteContext {
	return extractRouteContext(ctx)
}

// Use 声明式切换数据源 - 在上下文中指定要使用的数据源名称
func Use(ctx context.Context, dsName string) context.Context {
	rc := extractRouteContext(ctx)
	if rc == nil {
		rc = NewRouteContext()
	} else {
		rc = rc.Clone()
	}
	rc.DSName = dsName
	return WithRouteContext(ctx, rc)
}

// WithDS 创建一个使用指定数据源的上下文包装函数，用于中间件链式调用
func WithDS(dsName string, fn func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		newCtx := Use(ctx, dsName)
		return fn(newCtx)
	}
}

// WithGroup 在上下文中指定要使用的分组名称
func WithGroup(ctx context.Context, group string) context.Context {
	rc := extractRouteContext(ctx)
	if rc == nil {
		rc = NewRouteContext()
	} else {
		rc = rc.Clone()
	}
	rc.DSName = group
	return WithRouteContext(ctx, rc)
}

// WithShadow 在上下文中设置影子流量标记
func WithShadow(ctx context.Context, shadow bool) context.Context {
	rc := extractRouteContext(ctx)
	if rc == nil {
		rc = NewRouteContext()
	} else {
		rc = rc.Clone()
	}
	rc.Shadow = shadow
	return WithRouteContext(ctx, rc)
}

// WithTenant 在上下文中设置租户ID
func WithTenant(ctx context.Context, tenantID string) context.Context {
	rc := extractRouteContext(ctx)
	if rc == nil {
		rc = NewRouteContext()
	} else {
		rc = rc.Clone()
	}
	rc.TenantID = tenantID
	return WithRouteContext(ctx, rc)
}

// WithReadOnly 在上下文中设置只读标记
func WithReadOnly(ctx context.Context, readOnly bool) context.Context {
	rc := extractRouteContext(ctx)
	if rc == nil {
		rc = NewRouteContext()
	} else {
		rc = rc.Clone()
	}
	rc.ReadOnly = readOnly
	return WithRouteContext(ctx, rc)
}

// WithRouteHint 在上下文中设置路由提示
func WithRouteHint(ctx context.Context, hint string) context.Context {
	rc := extractRouteContext(ctx)
	if rc == nil {
		rc = NewRouteContext()
	} else {
		rc = rc.Clone()
	}
	rc.RouteHint = hint
	return WithRouteContext(ctx, rc)
}

// CurrentDS 获取当前上下文中指定的数据源名称，未设置则返回空字符串
func CurrentDS(ctx context.Context) string {
	rc := extractRouteContext(ctx)
	if rc == nil {
		return ""
	}
	return rc.DSName
}
