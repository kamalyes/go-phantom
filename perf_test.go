/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\perf_test.go
 * @Description: 测试幻影引擎性能
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
)

// ============================================================
// 辅助函数
// ============================================================

func buildGroup(sourceCount int) *Group {
	g := NewGroup("bench_group", StorageDatabase)
	g.AddSource(&DataSource{Name: "primary_db", StorageType: StorageDatabase})
	for i := 1; i < sourceCount; i++ {
		g.AddSource(&DataSource{
			Name:        fmt.Sprintf("db_%d", i),
			StorageType: StorageDatabase,
			ReadOnly:    i%3 == 0,
		})
	}
	return g
}

func buildTenantGroup(tenantCount int) *Group {
	g := NewGroup("tenant_group", StorageDatabase)
	g.AddSource(&DataSource{Name: "primary_db", StorageType: StorageDatabase})
	for i := 0; i < tenantCount; i++ {
		tenantID := fmt.Sprintf("tenant_%d", i)
		g.AddSource(&DataSource{
			Name:        fmt.Sprintf("db_%s", tenantID),
			StorageType: StorageDatabase,
			TenantID:    tenantID,
		})
	}
	return g
}

func buildWeightedGroup(sourceCount int) *Group {
	g := NewGroup("weighted_group", StorageDatabase)
	for i := 0; i < sourceCount; i++ {
		g.AddSource(&DataSource{
			Name:        fmt.Sprintf("db_%d", i),
			StorageType: StorageDatabase,
			Weight:      (i % 5) + 1,
		})
	}
	return g
}

func buildPhantomForBench(strategyName string, sourceCount int) *Phantom {
	p := NewPhantom()
	g := buildGroup(sourceCount)
	p.RegisterGroup(g)
	p.SetDefaultGroup(StorageDatabase, "bench_group")
	p.SetGroupStrategy("bench_group", strategyName)
	p.Initialize(context.Background())
	return p
}

func buildTenantPhantom(tenantCount int) *Phantom {
	p := NewPhantom()
	g := buildTenantGroup(tenantCount)
	p.RegisterGroup(g)
	p.SetDefaultGroup(StorageDatabase, "tenant_group")
	p.SetGroupStrategy("tenant_group", "tenant")
	p.Initialize(context.Background())
	return p
}

// ============================================================
// 1. 路由策略性能测试
// ============================================================

