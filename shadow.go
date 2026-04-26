/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\shadow.go
 * @Description: 幻影引擎影子流量管理 - 支持基于请求头、用户ID、租户ID、
 *   IP范围、百分比和标签等多种匹配规则的影子流量识别
 *   使用 FNV-1a 哈希实现稳定的百分比分流，确保同一标识的请求始终路由到相同环境
 *   使用 syncx.Map 管理影子规则，保证并发安全
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// ShadowMatchType 影子流量匹配类型
type ShadowMatchType string

const (
	ShadowMatchHeader   ShadowMatchType = "header"    // 请求头匹配
	ShadowMatchUserID   ShadowMatchType = "user_id"   // 用户ID匹配
	ShadowMatchTenantID ShadowMatchType = "tenant_id" // 租户ID匹配
	ShadowMatchIPRange  ShadowMatchType = "ip_range"  // IP范围匹配
	ShadowMatchPercent  ShadowMatchType = "percent"   // 百分比分流
	ShadowMatchTag      ShadowMatchType = "tag"       // 标签匹配
	ShadowMatchCustom   ShadowMatchType = "custom"    // 自定义匹配
)

// ShadowLogic 多规则组合逻辑
type ShadowLogic string

const (
	ShadowLogicOR  ShadowLogic = "or"  // 任一规则匹配即为影子流量
	ShadowLogicAND ShadowLogic = "and" // 所有规则匹配才为影子流量
)

// ShadowMatcher 自定义影子流量匹配函数
type ShadowMatcher func(ctx context.Context) bool

// ShadowMatchRule 影子流量匹配规则
type ShadowMatchRule struct {
	Type    ShadowMatchType // 匹配类型
	Key     string          // 匹配键（如请求头名称）
	Values  []string        // 匹配值列表
	Percent int             // 百分比分流值（1-100）
	Matcher ShadowMatcher   // 自定义匹配函数
}

// ShadowRule 影子流量规则，定义一个分组的影子流量识别逻辑
type ShadowRule struct {
	Enabled     bool               // 是否启用
	Logic       ShadowLogic        // 多规则组合逻辑（AND/OR）
	GroupName   string             // 所属分组名称
	ShadowDS    string             // 影子数据源名称
	ShadowTable string             // 影子表名
	MatchRules  []*ShadowMatchRule // 匹配规则列表
	FailSilent  bool               // 匹配失败时是否静默处理
}

// Match 判断当前请求是否匹配影子流量规则
func (r *ShadowRule) Match(ctx context.Context) bool {
	if !r.Enabled {
		return false
	}

	if len(r.MatchRules) == 0 {
		return false
	}

	logic := r.Logic
	if logic == "" {
		logic = ShadowLogicOR
	}

	switch logic {
	case ShadowLogicAND:
		for _, rule := range r.MatchRules {
			if !rule.Match(ctx) {
				return false
			}
		}
		return true
	default:
		for _, rule := range r.MatchRules {
			if rule.Match(ctx) {
				return true
			}
		}
		return false
	}
}

// Match 执行单条匹配规则
func (m *ShadowMatchRule) Match(ctx context.Context) bool {
	switch m.Type {
	case ShadowMatchHeader:
		return m.matchHeader(ctx)
	case ShadowMatchUserID:
		return m.matchUserID(ctx)
	case ShadowMatchTenantID:
		return m.matchTenantID(ctx)
	case ShadowMatchTag:
		return m.matchTag(ctx)
	case ShadowMatchPercent:
		return m.matchPercent(ctx)
	case ShadowMatchCustom:
		if m.Matcher != nil {
			return m.Matcher(ctx)
		}
		return false
	default:
		return false
	}
}

// matchHeader 匹配请求头中的指定键值
func (m *ShadowMatchRule) matchHeader(ctx context.Context) bool {
	routeCtx := extractRouteContext(ctx)
	if routeCtx == nil {
		return false
	}
	if val, ok := routeCtx.Extra[m.Key]; ok {
		strVal := fmt.Sprintf("%v", val)
		for _, v := range m.Values {
			if strVal == v {
				return true
			}
		}
	}
	return false
}

