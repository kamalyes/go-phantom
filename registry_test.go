/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\registry_test.go
 * @Description: 测试注册中心
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"fmt"
	"sync"
	"testing"

	"github.com/kamalyes/go-logger"
	"github.com/stretchr/testify/assert"
)

func TestDataSource_Health(t *testing.T) {
	ds := &DataSource{Name: "test"}
	ds.Healthy.Store(true)
	assert.True(t, ds.IsHealthy())

	ds.MarkUnhealthy()
	assert.False(t, ds.IsHealthy())

	ds.MarkHealthy()
	assert.True(t, ds.IsHealthy())
}

func TestGroup_AddRemoveSource(t *testing.T) {
	g := &Group{
		Name:        "test_group",
		StorageType: StorageDatabase,
	}

	primary := &DataSource{Name: "primary", StorageType: StorageDatabase}
	shadow := &DataSource{Name: "shadow", StorageType: StorageDatabase, Shadow: true}
	readOnly := &DataSource{Name: "readonly", StorageType: StorageDatabase, ReadOnly: true}

	g.AddSource(primary)
	g.AddSource(shadow)
	g.AddSource(readOnly)

	assert.Equal(t, primary, g.Primary)
	assert.Len(t, g.Sources, 3)
	assert.Len(t, g.Shadows, 1)
	assert.Equal(t, shadow, g.Shadows[0])

	readOnlys := g.GetReadOnlySources()
	assert.Len(t, readOnlys, 1)

	g.RemoveSource("primary")
	assert.Nil(t, g.Primary)
	assert.Len(t, g.Sources, 2)
	assert.Len(t, g.Shadows, 1)
}

func TestRegistry_GroupOperations(t *testing.T) {
	r := NewRegistry(nil)

	g := &Group{Name: "db_group", StorageType: StorageDatabase}
	err := r.RegisterGroup(g)
	assert.NoError(t, err)

	err = r.RegisterGroup(g)
	assert.Error(t, err)

	found, ok := r.GetGroup("db_group")
	assert.True(t, ok)
	assert.Equal(t, g, found)

	_, ok = r.GetGroup("nonexistent")
	assert.False(t, ok)

	names := r.ListGroups()
	assert.Contains(t, names, "db_group")

	r.RemoveGroup("db_group")
	_, ok = r.GetGroup("db_group")
	assert.False(t, ok)
}

func TestRegistry_AddSourceToGroup(t *testing.T) {
	r := NewRegistry(nil)

	err := r.AddSourceToGroup("nonexistent", &DataSource{Name: "test"})
	assert.Error(t, err)

	g := &Group{Name: "db_group", StorageType: StorageDatabase}
	r.RegisterGroup(g)

	ds := &DataSource{Name: "primary", StorageType: StorageDatabase}
	err = r.AddSourceToGroup("db_group", ds)
	assert.NoError(t, err)

	found, _ := r.GetGroup("db_group")
	assert.Equal(t, ds, found.Primary)
}