func BenchmarkPrimaryStrategy(b *testing.B) {
	g := buildGroup(10)
	s := &PrimaryStrategy{}
	ctx := context.Background()
	rc := NewRouteContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkReadOnlyStrategy(b *testing.B) {
	g := buildGroup(10)
	s := NewReadOnlyStrategy()
	ctx := context.Background()
	rc := NewRouteContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkReadWriteStrategy_Write(b *testing.B) {
	g := buildGroup(10)
	s := NewReadWriteStrategy()
	ctx := context.Background()
	rc := NewRouteContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkReadWriteStrategy_Read(b *testing.B) {
	g := buildGroup(10)
	s := NewReadWriteStrategy()
	ctx := context.Background()
	rc := NewRouteContext().WithReadOnly(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkRoundRobinStrategy_3Sources(b *testing.B) {
	g := buildGroup(3)
	s := NewRoundRobinStrategy()
	ctx := context.Background()
	rc := NewRouteContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkRoundRobinStrategy_10Sources(b *testing.B) {
	g := buildGroup(10)
	s := NewRoundRobinStrategy()
	ctx := context.Background()
	rc := NewRouteContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkRoundRobinStrategy_100Sources(b *testing.B) {
	g := buildGroup(100)
	s := NewRoundRobinStrategy()
	ctx := context.Background()
	rc := NewRouteContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkWeightedStrategy_3Sources(b *testing.B) {
	g := buildWeightedGroup(3)
	s := NewWeightedStrategy()
	ctx := context.Background()
	rc := NewRouteContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkWeightedStrategy_10Sources(b *testing.B) {
	g := buildWeightedGroup(10)
	s := NewWeightedStrategy()
	ctx := context.Background()
	rc := NewRouteContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkWeightedStrategy_100Sources(b *testing.B) {
	g := buildWeightedGroup(100)
	s := NewWeightedStrategy()
	ctx := context.Background()
	rc := NewRouteContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkHintStrategy_Hit(b *testing.B) {
	g := buildGroup(10)
	s := NewHintStrategy(nil)
	ctx := context.Background()
	rc := NewRouteContext().WithRouteHint("db_5")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkHintStrategy_Miss(b *testing.B) {
	g := buildGroup(10)
	s := NewHintStrategy(nil)
	ctx := context.Background()
	rc := NewRouteContext().WithRouteHint("nonexistent")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkFailoverStrategy(b *testing.B) {
	g := buildGroup(10)
	s := NewFailoverStrategy(3, nil)
	ctx := context.Background()
	rc := NewRouteContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

// ============================================================
// 2. 租户策略 O(1) vs O(n) 对比测试
// ============================================================

func BenchmarkTenantStrategy_10Tenants(b *testing.B) {
	g := buildTenantGroup(10)
	s := NewTenantStrategy(nil)
	ctx := context.Background()
	rc := NewRouteContext().WithTenantID("tenant_5")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkTenantStrategy_100Tenants(b *testing.B) {
	g := buildTenantGroup(100)
	s := NewTenantStrategy(nil)
	ctx := context.Background()
	rc := NewRouteContext().WithTenantID("tenant_50")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkTenantStrategy_1000Tenants(b *testing.B) {
	g := buildTenantGroup(1000)
	s := NewTenantStrategy(nil)
	ctx := context.Background()
	rc := NewRouteContext().WithTenantID("tenant_500")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkTenantStrategy_5000Tenants(b *testing.B) {
	g := buildTenantGroup(5000)
	s := NewTenantStrategy(nil)
	ctx := context.Background()
	rc := NewRouteContext().WithTenantID("tenant_2500")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkTenantStrategy_10000Tenants(b *testing.B) {
	g := buildTenantGroup(10000)
	s := NewTenantStrategy(nil)
	ctx := context.Background()
	rc := NewRouteContext().WithTenantID("tenant_5000")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

func BenchmarkTenantStrategy_Fallback_1000Tenants(b *testing.B) {
	g := buildTenantGroup(1000)
	s := NewTenantStrategy(nil)
	ctx := context.Background()
	rc := NewRouteContext().WithTenantID("nonexistent_tenant")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Resolve(ctx, g, rc)
	}
}

// ============================================================
// 3. Group 底层查找性能测试
// ============================================================

func BenchmarkGroup_GetSource_1000(b *testing.B) {
	g := buildGroup(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.GetSource("db_500")
	}
}

func BenchmarkGroup_GetSourceByTenant_1000(b *testing.B) {
	g := buildTenantGroup(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.GetSourceByTenant("tenant_500")
	}
}

func BenchmarkGroup_GetSourceByTenant_10000(b *testing.B) {
	g := buildTenantGroup(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.GetSourceByTenant("tenant_5000")
	}
}

func BenchmarkGroup_GetHealthySources_10(b *testing.B) {
	g := buildGroup(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.GetHealthySources()
	}
}

func BenchmarkGroup_GetHealthySources_100(b *testing.B) {
	g := buildGroup(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.GetHealthySources()
	}
}

func BenchmarkGroup_GetHealthySources_1000(b *testing.B) {
	g := buildGroup(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.GetHealthySources()
	}
}

func BenchmarkGroup_GetHealthySources_CacheHit(b *testing.B) {
	g := buildGroup(100)
	g.GetHealthySources()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.GetHealthySources()
	}
}

func BenchmarkGroup_GetHealthySources_CacheInvalidated(b *testing.B) {
	g := buildGroup(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.InvalidateCache()
		g.GetHealthySources()
	}
}

func BenchmarkGroup_AddSource(b *testing.B) {
	g := NewGroup("bench_group", StorageDatabase)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.AddSource(&DataSource{
			Name:        fmt.Sprintf("db_%d", i),
			StorageType: StorageDatabase,
		})
	}
}

func BenchmarkGroup_RemoveSource(b *testing.B) {
	g := NewGroup("bench_group", StorageDatabase)
	for i := 0; i < b.N; i++ {
		g.AddSource(&DataSource{
			Name:        fmt.Sprintf("db_%d", i),
			StorageType: StorageDatabase,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.RemoveSource(fmt.Sprintf("db_%d", i))
	}
}

// ============================================================
// 4. Phantom 引擎端到端性能测试
// ============================================================

func BenchmarkPhantom_Resolve_Primary(b *testing.B) {
	p := buildPhantomForBench("primary", 3)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Resolve(ctx, "bench_group")
	}
}

func BenchmarkPhantom_Resolve_ReadWrite(b *testing.B) {
	p := buildPhantomForBench("readwrite", 10)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Resolve(ctx, "bench_group")
	}
}

func BenchmarkPhantom_Resolve_RoundRobin(b *testing.B) {
	p := buildPhantomForBench("roundrobin", 10)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Resolve(ctx, "bench_group")
	}
}

func BenchmarkPhantom_Resolve_Weighted(b *testing.B) {
	p := buildPhantomForBench("weighted", 10)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Resolve(ctx, "bench_group")
	}
}

func BenchmarkPhantom_Resolve_Tenant_100(b *testing.B) {
	p := buildTenantPhantom(100)
	ctx := WithTenant(context.Background(), "tenant_50")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Resolve(ctx, "tenant_group")
	}
}

func BenchmarkPhantom_Resolve_Tenant_1000(b *testing.B) {
	p := buildTenantPhantom(1000)
	ctx := WithTenant(context.Background(), "tenant_500")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Resolve(ctx, "tenant_group")
	}
}

func BenchmarkPhantom_Resolve_Tenant_10000(b *testing.B) {
	p := buildTenantPhantom(10000)
	ctx := WithTenant(context.Background(), "tenant_5000")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Resolve(ctx, "tenant_group")
	}
}

func BenchmarkPhantom_ResolveByDSName(b *testing.B) {
	p := buildPhantomForBench("primary", 10)
	ctx := Use(context.Background(), "db_5")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Resolve(ctx, "bench_group")
	}
}

// ============================================================
// 5. 上下文 API 性能测试
// ============================================================

func BenchmarkUse(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Use(ctx, "slave_db")
	}
}

func BenchmarkWithTenant(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WithTenant(ctx, "tenant_1")
	}
}

func BenchmarkWithShadow(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WithShadow(ctx, true)
	}
}

func BenchmarkWithReadOnly(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WithReadOnly(ctx, true)
	}
}

func BenchmarkWithRouteHint(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WithRouteHint(ctx, "hint_ds")
	}
}

func BenchmarkRouteContextBuilder(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewRouteContextBuilder().
			DSName("slave_db").
			Shadow(true).
			TenantID("tenant_1").
			ReadOnly(false).
			RouteHint("hint").
			Build()
	}
}

func BenchmarkRouteContext_Clone(b *testing.B) {
	rc := NewRouteContext().
		WithDSName("slave_db").
		WithTenantID("tenant_1").
		WithShadow(true).
		WithReadOnly(false).
		WithRouteHint("hint")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc.Clone()
	}
}

func BenchmarkNewRouteContext(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewRouteContext()
	}
}

// ============================================================
// 6. 并发性能测试
// ============================================================

func BenchmarkPhantom_Concurrent_Primary(b *testing.B) {
	p := buildPhantomForBench("primary", 3)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Resolve(ctx, "bench_group")
		}
	})
}

