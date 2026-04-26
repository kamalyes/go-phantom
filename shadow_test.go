/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @FilePath: \go-phantom\shadow_test.go
 * @Description: 测试幻影规则
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPhantom_ShadowRule_TrafficIsolation(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	shadowDS := &DataSource{Name: "shadow_db", StorageType: StorageDatabase, Shadow: true, Weight: 1}
	shadowDS.Healthy.Store(true)
	dbGroup.AddSource(primary)
	dbGroup.AddSource(shadowDS)

	p.RegisterGroup(dbGroup)
	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "primary")
	p.Initialize(context.Background())

	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:     true,
		GroupName:   "db_group",
		ShadowDS:    "shadow_db",
		ShadowTable: "shadow_",
		MatchRules: []*ShadowMatchRule{
			{
				Type:   ShadowMatchTag,
				Values: []string{"pressure_test"},
			},
		},
		FailSilent: true,
	})

	normalCtx := context.Background()
	assert.False(t, p.IsShadowTraffic(normalCtx, "db_group"))

	shadowCtx := NewRouteContext().WithExtra("shadow_tag", "pressure_test")
	ctx := WithRouteContext(context.Background(), shadowCtx)
	assert.True(t, p.IsShadowTraffic(ctx, "db_group"))

	assert.True(t, p.IsShadowEnabled("db_group"))
	assert.False(t, p.IsShadowEnabled("nonexistent"))
}

func TestPhantom_ShadowRule_TenantMatch(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:   true,
		GroupName: "db_group",
		ShadowDS:  "shadow_db",
		MatchRules: []*ShadowMatchRule{
			{
				Type:   ShadowMatchTenantID,
				Values: []string{"test_tenant"},
			},
		},
	})

	tenantCtx := NewRouteContext().WithTenantID("test_tenant")
	ctx := WithRouteContext(context.Background(), tenantCtx)
	assert.True(t, p.IsShadowTraffic(ctx, "db_group"))

	otherCtx := NewRouteContext().WithTenantID("other_tenant")
	ctx2 := WithRouteContext(context.Background(), otherCtx)
	assert.False(t, p.IsShadowTraffic(ctx2, "db_group"))
}

func TestPhantom_ShadowRule_PercentMatch(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:   true,
		GroupName: "db_group",
		ShadowDS:  "shadow_db",
		MatchRules: []*ShadowMatchRule{
			{
				Type:    ShadowMatchPercent,
				Percent: 100,
			},
		},
	})

	routeCtx := NewRouteContext().WithExtra("user_id", "user_123")
	ctx := WithRouteContext(context.Background(), routeCtx)
	assert.True(t, p.IsShadowTraffic(ctx, "db_group"))

	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:   true,
		GroupName: "db_group",
		ShadowDS:  "shadow_db",
		MatchRules: []*ShadowMatchRule{
			{
				Type:    ShadowMatchPercent,
				Percent: 0,
			},
		},
	})
	assert.False(t, p.IsShadowTraffic(ctx, "db_group"))
}

func TestPhantom_ShadowRule_CustomMatcher(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:   true,
		GroupName: "db_group",
		ShadowDS:  "shadow_db",
		MatchRules: []*ShadowMatchRule{
			{
				Type: ShadowMatchCustom,
				Matcher: func(ctx context.Context) bool {
					rc := GetRouteContext(ctx)
					if rc == nil {
						return false
					}
					if v, ok := rc.Extra["force_shadow"]; ok {
						return v.(bool)
					}
					return false
				},
			},
		},
	})

	normalCtx := context.Background()
	assert.False(t, p.IsShadowTraffic(normalCtx, "db_group"))

	forceCtx := NewRouteContext().WithExtra("force_shadow", true)
	ctx := WithRouteContext(context.Background(), forceCtx)
	assert.True(t, p.IsShadowTraffic(ctx, "db_group"))
}

func TestPhantom_ShadowRule_Disabled(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:   false,
		GroupName: "db_group",
		MatchRules: []*ShadowMatchRule{
			{
				Type:   ShadowMatchTag,
				Values: []string{"pressure_test"},
			},
		},
	})

	shadowCtx := NewRouteContext().WithExtra("shadow_tag", "pressure_test")
	ctx := WithRouteContext(context.Background(), shadowCtx)
	assert.False(t, p.IsShadowTraffic(ctx, "db_group"))
}

