/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\phantom_test.go
 * @Description: 测试幻影引擎
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/kamalyes/go-logger"
	"github.com/stretchr/testify/assert"
)

func TestPhantom_DeclarativeSwitching(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	slave := &DataSource{Name: "slave_db", StorageType: StorageDatabase, ReadOnly: true, Weight: 1}
	slave.Healthy.Store(true)
	dbGroup.AddSource(primary)
	dbGroup.AddSource(slave)

	p.RegisterGroup(dbGroup)
	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "primary")
	p.Initialize(context.Background())

	source, err := p.Resolve(context.Background(), "db_group")
	assert.NoError(t, err)
	assert.Equal(t, primary, source)

	ctx := Use(context.Background(), "slave_db")
	source, err = p.Resolve(ctx, "db_group")
	assert.NoError(t, err)
	assert.Equal(t, slave, source)

	ctx = Use(context.Background(), "primary_db")
	source, err = p.Resolve(ctx, "db_group")
	assert.NoError(t, err)
	assert.Equal(t, primary, source)
}

func TestPhantom_WithDS(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	slave := &DataSource{Name: "slave_db", StorageType: StorageDatabase, ReadOnly: true, Weight: 1}
	slave.Healthy.Store(true)
	dbGroup.AddSource(primary)
	dbGroup.AddSource(slave)

	p.RegisterGroup(dbGroup)
	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "primary")
	p.Initialize(context.Background())

	var capturedName string
	fn := WithDS("slave_db", func(ctx context.Context) error {
		capturedName = CurrentDS(ctx)
		source, err := p.Resolve(ctx, "db_group")
		assert.NoError(t, err)
		assert.Equal(t, slave, source)
		return nil
	})

	err := fn(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "slave_db", capturedName)
}

func TestPhantom_FullFlow(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	shadow := &DataSource{Name: "shadow_db", StorageType: StorageDatabase, Shadow: true, Weight: 1}
	shadow.Healthy.Store(true)
	dbGroup.AddSource(primary)
	dbGroup.AddSource(shadow)

	err := p.RegisterGroup(dbGroup)
	assert.NoError(t, err)

	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "primary")

	err = p.Initialize(context.Background())
	assert.NoError(t, err)

	source, err := p.Resolve(context.Background(), "db_group")
	assert.NoError(t, err)
	assert.Equal(t, primary, source)

	ctx := WithShadow(context.Background(), true)
	source, err = p.ResolveWithRoute(ctx, "db_group", GetRouteContext(ctx))
	assert.NoError(t, err)
	assert.Equal(t, shadow, source)

	source, err = p.ResolveByType(context.Background(), StorageDatabase)
	assert.NoError(t, err)
	assert.Equal(t, primary, source)
}

func TestPhantom_GetDB_GetRedis(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "default_db", StorageType: StorageDatabase}
	primaryDB := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primaryDB.Healthy.Store(true)
	dbGroup.AddSource(primaryDB)
	p.RegisterGroup(dbGroup)
	p.SetDefaultGroup(StorageDatabase, "default_db")
	p.SetGroupStrategy("default_db", "primary")

	redisGroup := &Group{Name: "default_redis", StorageType: StorageRedis}
	primaryRedis := &DataSource{Name: "primary_redis", StorageType: StorageRedis, Weight: 1}
	primaryRedis.Healthy.Store(true)
	redisGroup.AddSource(primaryRedis)
	p.RegisterGroup(redisGroup)
	p.SetDefaultGroup(StorageRedis, "default_redis")
	p.SetGroupStrategy("default_redis", "primary")

	p.Initialize(context.Background())

	ds, err := p.GetDB(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, primaryDB, ds)

	ds, err = p.GetRedis(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, primaryRedis, ds)

	ctx := Use(context.Background(), "primary_db")
	ds, err = p.GetDB(ctx)
	assert.NoError(t, err)
	assert.Equal(t, primaryDB, ds)
}

func TestPhantom_Close(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	err := p.Close()
	assert.NoError(t, err)
}

func TestPhantom_ResolveByType_NoDefaultGroup(t *testing.T) {
	p := NewPhantom()
	_, err := p.ResolveByType(context.Background(), StorageDatabase)
	assert.Error(t, err)
	assert.Equal(t, ErrNoDefaultGroup, err)
}