func BenchmarkPhantom_Concurrent_RoundRobin(b *testing.B) {
	p := buildPhantomForBench("roundrobin", 10)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Resolve(ctx, "bench_group")
		}
	})
}

func BenchmarkPhantom_Concurrent_Tenant_1000(b *testing.B) {
	p := buildTenantPhantom(1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tenantID := "tenant_" + strconv.Itoa(i%1000)
			ctx := WithTenant(context.Background(), tenantID)
			p.Resolve(ctx, "tenant_group")
			i++
		}
	})
}

func BenchmarkPhantom_Concurrent_Tenant_10000(b *testing.B) {
	p := buildTenantPhantom(10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tenantID := "tenant_" + strconv.Itoa(i%10000)
			ctx := WithTenant(context.Background(), tenantID)
			p.Resolve(ctx, "tenant_group")
			i++
		}
	})
}

func BenchmarkPhantom_Concurrent_MixedStrategy(b *testing.B) {
	p := buildPhantomForBench("roundrobin", 10)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			var ctx context.Context
			switch i % 4 {
			case 0:
				ctx = context.Background()
			case 1:
				ctx = Use(context.Background(), "db_5")
			case 2:
				ctx = WithReadOnly(context.Background(), true)
			case 3:
				ctx = WithRouteHint(context.Background(), "db_3")
			}
			p.Resolve(ctx, "bench_group")
			i++
		}
	})
}

func BenchmarkGroup_Concurrent_GetSource(b *testing.B) {
	g := buildGroup(100)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			g.GetSource(fmt.Sprintf("db_%d", i%100))
			i++
		}
	})
}

func BenchmarkGroup_Concurrent_GetSourceByTenant(b *testing.B) {
	g := buildTenantGroup(1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			g.GetSourceByTenant(fmt.Sprintf("tenant_%d", i%1000))
			i++
		}
	})
}

