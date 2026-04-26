/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\health_test.go
 * @Description: 测试健康检查管理器
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kamalyes/go-logger"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheckManager_StartStop(t *testing.T) {
	r := NewRegistry(nil)
	config := HealthCheckConfig{Enabled: false}
	hcm := NewHealthCheckManager(r, config, nil)

	hcm.Start(context.Background())
	hcm.Stop()
}

func TestHealthCheckManager_RegisterChecker(t *testing.T) {
	r := NewRegistry(nil)
	config := HealthCheckConfig{Enabled: false}
	hcm := NewHealthCheckManager(r, config, nil)

	custom := &GenericHealthChecker{}
	hcm.RegisterChecker(StorageCustom, custom)

	checker, ok := hcm.checkers.Load(StorageCustom)
	assert.True(t, ok)
	assert.Equal(t, custom, checker)
}

func TestHealthCheckManager_CheckAll(t *testing.T) {
	r := NewRegistry(nil)
	config := HealthCheckConfig{Enabled: false, Interval: 1 * time.Hour, Timeout: 1 * time.Second}
	hcm := NewHealthCheckManager(r, config, nil)

	g := NewGroup("test", StorageDatabase)
	ds := &DataSource{Name: "ds1", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	r.RegisterGroup(g)

	hcm.checkAll(context.Background())
}

func TestDBHealthChecker_NilInstance(t *testing.T) {
	c := &DBHealthChecker{}
	ds := &DataSource{}
	ds.Healthy.Store(true)
	assert.True(t, c.Check(context.Background(), ds))

	ds.MarkUnhealthy()
	assert.False(t, c.Check(context.Background(), ds))
}

func TestRedisHealthChecker_NilInstance(t *testing.T) {
	c := &RedisHealthChecker{}
	ds := &DataSource{}
	ds.Healthy.Store(true)
	assert.True(t, c.Check(context.Background(), ds))

	ds.MarkUnhealthy()
	assert.False(t, c.Check(context.Background(), ds))
}

func TestGenericHealthChecker_NilInstance(t *testing.T) {
	c := &GenericHealthChecker{}
	ds := &DataSource{}
	ds.Healthy.Store(true)
	assert.True(t, c.Check(context.Background(), ds))

	ds.MarkUnhealthy()
	assert.False(t, c.Check(context.Background(), ds))
}

func TestDefaultHealthCheckConfig(t *testing.T) {
	config := DefaultHealthCheckConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, 10*time.Second, config.Interval)
	assert.Equal(t, 3*time.Second, config.Timeout)
}