func TestPhantom_ShadowManager_GetShadowTable(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:     true,
		GroupName:   "db_group",
		ShadowDS:    "shadow_db",
		ShadowTable: "shadow_",
	})

	sm := p.GetShadowManager()
	assert.Equal(t, "shadow_", sm.GetShadowTable("db_group"))
	assert.Equal(t, "shadow_db", sm.GetShadowDS("db_group"))
	assert.Equal(t, "", sm.GetShadowTable("nonexistent"))
}

func TestShadowRule_AND_Logic(t *testing.T) {
	rule := &ShadowRule{
		Enabled: true,
		Logic:   ShadowLogicAND,
		MatchRules: []*ShadowMatchRule{
			{Type: ShadowMatchTenantID, Values: []string{"t1"}},
			{Type: ShadowMatchTag, Values: []string{"test"}},
		},
	}

	rc := NewRouteContext().WithTenantID("t1").WithExtra("shadow_tag", "test")
	ctx := WithRouteContext(context.Background(), rc)
	assert.True(t, rule.Match(ctx))

	rc2 := NewRouteContext().WithTenantID("t1")
	ctx2 := WithRouteContext(context.Background(), rc2)
	assert.False(t, rule.Match(ctx2))
}

func TestShadowRule_OR_Logic(t *testing.T) {
	rule := &ShadowRule{
		Enabled: true,
		Logic:   ShadowLogicOR,
		MatchRules: []*ShadowMatchRule{
			{Type: ShadowMatchTenantID, Values: []string{"t1"}},
			{Type: ShadowMatchTag, Values: []string{"test"}},
		},
	}

	rc := NewRouteContext().WithTenantID("t1")
	ctx := WithRouteContext(context.Background(), rc)
	assert.True(t, rule.Match(ctx))

	rc2 := NewRouteContext().WithExtra("shadow_tag", "test")
	ctx2 := WithRouteContext(context.Background(), rc2)
	assert.True(t, rule.Match(ctx2))

	rc3 := NewRouteContext().WithTenantID("t2")
	ctx3 := WithRouteContext(context.Background(), rc3)
	assert.False(t, rule.Match(ctx3))
}

func TestShadowRule_EmptyRules(t *testing.T) {
	rule := &ShadowRule{
		Enabled:    true,
		MatchRules: []*ShadowMatchRule{},
	}
	assert.False(t, rule.Match(context.Background()))
}

func TestShadowRule_DefaultLogic(t *testing.T) {
	rule := &ShadowRule{
		Enabled: true,
		Logic:   "",
		MatchRules: []*ShadowMatchRule{
			{Type: ShadowMatchTenantID, Values: []string{"t1"}},
		},
	}

	rc := NewRouteContext().WithTenantID("t1")
	ctx := WithRouteContext(context.Background(), rc)
	assert.True(t, rule.Match(ctx))
}

func TestShadowMatch_Header(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchHeader, Key: "X-Shadow", Values: []string{"true", "1"}}

	rc := NewRouteContext().WithExtra("X-Shadow", "true")
	ctx := WithRouteContext(context.Background(), rc)
	assert.True(t, rule.Match(ctx))

	rc2 := NewRouteContext().WithExtra("X-Shadow", "1")
	ctx2 := WithRouteContext(context.Background(), rc2)
	assert.True(t, rule.Match(ctx2))

	rc3 := NewRouteContext().WithExtra("X-Shadow", "false")
	ctx3 := WithRouteContext(context.Background(), rc3)
	assert.False(t, rule.Match(ctx3))
}

func TestShadowMatch_UserID(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchUserID, Values: []string{"user_1", "user_2"}}

	rc := NewRouteContext().WithExtra("user_id", "user_1")
	ctx := WithRouteContext(context.Background(), rc)
	assert.True(t, rule.Match(ctx))

	rc2 := NewRouteContext().WithExtra("user_id", "user_3")
	ctx2 := WithRouteContext(context.Background(), rc2)
	assert.False(t, rule.Match(ctx2))
}

func TestShadowMatch_TenantID(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchTenantID, Values: []string{"t1"}}

	rc := NewRouteContext().WithTenantID("t1")
	ctx := WithRouteContext(context.Background(), rc)
	assert.True(t, rule.Match(ctx))

	rc2 := NewRouteContext().WithTenantID("t2")
	ctx2 := WithRouteContext(context.Background(), rc2)
	assert.False(t, rule.Match(ctx2))
}

func TestShadowMatch_Tag_CaseInsensitive(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchTag, Values: []string{"Pressure_Test"}}

	rc := NewRouteContext().WithExtra("shadow_tag", "pressure_test")
	ctx := WithRouteContext(context.Background(), rc)
	assert.True(t, rule.Match(ctx))

	rc2 := NewRouteContext().WithExtra("shadow_tag", "PRESSURE_TEST")
	ctx2 := WithRouteContext(context.Background(), rc2)
	assert.True(t, rule.Match(ctx2))
}