func TestPhantom_Resolve_NonexistentGroup(t *testing.T) {
	p := NewPhantom()
	_, err := p.Resolve(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestPhantom_ResolveByDSName_Unhealthy(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	ds := &DataSource{Name: "unhealthy_db", StorageType: StorageDatabase}
	dbGroup.AddSource(ds)
	ds.MarkUnhealthy()

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	ctx := Use(context.Background(), "unhealthy_db")
	_, err := p.Resolve(ctx, "db_group")
	assert.Error(t, err)
}

func TestPhantom_ResolveByDSName_NotFound(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	ctx := Use(context.Background(), "nonexistent_db")
	_, err := p.Resolve(ctx, "db_group")
	assert.Error(t, err)
}

func TestPhantom_SetGroupStrategy_NotFound(t *testing.T) {
	p := NewPhantom()
	err := p.SetGroupStrategy("db_group", "nonexistent_strategy")
	assert.Error(t, err)
	assert.Equal(t, ErrStrategyNotFound, err)
}

func TestPhantom_GetDB_NoDefaultGroup(t *testing.T) {
	p := NewPhantom()
	_, err := p.GetDB(context.Background())
	assert.Error(t, err)
}

func TestPhantom_GetRedis_NoDefaultGroup(t *testing.T) {
	p := NewPhantom()
	_, err := p.GetRedis(context.Background())
	assert.Error(t, err)
}

func TestPhantom_GetCustom_NoDefaultGroup(t *testing.T) {
	p := NewPhantom()
	_, err := p.GetCustom(context.Background(), StorageCustom)
	assert.Error(t, err)
}

func TestPhantom_GetDB_WithGroupName(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.SetGroupStrategy("db_group", "primary")
	p.Initialize(context.Background())

	ds, err := p.GetDB(context.Background(), "db_group")
	assert.NoError(t, err)
	assert.Equal(t, primary, ds)
}

func TestPhantom_RemoveSource(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	err := p.RemoveSource("db_group", "primary_db")
	assert.NoError(t, err)

	err = p.RemoveSource("nonexistent", "primary_db")
	assert.Error(t, err)
}

func TestPhantom_GetAccessors(t *testing.T) {
	p := NewPhantom()

	assert.NotNil(t, p.GetRegistry())
	assert.NotNil(t, p.GetRouter())
	assert.NotNil(t, p.GetShadowManager())
	assert.NotNil(t, p.GetHealthCheckManager())
}

func TestPhantom_RegisterHealthChecker(t *testing.T) {
	p := NewPhantom()
	p.RegisterHealthChecker(StorageCustom, &GenericHealthChecker{})
}

func TestPhantom_ShadowResolve_FallbackToNormal(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.SetGroupStrategy("db_group", "primary")
	p.Initialize(context.Background())

	ctx := WithShadow(context.Background(), true)
	source, err := p.Resolve(ctx, "db_group")
	assert.NoError(t, err)
	assert.Equal(t, primary, source)
}

func TestPhantom_ResolveWithRoute(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.SetGroupStrategy("db_group", "primary")
	p.Initialize(context.Background())

	routeCtx := NewRouteContext()
	source, err := p.ResolveWithRoute(context.Background(), "db_group", routeCtx)
	assert.NoError(t, err)
	assert.Equal(t, primary, source)
}

func TestPhantom_ResolveWithRoute_DSName(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	slave := &DataSource{Name: "slave_db", StorageType: StorageDatabase, ReadOnly: true, Weight: 1}
	slave.Healthy.Store(true)
	dbGroup.AddSource(primary)
	dbGroup.AddSource(slave)

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	routeCtx := NewRouteContext().WithDSName("slave_db")
	source, err := p.ResolveWithRoute(context.Background(), "db_group", routeCtx)
	assert.NoError(t, err)
	assert.Equal(t, slave, source)
}

func TestPhantom_ResolveWithRoute_NilRouteCtx(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.SetGroupStrategy("db_group", "primary")
	p.Initialize(context.Background())

	source, err := p.ResolveWithRoute(context.Background(), "db_group", nil)
	assert.NoError(t, err)
	assert.Equal(t, primary, source)
}

func TestPhantom_DeclarativeSwitching_ReadWrite(t *testing.T) {
	p := NewPhantom(WithHealthCheck(HealthCheckConfig{Enabled: false}))

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	slave := &DataSource{Name: "slave_db", StorageType: StorageDatabase, ReadOnly: true, Weight: 1}
	slave.Healthy.Store(true)
	dbGroup.AddSource(primary)
	dbGroup.AddSource(slave)

	p.RegisterGroup(dbGroup)
	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "readwrite")
	p.Initialize(context.Background())

	writeCtx := context.Background()
	source, err := p.Resolve(writeCtx, "db_group")
	assert.NoError(t, err)
	assert.Equal(t, primary, source)

	readCtx := WithReadOnly(context.Background(), true)
	source, err = p.Resolve(readCtx, "db_group")
	assert.NoError(t, err)
	assert.Equal(t, slave, source)

	explicitCtx := Use(context.Background(), "slave_db")
	source, err = p.Resolve(explicitCtx, "db_group")
	assert.NoError(t, err)
	assert.Equal(t, slave, source)
}

func TestPhantom_DeclarativeSwitching_OverrideStrategy(t *testing.T) {
	p := NewPhantom(WithHealthCheck(HealthCheckConfig{Enabled: false}))

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	slave := &DataSource{Name: "slave_db", StorageType: StorageDatabase, ReadOnly: true, Weight: 1}
	slave.Healthy.Store(true)
	dbGroup.AddSource(primary)
	dbGroup.AddSource(slave)

	p.RegisterGroup(dbGroup)
	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "primary")
	p.Initialize(context.Background())

	source, err := p.Resolve(context.Background(), "db_group")
	assert.NoError(t, err)
	assert.Equal(t, primary, source)

	p.SetGroupStrategy("db_group", "readonly")
	source, err = p.Resolve(context.Background(), "db_group")
	assert.NoError(t, err)
	assert.Equal(t, slave, source)
}