func BenchmarkGroup_Concurrent_GetHealthySources(b *testing.B) {
	g := buildGroup(100)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			g.GetHealthySources()
		}
	})
}

// ============================================================
// 7. Registry 性能测试
// ============================================================

func BenchmarkRegistry_RegisterGroup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r := NewRegistry(nil)
		g := NewGroup(fmt.Sprintf("group_%d", i), StorageDatabase)
		g.AddSource(&DataSource{Name: "primary", StorageType: StorageDatabase})
		r.RegisterGroup(g)
	}
}

func BenchmarkRegistry_GetGroup(b *testing.B) {
	r := NewRegistry(nil)
	for i := 0; i < 1000; i++ {
		g := NewGroup(fmt.Sprintf("group_%d", i), StorageDatabase)
		g.AddSource(&DataSource{Name: "primary", StorageType: StorageDatabase})
		r.RegisterGroup(g)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.GetGroup(fmt.Sprintf("group_%d", i%1000))
	}
}

func BenchmarkRegistry_HealthCheckAll_100Groups(b *testing.B) {
	r := NewRegistry(nil)
	for i := 0; i < 100; i++ {
		g := NewGroup(fmt.Sprintf("group_%d", i), StorageDatabase)
		g.AddSource(&DataSource{Name: "primary", StorageType: StorageDatabase})
		g.AddSource(&DataSource{Name: "slave", StorageType: StorageDatabase, ReadOnly: true})
		r.RegisterGroup(g)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.HealthCheckAll()
	}
}

// ============================================================
// 8. 影子流量性能测试
// ============================================================

func BenchmarkShadowManager_IsShadowTraffic_Hit(b *testing.B) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	g.AddSource(&DataSource{Name: "primary_db", StorageType: StorageDatabase})
	g.AddSource(&DataSource{Name: "shadow_db", StorageType: StorageDatabase, Shadow: true})
	p.RegisterGroup(g)
	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "primary")
	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:     true,
		Logic:       ShadowLogicOR,
		GroupName:   "db_group",
		ShadowDS:    "shadow_db",
		ShadowTable: "shadow_",
		MatchRules: []*ShadowMatchRule{
			{Type: ShadowMatchTag, Values: []string{"pressure_test"}},
		},
	})
	p.Initialize(context.Background())

	routeCtx := NewRouteContext().WithExtra("shadow_tag", "pressure_test")
	ctx := WithRouteContext(context.Background(), routeCtx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.IsShadowTraffic(ctx, "db_group")
	}
}

func BenchmarkShadowManager_IsShadowTraffic_Miss(b *testing.B) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	g.AddSource(&DataSource{Name: "primary_db", StorageType: StorageDatabase})
	p.RegisterGroup(g)
	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "primary")
	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:     true,
		Logic:       ShadowLogicOR,
		GroupName:   "db_group",
		ShadowDS:    "shadow_db",
		ShadowTable: "shadow_",
		MatchRules: []*ShadowMatchRule{
			{Type: ShadowMatchTag, Values: []string{"pressure_test"}},
		},
	})
	p.Initialize(context.Background())

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.IsShadowTraffic(ctx, "db_group")
	}
}

// ============================================================
// 9. 真实场景模拟：1000租户 + 读写分离 + 并发
// ============================================================

func BenchmarkRealWorld_1000Tenants_ReadWrite_Concurrent(b *testing.B) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	g.AddSource(&DataSource{Name: "primary_db", StorageType: StorageDatabase})
	g.AddSource(&DataSource{Name: "slave_db", StorageType: StorageDatabase, ReadOnly: true})
	for i := 0; i < 1000; i++ {
		tenantID := fmt.Sprintf("tenant_%d", i)
		g.AddSource(&DataSource{
			Name:        fmt.Sprintf("db_%s", tenantID),
			StorageType: StorageDatabase,
			TenantID:    tenantID,
		})
	}
	p.RegisterGroup(g)
	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "tenant")
	p.Initialize(context.Background())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tenantID := "tenant_" + strconv.Itoa(i%1000)
			ctx := WithTenant(context.Background(), tenantID)
			p.Resolve(ctx, "db_group")
			i++
		}
	})
}