func TestShadowMatch_Percent_NoSeed(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchPercent, Percent: 50}
	rc := NewRouteContext()
	ctx := WithRouteContext(context.Background(), rc)
	assert.False(t, rule.Match(ctx))
}

func TestShadowMatch_Percent_WithTenantID(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchPercent, Percent: 100}
	rc := NewRouteContext().WithTenantID("consistent_tenant")
	ctx := WithRouteContext(context.Background(), rc)

	result1 := rule.Match(ctx)
	result2 := rule.Match(ctx)
	assert.Equal(t, result1, result2)
	assert.True(t, result1)
}

func TestShadowMatch_Percent_WithUserID(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchPercent, Percent: 100}
	rc := NewRouteContext().WithExtra("user_id", "user_123")
	ctx := WithRouteContext(context.Background(), rc)
	assert.True(t, rule.Match(ctx))
}

func TestShadowMatch_Percent_WithRequestID(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchPercent, Percent: 100}
	rc := NewRouteContext().WithExtra("request_id", "req_123")
	ctx := WithRouteContext(context.Background(), rc)
	assert.True(t, rule.Match(ctx))
}

func TestShadowMatch_CustomMatcher(t *testing.T) {
	rule := &ShadowMatchRule{
		Type: ShadowMatchCustom,
		Matcher: func(ctx context.Context) bool {
			return true
		},
	}
	assert.True(t, rule.Match(context.Background()))

	rule2 := &ShadowMatchRule{
		Type:    ShadowMatchCustom,
		Matcher: nil,
	}
	assert.False(t, rule2.Match(context.Background()))
}

func TestShadowMatch_UnknownType(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchType("unknown")}
	assert.False(t, rule.Match(context.Background()))
}

func TestShadowMatch_NoRouteContext(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchHeader, Key: "X-Shadow", Values: []string{"true"}}
	assert.False(t, rule.Match(context.Background()))
}

func TestShadowMatchType_Constants(t *testing.T) {
	assert.Equal(t, ShadowMatchType("header"), ShadowMatchHeader)
	assert.Equal(t, ShadowMatchType("user_id"), ShadowMatchUserID)
	assert.Equal(t, ShadowMatchType("tenant_id"), ShadowMatchTenantID)
	assert.Equal(t, ShadowMatchType("ip_range"), ShadowMatchIPRange)
	assert.Equal(t, ShadowMatchType("percent"), ShadowMatchPercent)
	assert.Equal(t, ShadowMatchType("tag"), ShadowMatchTag)
	assert.Equal(t, ShadowMatchType("custom"), ShadowMatchCustom)
}

func TestShadowLogic_Constants(t *testing.T) {
	assert.Equal(t, ShadowLogic("or"), ShadowLogicOR)
	assert.Equal(t, ShadowLogic("and"), ShadowLogicAND)
}

func TestShadowMatch_UserID_NoRouteContext(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchUserID, Values: []string{"user_1"}}
	assert.False(t, rule.Match(context.Background()))
}

func TestShadowMatch_TenantID_NoRouteContext(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchTenantID, Values: []string{"t1"}}
	assert.False(t, rule.Match(context.Background()))
}

func TestShadowMatch_Percent_NoRouteContext(t *testing.T) {
	rule := &ShadowMatchRule{Type: ShadowMatchPercent, Percent: 50}
	assert.False(t, rule.Match(context.Background()))
}

func TestShadowManager_GetShadowRule(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	ds := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	p.RegisterGroup(g)
	p.Initialize(context.Background())

	rule := &ShadowRule{Enabled: true, GroupName: "db_group", ShadowDS: "shadow_db"}
	p.RegisterShadowRule("db_group", rule)

	sm := p.GetShadowManager()
	foundRule, ok := sm.GetShadowRule("db_group")
	assert.True(t, ok)
	assert.Equal(t, rule, foundRule)

	_, ok = sm.GetShadowRule("nonexistent")
	assert.False(t, ok)
}

func TestShadowManager_GetShadowDS_NoRule(t *testing.T) {
	p := NewPhantom()
	sm := p.GetShadowManager()
	assert.Equal(t, "", sm.GetShadowDS("nonexistent"))
}

func TestShadowManager_IsShadowTraffic_NoRule(t *testing.T) {
	p := NewPhantom()
	sm := p.GetShadowManager()
	assert.False(t, sm.IsShadowTraffic(context.Background(), "nonexistent"))
}