func TestPhantom_ConcurrentAccess(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	slave := &DataSource{Name: "slave_db", StorageType: StorageDatabase, ReadOnly: true, Weight: 1}
	slave.Healthy.Store(true)
	dbGroup.AddSource(primary)
	dbGroup.AddSource(slave)

	p.RegisterGroup(dbGroup)
	p.SetDefaultGroup(StorageDatabase, "db_group")
	p.SetGroupStrategy("db_group", "roundrobin")
	p.Initialize(context.Background())

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Resolve(context.Background(), "db_group")
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("并发访问出错: %v", err)
	}
}

func TestPhantom_ConcurrentShadowRuleAccess(t *testing.T) {
	p := NewPhantom()

	dbGroup := &Group{Name: "db_group", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Weight: 1}
	primary.Healthy.Store(true)
	dbGroup.AddSource(primary)

	p.RegisterGroup(dbGroup)
	p.Initialize(context.Background())

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rule := &ShadowRule{
				Enabled:   true,
				GroupName: "db_group",
				ShadowDS:  fmt.Sprintf("shadow_%d", i),
				MatchRules: []*ShadowMatchRule{
					{Type: ShadowMatchTag, Values: []string{fmt.Sprintf("tag_%d", i)}},
				},
			}
			p.RegisterShadowRule("db_group", rule)
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.IsShadowTraffic(context.Background(), "db_group")
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("并发影子规则访问出错: %v", err)
	}
}

func TestPhantom_Options(t *testing.T) {
	log := logger.NewLogger()
	p := NewPhantom(
		WithLogger(log),
		WithDefaultGroup(StorageDatabase, "db_group"),
		WithHealthCheck(HealthCheckConfig{Enabled: false}),
	)

	assert.NotNil(t, p)
}

func TestPhantom_Initialize_AlreadyInitialized(t *testing.T) {
	p := NewPhantom()
	err := p.Initialize(context.Background())
	assert.NoError(t, err)

	err = p.Initialize(context.Background())
	assert.NoError(t, err)
}

func TestPhantom_WithStrategy(t *testing.T) {
	custom := &PrimaryStrategy{}
	p := NewPhantom(WithStrategy("custom", custom))
	err := p.SetGroupStrategy("db_group", "custom")
	assert.NoError(t, err)
}

func TestPhantom_ResolveWithRoute_NonexistentGroup(t *testing.T) {
	p := NewPhantom()
	p.Initialize(context.Background())

	routeCtx := NewRouteContext().WithDSName("some_db")
	_, err := p.ResolveWithRoute(context.Background(), "nonexistent", routeCtx)
	assert.Error(t, err)
}

func TestPhantom_ResolveWithRoute_ShadowNonexistentGroup(t *testing.T) {
	p := NewPhantom()
	p.Initialize(context.Background())

	routeCtx := NewRouteContext().WithShadow(true)
	_, err := p.ResolveWithRoute(context.Background(), "nonexistent", routeCtx)
	assert.Error(t, err)
}

func TestPhantom_ResolveWithRoute_ShadowFallback(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	ds := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	p.RegisterGroup(g)
	p.Initialize(context.Background())

	routeCtx := NewRouteContext().WithShadow(true)
	result, err := p.ResolveWithRoute(context.Background(), "db_group", routeCtx)
	assert.NoError(t, err)
	assert.Equal(t, "primary_db", result.Name)
}