func TestRegistry_HealthCheckAll(t *testing.T) {
	r := NewRegistry(nil)

	g := &Group{Name: "db_group", StorageType: StorageDatabase}
	ds := &DataSource{Name: "primary", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	r.RegisterGroup(g)

	health := r.HealthCheckAll()
	assert.Contains(t, health, "db_group")
	assert.Contains(t, health["db_group"], "primary")
	assert.True(t, health["db_group"]["primary"])
}

func TestRegistry_ListGroupsByType(t *testing.T) {
	r := NewRegistry(nil)

	g1 := NewGroup("db_group1", StorageDatabase)
	g2 := NewGroup("db_group2", StorageDatabase)
	g3 := NewGroup("redis_group", StorageRedis)
	r.RegisterGroup(g1)
	r.RegisterGroup(g2)
	r.RegisterGroup(g3)

	dbGroups := r.ListGroupsByType(StorageDatabase)
	assert.Len(t, dbGroups, 2)
	assert.Contains(t, dbGroups, "db_group1")
	assert.Contains(t, dbGroups, "db_group2")

	redisGroups := r.ListGroupsByType(StorageRedis)
	assert.Len(t, redisGroups, 1)
	assert.Contains(t, redisGroups, "redis_group")

	customGroups := r.ListGroupsByType(StorageCustom)
	assert.Empty(t, customGroups)
}

func TestRegistry_RemoveGroup(t *testing.T) {
	r := NewRegistry(nil)
	g := NewGroup("test", StorageDatabase)
	r.RegisterGroup(g)

	_, ok := r.GetGroup("test")
	assert.True(t, ok)

	r.RemoveGroup("test")
	_, ok = r.GetGroup("test")
	assert.False(t, ok)
}

func TestRegistry_RemoveSourceFromGroup_Nonexistent(t *testing.T) {
	r := NewRegistry(nil)
	err := r.RemoveSourceFromGroup("nonexistent", "ds1")
	assert.Error(t, err)
}

func TestGroup_CacheInvalidation(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds1 := &DataSource{Name: "ds1", Weight: 1}
	ds1.Healthy.Store(true)
	g.AddSource(ds1)

	healthy := g.GetHealthySources()
	assert.Len(t, healthy, 1)

	ds2 := &DataSource{Name: "ds2", Weight: 1}
	ds2.Healthy.Store(true)
	g.AddSource(ds2)
	g.InvalidateCache()

	healthy = g.GetHealthySources()
	assert.Len(t, healthy, 2)
}

func TestGroup_HealthyCache_AutoRefresh(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds := &DataSource{Name: "ds1", Weight: 1}
	ds.Healthy.Store(true)
	g.AddSource(ds)

	healthy := g.GetHealthySources()
	assert.Len(t, healthy, 1)

	ds.MarkUnhealthy()
	g.InvalidateCache()

	healthy = g.GetHealthySources()
	assert.Len(t, healthy, 0)
}

func TestGroup_ShadowCache(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds := &DataSource{Name: "shadow1", Shadow: true, Weight: 1}
	ds.Healthy.Store(true)
	g.AddSource(ds)

	shadows := g.GetHealthyShadows()
	assert.Len(t, shadows, 1)

	ds.MarkUnhealthy()
	g.InvalidateCache()

	shadows = g.GetHealthyShadows()
	assert.Len(t, shadows, 0)
}

func TestGroup_ReadOnlyCache(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds := &DataSource{Name: "readonly1", ReadOnly: true, Weight: 1}
	ds.Healthy.Store(true)
	g.AddSource(ds)

	readOnlys := g.GetReadOnlySources()
	assert.Len(t, readOnlys, 1)
}

func TestGroup_RemoveSource_PrimaryReElection(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds1 := &DataSource{Name: "ds1", Weight: 1}
	ds1.Healthy.Store(true)
	ds2 := &DataSource{Name: "ds2", Weight: 1}
	ds2.Healthy.Store(true)

	g.AddSource(ds1)
	g.AddSource(ds2)

	assert.Equal(t, ds1, g.Primary)

	g.RemoveSource("ds1")
	assert.Equal(t, ds2, g.Primary)
}

func TestGroup_RemoveSource_ShadowRemoval(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds := &DataSource{Name: "shadow1", Shadow: true, Weight: 1}
	ds.Healthy.Store(true)
	g.AddSource(ds)

	assert.Len(t, g.Shadows, 1)

	g.RemoveSource("shadow1")
	assert.Len(t, g.Shadows, 0)
}

func TestGroup_GetSource_O1(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds := &DataSource{Name: "ds1", Weight: 1}
	ds.Healthy.Store(true)
	g.AddSource(ds)

	found := g.GetSource("ds1")
	assert.Equal(t, ds, found)

	notFound := g.GetSource("nonexistent")
	assert.Nil(t, notFound)
}

func TestGroup_NoSources(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	assert.Nil(t, g.Primary)
	assert.Empty(t, g.Sources)
	assert.Empty(t, g.GetHealthySources())
	assert.Empty(t, g.GetHealthyShadows())
}

func TestGroup_RemoveSource_TenantMap(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds := &DataSource{Name: "tenant_ds", TenantID: "t1", Weight: 1}
	ds.Healthy.Store(true)
	g.AddSource(ds)

	assert.Equal(t, 1, g.TenantCount())

	g.RemoveSource("tenant_ds")
	assert.Equal(t, 0, g.TenantCount())
}

func TestGroup_GetSource_NilMap(t *testing.T) {
	g := &Group{Name: "test", StorageType: StorageDatabase}
	found := g.GetSource("nonexistent")
	assert.Nil(t, found)
}

func TestGroup_GetSourceByTenant_NilMap(t *testing.T) {
	g := &Group{Name: "test", StorageType: StorageDatabase}
	found := g.GetSourceByTenant("t1")
	assert.Nil(t, found)
}

func TestGroup_TenantCount(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	assert.Equal(t, 0, g.TenantCount())

	ds1 := &DataSource{Name: "tenant_1", TenantID: "t1", Weight: 1}
	ds1.Healthy.Store(true)
	g.AddSource(ds1)
	assert.Equal(t, 1, g.TenantCount())

	ds2 := &DataSource{Name: "tenant_2", TenantID: "t2", Weight: 1}
	ds2.Healthy.Store(true)
	g.AddSource(ds2)
	assert.Equal(t, 2, g.TenantCount())
}

func TestGroup_GetSourceByTenant(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds := &DataSource{Name: "tenant_ds", TenantID: "t1", Weight: 1}
	ds.Healthy.Store(true)
	g.AddSource(ds)

	found := g.GetSourceByTenant("t1")
	assert.Equal(t, ds, found)

	notFound := g.GetSourceByTenant("nonexistent")
	assert.Nil(t, notFound)
}

func TestGroup_GetHealthyShadows_CachePath(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds := &DataSource{Name: "shadow1", Shadow: true, Weight: 1}
	ds.Healthy.Store(true)
	g.AddSource(ds)

	shadows1 := g.GetHealthyShadows()
	assert.Len(t, shadows1, 1)

	shadows2 := g.GetHealthyShadows()
	assert.Len(t, shadows2, 1)
}

func TestGroup_GetReadOnlySources_CachePath(t *testing.T) {
	g := NewGroup("test", StorageDatabase)

	ds := &DataSource{Name: "readonly1", ReadOnly: true, Weight: 1}
	ds.Healthy.Store(true)
	g.AddSource(ds)

	ro1 := g.GetReadOnlySources()
	assert.Len(t, ro1, 1)

	ro2 := g.GetReadOnlySources()
	assert.Len(t, ro2, 1)
}

func TestGroup_GetReadOnlySources_Empty(t *testing.T) {
	g := NewGroup("test", StorageDatabase)
	ds := &DataSource{Name: "primary", Weight: 1}
	ds.Healthy.Store(true)
	g.AddSource(ds)

	ro := g.GetReadOnlySources()
	assert.Empty(t, ro)
}

func TestGroup_GetHealthySources_ConcurrentCachePath(t *testing.T) {
	g := NewGroup("test", StorageDatabase)
	for i := 0; i < 5; i++ {
		ds := &DataSource{Name: fmt.Sprintf("ds%d", i), Weight: 1}
		ds.Healthy.Store(true)
		g.AddSource(ds)
	}

	g.GetHealthySources()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			g.InvalidateCache()
		}()
		go func() {
			defer wg.Done()
			g.GetHealthySources()
		}()
	}
	wg.Wait()
}