func TestHealthCheckManager_Run(t *testing.T) {
	r := NewRegistry(nil)
	config := HealthCheckConfig{Enabled: true, Interval: 50 * time.Millisecond, Timeout: 1 * time.Second}
	hcm := NewHealthCheckManager(r, config, nil)

	g := NewGroup("test", StorageDatabase)
	ds := &DataSource{Name: "ds1", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	r.RegisterGroup(g)

	ctx, cancel := context.WithCancel(context.Background())
	hcm.Start(ctx)

	time.Sleep(150 * time.Millisecond)
	cancel()
	hcm.Stop()
}

func TestHealthCheckManager_CheckOne_HealthChange(t *testing.T) {
	r := NewRegistry(nil)
	config := HealthCheckConfig{Enabled: false}
	hcm := NewHealthCheckManager(r, config, nil)

	g := NewGroup("test", StorageDatabase)
	ds := &DataSource{Name: "ds1", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	r.RegisterGroup(g)

	hcm.RegisterChecker(StorageDatabase, &GenericHealthChecker{})

	hcm.checkOne(context.Background(), "test", ds)
	assert.True(t, ds.IsHealthy())

	ds.MarkUnhealthy()
	g.InvalidateCache()
	hcm.checkOne(context.Background(), "test", ds)
	assert.False(t, ds.IsHealthy())
}

func TestHealthCheckManager_CheckOne_NoChecker(t *testing.T) {
	r := NewRegistry(nil)
	config := HealthCheckConfig{Enabled: false}
	hcm := NewHealthCheckManager(r, config, nil)

	hcm.checkers.Delete(StorageDatabase)

	g := NewGroup("test", StorageDatabase)
	ds := &DataSource{Name: "ds1", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	r.RegisterGroup(g)

	hcm.checkOne(context.Background(), "test", ds)
}

func TestHealthCheckManager_CheckOne_HealthRecovery(t *testing.T) {
	log := logger.NewLogger()
	r := NewRegistry(log)
	config := HealthCheckConfig{Enabled: false}
	hcm := NewHealthCheckManager(r, config, log)

	g := NewGroup("test", StorageDatabase)
	ds := &DataSource{Name: "ds1", StorageType: StorageDatabase}
	ds.Healthy.Store(false)
	g.AddSource(ds)
	r.RegisterGroup(g)

	hcm.RegisterChecker(StorageDatabase, &alwaysHealthyChecker{})
	hcm.checkOne(context.Background(), "test", ds)
	assert.True(t, ds.IsHealthy())
}

func TestHealthCheckManager_CheckOne_HealthDegrade(t *testing.T) {
	log := logger.NewLogger()
	r := NewRegistry(log)
	config := HealthCheckConfig{Enabled: false}
	hcm := NewHealthCheckManager(r, config, log)

	g := NewGroup("test", StorageDatabase)
	ds := &DataSource{Name: "ds1", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	r.RegisterGroup(g)

	hcm.RegisterChecker(StorageDatabase, &alwaysUnhealthyChecker{})
	hcm.checkOne(context.Background(), "test", ds)
	assert.False(t, ds.IsHealthy())
}

type alwaysHealthyChecker struct{}

func (c *alwaysHealthyChecker) Check(_ context.Context, _ *DataSource) bool { return true }

type alwaysUnhealthyChecker struct{}

func (c *alwaysUnhealthyChecker) Check(_ context.Context, _ *DataSource) bool { return false }

func TestDBHealthChecker_WithPinger(t *testing.T) {
	c := &DBHealthChecker{}
	ds := &DataSource{Instance: &mockPinger{}}
	ds.Healthy.Store(true)
	assert.True(t, c.Check(context.Background(), ds))
}

func TestDBHealthChecker_WithDBGetter(t *testing.T) {
	c := &DBHealthChecker{}
	ds := &DataSource{Instance: &mockDBGetter{}}
	ds.Healthy.Store(true)
	assert.True(t, c.Check(context.Background(), ds))
}

func TestDBHealthChecker_WithDBGetterError(t *testing.T) {
	c := &DBHealthChecker{}
	ds := &DataSource{Instance: &mockDBGetterWithError{}}
	ds.Healthy.Store(true)
	assert.False(t, c.Check(context.Background(), ds))
}

func TestDBHealthChecker_WithDBGetterNoPinger(t *testing.T) {
	c := &DBHealthChecker{}
	ds := &DataSource{Instance: &mockDBGetterNoPinger{}}
	ds.Healthy.Store(true)
	assert.True(t, c.Check(context.Background(), ds))
}

func TestDBHealthChecker_NoPinger(t *testing.T) {
	c := &DBHealthChecker{}
	ds := &DataSource{Instance: "just_a_string"}
	ds.Healthy.Store(true)
	assert.True(t, c.Check(context.Background(), ds))
}

func TestRedisHealthChecker_WithPinger(t *testing.T) {
	c := &RedisHealthChecker{}
	ds := &DataSource{Instance: &mockRedisPinger{}}
	ds.Healthy.Store(true)
	assert.True(t, c.Check(context.Background(), ds))
}

func TestRedisHealthChecker_WithPingerError(t *testing.T) {
	c := &RedisHealthChecker{}
	ds := &DataSource{Instance: &mockRedisPingerWithError{}}
	ds.Healthy.Store(true)
	assert.False(t, c.Check(context.Background(), ds))
}

func TestRedisHealthChecker_NoPinger(t *testing.T) {
	c := &RedisHealthChecker{}
	ds := &DataSource{Instance: "just_a_string"}
	ds.Healthy.Store(true)
	assert.True(t, c.Check(context.Background(), ds))
}

func TestGenericHealthChecker_WithInstance(t *testing.T) {
	c := &GenericHealthChecker{}
	ds := &DataSource{Instance: "some_instance"}
	ds.Healthy.Store(true)
	assert.True(t, c.Check(context.Background(), ds))

	ds.MarkUnhealthy()
	assert.False(t, c.Check(context.Background(), ds))
}

type mockPinger struct{}

func (m *mockPinger) PingContext(ctx context.Context) error { return nil }

type mockDBGetter struct{}

func (m *mockDBGetter) DB() (interface{}, error) {
	return &mockPinger{}, nil
}

type mockDBGetterWithError struct{}

func (m *mockDBGetterWithError) DB() (interface{}, error) {
	return nil, fmt.Errorf("db error")
}

type mockDBGetterNoPinger struct{}

func (m *mockDBGetterNoPinger) DB() (interface{}, error) {
	return "not_a_pinger", nil
}

type mockRedisPinger struct{}

func (m *mockRedisPinger) Ping(ctx context.Context) interface{ Err() error } {
	return &mockRedisResult{}
}

type mockRedisResult struct{}

func (m *mockRedisResult) Err() error { return nil }

type mockRedisPingerWithError struct{}

func (m *mockRedisPingerWithError) Ping(ctx context.Context) interface{ Err() error } {
	return &mockRedisResultWithError{}
}

type mockRedisResultWithError struct{}

func (m *mockRedisResultWithError) Err() error { return fmt.Errorf("redis error") }