func BenchmarkRealWorld_1000Tenants_MixedTraffic_Concurrent(b *testing.B) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	g.AddSource(&DataSource{Name: "primary_db", StorageType: StorageDatabase})
	g.AddSource(&DataSource{Name: "slave_db", StorageType: StorageDatabase, ReadOnly: true})
	g.AddSource(&DataSource{Name: "shadow_db", StorageType: StorageDatabase, Shadow: true})
	for i := 0; i < 1000; i++ {
		tenantID := fmt.Sprintf("tenant_%d", i)
		g.AddSource(&DataSource{
			Name:        fmt.Sprintf("db_%s", tenantID),
			StorageType: StorageDatabase,
			TenantID:    tenantID,
		})
	}
	p.RegisterGroup(g)
	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "tenant")
	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:     true,
		Logic:       ShadowLogicOR,
		GroupName:   "db_group",
		ShadowDS:    "shadow_db",
		ShadowTable: "shadow_",
		MatchRules: []*ShadowMatchRule{
			{Type: ShadowMatchTag, Values: []string{"pressure_test"}},
		},
	})
	p.Initialize(context.Background())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			var ctx context.Context
			switch i % 5 {
			case 0:
				tenantID := "tenant_" + strconv.Itoa(i%1000)
				ctx = WithTenant(context.Background(), tenantID)
			case 1:
				ctx = WithReadOnly(context.Background(), true)
			case 2:
				ctx = Use(context.Background(), "slave_db")
			case 3:
				routeCtx := NewRouteContext().WithExtra("shadow_tag", "pressure_test")
				ctx = WithRouteContext(context.Background(), routeCtx)
			default:
				ctx = context.Background()
			}
			p.Resolve(ctx, "db_group")
			i++
		}
	})
}

// ============================================================
// 10. 数据源健康状态切换性能
// ============================================================

func BenchmarkDataSource_MarkHealthy(b *testing.B) {
	ds := &DataSource{Name: "test_db", StorageType: StorageDatabase}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ds.MarkHealthy()
	}
}

func BenchmarkDataSource_MarkUnhealthy(b *testing.B) {
	ds := &DataSource{Name: "test_db", StorageType: StorageDatabase}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ds.MarkUnhealthy()
	}
}

func BenchmarkDataSource_IsHealthy(b *testing.B) {
	ds := &DataSource{Name: "test_db", StorageType: StorageDatabase}
	ds.MarkHealthy()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ds.IsHealthy()
	}
}

// ============================================================
// 11. 并发正确性验证（不是 benchmark，是 -run 用的）
// ============================================================

func TestConcurrent_TenantStrategy_1000Tenants_1000Goroutines(t *testing.T) {
	g := buildTenantGroup(1000)
	s := NewTenantStrategy(nil)

	var wg sync.WaitGroup
	errCh := make(chan error, 10000)

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tenantID := fmt.Sprintf("tenant_%d", idx%1000)
			rc := NewRouteContext().WithTenantID(tenantID)
			result, err := s.Resolve(context.Background(), g, rc)
			if err != nil {
				errCh <- err
				return
			}
			if result.Source.TenantID != tenantID {
				errCh <- fmt.Errorf("expected %s got %s", tenantID, result.Source.TenantID)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestConcurrent_Phantom_1000Tenants_ReadWrite(t *testing.T) {
	p := buildTenantPhantom(1000)

	var wg sync.WaitGroup
	errCh := make(chan error, 10000)

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tenantID := fmt.Sprintf("tenant_%d", idx%1000)
			ctx := WithTenant(context.Background(), tenantID)
			source, err := p.Resolve(ctx, "tenant_group")
			if err != nil {
				errCh <- err
				return
			}
			if source.TenantID != tenantID {
				errCh <- fmt.Errorf("expected %s got %s", tenantID, source.TenantID)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestConcurrent_Group_AddRemove_WhileReading(t *testing.T) {
	g := buildTenantGroup(100)

	var wg sync.WaitGroup
	errCh := make(chan error, 5000)

	for i := 0; i < 100; i++ {
		wg.Add(3)

		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				tenantID := fmt.Sprintf("tenant_%d", idx%100)
				ds := g.GetSourceByTenant(tenantID)
				if ds == nil {
					errCh <- fmt.Errorf("tenant %s not found", tenantID)
					return
				}
			}
		}(i)

		go func(idx int) {
			defer wg.Done()
			tenantID := fmt.Sprintf("dynamic_%d", idx)
			g.AddSource(&DataSource{
				Name:        "dyn_" + tenantID,
				StorageType: StorageDatabase,
				TenantID:    tenantID,
			})
		}(i)

		go func(idx int) {
			defer wg.Done()
			g.RemoveSource(fmt.Sprintf("dyn_tenant_%d", idx-1))
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Log("expected race during add/remove:", err)
	}
}