// matchUserID 匹配用户ID
func (m *ShadowMatchRule) matchUserID(ctx context.Context) bool {
	routeCtx := extractRouteContext(ctx)
	if routeCtx == nil {
		return false
	}
	if val, ok := routeCtx.Extra["user_id"]; ok {
		strVal := fmt.Sprintf("%v", val)
		for _, v := range m.Values {
			if strVal == v {
				return true
			}
		}
	}
	return false
}

// matchTenantID 匹配租户ID
func (m *ShadowMatchRule) matchTenantID(ctx context.Context) bool {
	routeCtx := extractRouteContext(ctx)
	if routeCtx == nil {
		return false
	}
	for _, v := range m.Values {
		if routeCtx.TenantID == v {
			return true
		}
	}
	return false
}

// matchTag 匹配影子标签（不区分大小写）
func (m *ShadowMatchRule) matchTag(ctx context.Context) bool {
	routeCtx := extractRouteContext(ctx)
	if routeCtx == nil {
		return false
	}
	if val, ok := routeCtx.Extra["shadow_tag"]; ok {
		strVal := fmt.Sprintf("%v", val)
		for _, v := range m.Values {
			if strings.EqualFold(strVal, v) {
				return true
			}
		}
	}
	return false
}

// matchPercent 使用 FNV-1a 哈希进行稳定的百分比分流
// 同一标识（租户ID/用户ID/请求ID）的请求始终路由到相同环境
func (m *ShadowMatchRule) matchPercent(ctx context.Context) bool {
	routeCtx := extractRouteContext(ctx)
	if routeCtx == nil {
		return false
	}
	var seed string
	if routeCtx.TenantID != "" {
		seed = routeCtx.TenantID
	} else if val, ok := routeCtx.Extra["user_id"]; ok {
		seed = fmt.Sprintf("%v", val)
	} else if val, ok := routeCtx.Extra["request_id"]; ok {
		seed = fmt.Sprintf("%v", val)
	} else {
		return false
	}

	h := fnv.New32a()
	h.Write([]byte(seed))
	percent := int(h.Sum32()%100) + 1
	return percent <= m.Percent
}

// ShadowManager 影子流量管理器，管理各分组的影子流量规则
type ShadowManager struct {
	rules    *syncx.Map[string, *ShadowRule] // 分组到影子规则的映射，使用 syncx.Map 保证并发安全
	registry *Registry
	logger   logger.ILogger
}

// NewShadowManager 创建影子流量管理器
func NewShadowManager(registry *Registry, log logger.ILogger) *ShadowManager {
	return &ShadowManager{
		rules:    syncx.NewMap[string, *ShadowRule](),
		registry: registry,
		logger:   log,
	}
}

// RegisterRule 为指定分组注册影子流量规则
func (sm *ShadowManager) RegisterRule(groupName string, rule *ShadowRule) {
	sm.rules.Store(groupName, rule)
}

// IsShadowTraffic 判断当前请求是否为指定分组的影子流量
func (sm *ShadowManager) IsShadowTraffic(ctx context.Context, groupName string) bool {
	rule, ok := sm.rules.Load(groupName)
	if !ok {
		return false
	}
	return rule.Match(ctx)
}

// IsShadowEnabled 检查指定分组是否启用了影子流量
func (sm *ShadowManager) IsShadowEnabled(groupName string) bool {
	if rule, ok := sm.rules.Load(groupName); ok {
		return rule.Enabled
	}
	return false
}

// GetShadowRule 获取指定分组的影子流量规则
func (sm *ShadowManager) GetShadowRule(groupName string) (*ShadowRule, bool) {
	return sm.rules.Load(groupName)
}

// GetShadowTable 获取指定分组的影子表名
func (sm *ShadowManager) GetShadowTable(groupName string) string {
	if rule, ok := sm.rules.Load(groupName); ok {
		return rule.ShadowTable
	}
	return ""
}

// GetShadowDS 获取指定分组的影子数据源名称
func (sm *ShadowManager) GetShadowDS(groupName string) string {
	if rule, ok := sm.rules.Load(groupName); ok {
		return rule.ShadowDS
	}
	return ""
}