func TestPhantom_ResolveWithRoute_WithDSName(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	ds := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	p.RegisterGroup(g)
	p.Initialize(context.Background())

	routeCtx := NewRouteContext().WithDSName("primary_db")
	result, err := p.ResolveWithRoute(context.Background(), "db_group", routeCtx)
	assert.NoError(t, err)
	assert.Equal(t, "primary_db", result.Name)
}

func TestPhantom_AddSource(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	primary.Healthy.Store(true)
	g.AddSource(primary)
	p.RegisterGroup(g)
	p.Initialize(context.Background())

	newDS := &DataSource{Name: "new_db", StorageType: StorageDatabase}
	newDS.Healthy.Store(true)
	err := p.AddSource("db_group", newDS)
	assert.NoError(t, err)

	err = p.AddSource("nonexistent", newDS)
	assert.Error(t, err)
}

func TestPhantom_ResolveWithRoute_Shadow(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	primary := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	primary.Healthy.Store(true)
	shadow := &DataSource{Name: "shadow_db", StorageType: StorageDatabase, Shadow: true}
	shadow.Healthy.Store(true)
	g.AddSource(primary)
	g.AddSource(shadow)
	p.RegisterGroup(g)
	p.SetGroupStrategy("db_group", "primary")
	p.Initialize(context.Background())

	routeCtx := NewRouteContext().WithShadow(true)
	source, err := p.ResolveWithRoute(context.Background(), "db_group", routeCtx)
	assert.NoError(t, err)
	assert.Equal(t, shadow, source)
}

func TestPhantom_GetCustom(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("custom_group", StorageCustom)
	ds := &DataSource{Name: "custom_ds", StorageType: StorageCustom}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	p.RegisterGroup(g)
	p.SetDefaultGroup(StorageCustom, "custom_group")
	p.SetGroupStrategy("custom_group", "primary")
	p.Initialize(context.Background())

	source, err := p.GetCustom(context.Background(), StorageCustom)
	assert.NoError(t, err)
	assert.Equal(t, ds, source)
}

func TestPhantom_Close_WithClosableInstance(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)

	closable := &mockClosable{closed: false}
	ds := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Instance: closable}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	p.RegisterGroup(g)
	p.Initialize(context.Background())

	err := p.Close()
	assert.NoError(t, err)
	assert.True(t, closable.closed)
}

type mockClosable struct {
	closed bool
}

func (m *mockClosable) Close() error {
	m.closed = true
	return nil
}

func TestPhantom_Close_WithClosableError(t *testing.T) {
	p := NewPhantom(WithLogger(logger.NewLogger()))
	g := NewGroup("db_group", StorageDatabase)

	closable := &mockClosableWithError{}
	ds := &DataSource{Name: "primary_db", StorageType: StorageDatabase, Instance: closable}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	p.RegisterGroup(g)
	p.Initialize(context.Background())

	err := p.Close()
	assert.NoError(t, err)
}

type mockClosableWithError struct{}

func (m *mockClosableWithError) Close() error {
	return fmt.Errorf("close error")
}

func TestPhantom_HealthCheck(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	ds := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	p.RegisterGroup(g)

	result := p.HealthCheck()
	assert.Contains(t, result, "db_group")
}

func TestPhantom_IsShadowTraffic(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	ds := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	p.RegisterGroup(g)
	p.Initialize(context.Background())

	p.RegisterShadowRule("db_group", &ShadowRule{
		Enabled:   true,
		GroupName: "db_group",
		ShadowDS:  "shadow_db",
		MatchRules: []*ShadowMatchRule{
			{Type: ShadowMatchTag, Values: []string{"test"}},
		},
	})

	assert.False(t, p.IsShadowTraffic(context.Background(), "db_group"))
}

func TestPhantom_IsShadowEnabled(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	ds := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	p.RegisterGroup(g)
	p.Initialize(context.Background())

	p.RegisterShadowRule("db_group", &ShadowRule{Enabled: true, GroupName: "db_group"})
	assert.True(t, p.IsShadowEnabled("db_group"))
}

func TestPhantom_Resolve_NilRouteContext(t *testing.T) {
	p := NewPhantom()
	g := NewGroup("db_group", StorageDatabase)
	ds := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	p.RegisterGroup(g)
	p.SetGroupStrategy("db_group", "primary")
	p.Initialize(context.Background())

	source, err := p.Resolve(context.Background(), "db_group")
	assert.NoError(t, err)
	assert.Equal(t, ds, source)
}