func TestGroup_GetHealthyShadows_ConcurrentCachePath(t *testing.T) {
	g := NewGroup("test", StorageDatabase)
	for i := 0; i < 5; i++ {
		ds := &DataSource{Name: fmt.Sprintf("shadow%d", i), Shadow: true, Weight: 1}
		ds.Healthy.Store(true)
		g.AddSource(ds)
	}

	g.GetHealthyShadows()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			g.InvalidateCache()
		}()
		go func() {
			defer wg.Done()
			g.GetHealthyShadows()
		}()
	}
	wg.Wait()
}

func TestGroup_GetReadOnlySources_ConcurrentCachePath(t *testing.T) {
	g := NewGroup("test", StorageDatabase)
	for i := 0; i < 5; i++ {
		ds := &DataSource{Name: fmt.Sprintf("ro%d", i), ReadOnly: true, Weight: 1}
		ds.Healthy.Store(true)
		g.AddSource(ds)
	}

	g.GetReadOnlySources()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			g.InvalidateCache()
		}()
		go func() {
			defer wg.Done()
			g.GetReadOnlySources()
		}()
	}
	wg.Wait()
}

func TestRegistry_AddSourceToGroup_WithLogger(t *testing.T) {
	log := logger.NewLogger()
	r := NewRegistry(log)

	g := NewGroup("db_group", StorageDatabase)
	r.RegisterGroup(g)

	ds := &DataSource{Name: "primary_db", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	err := r.AddSourceToGroup("db_group", ds)
	assert.NoError(t, err)
}

func TestRegistry_ForEach(t *testing.T) {
	r := NewRegistry(nil)
	g1 := NewGroup("g1", StorageDatabase)
	g2 := NewGroup("g2", StorageRedis)
	r.RegisterGroup(g1)
	r.RegisterGroup(g2)

	names := []string{}
	r.ForEach(func(name string, group *Group) {
		names = append(names, name)
	})
	assert.Len(t, names, 2)
	assert.Contains(t, names, "g1")
	assert.Contains(t, names, "g2")
}
