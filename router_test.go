/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @FilePath: \go-phantom\router_test.go
 * @Description: 测试路由
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package phantom

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrimaryStrategy(t *testing.T) {
	s := &PrimaryStrategy{}
	g := &Group{Name: "test", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary"}
	primary.Healthy.Store(true)
	g.AddSource(primary)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.Equal(t, primary, result.Source)

	assert.Equal(t, "primary", s.Name())
}

func TestPrimaryStrategy_NoPrimary(t *testing.T) {
	s := &PrimaryStrategy{}
	g := &Group{Name: "test", StorageType: StorageDatabase}

	_, err := s.Resolve(context.Background(), g, nil)
	assert.Error(t, err)
}

func TestPrimaryStrategy_UnhealthyPrimary(t *testing.T) {
	s := &PrimaryStrategy{}
	g := &Group{Name: "test", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary"}
	g.AddSource(primary)
	primary.MarkUnhealthy()

	_, err := s.Resolve(context.Background(), g, nil)
	assert.Error(t, err)
}

func TestReadOnlyStrategy(t *testing.T) {
	s := &ReadOnlyStrategy{}
	g := &Group{Name: "test", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary"}
	primary.Healthy.Store(true)
	readOnly := &DataSource{Name: "readonly", ReadOnly: true}
	readOnly.Healthy.Store(true)
	g.AddSource(primary)
	g.AddSource(readOnly)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.Equal(t, readOnly, result.Source)

	assert.Equal(t, "readonly", s.Name())
}

func TestReadOnlyStrategy_FallbackToPrimary(t *testing.T) {
	s := &ReadOnlyStrategy{}
	g := &Group{Name: "test", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary"}
	primary.Healthy.Store(true)
	g.AddSource(primary)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.Equal(t, primary, result.Source)
}

func TestReadOnlyStrategy_NoReadSource(t *testing.T) {
	s := &ReadOnlyStrategy{}
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds := &DataSource{Name: "ds1"}
	g.AddSource(ds)
	ds.MarkUnhealthy()
	g.InvalidateCache()

	_, err := s.Resolve(context.Background(), g, nil)
	assert.Error(t, err)
}

func TestReadWriteStrategy(t *testing.T) {
	s := NewReadWriteStrategy()
	g := &Group{Name: "test", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary"}
	primary.Healthy.Store(true)
	readOnly := &DataSource{Name: "readonly", ReadOnly: true}
	readOnly.Healthy.Store(true)
	g.AddSource(primary)
	g.AddSource(readOnly)

	writeResult, err := s.Resolve(context.Background(), g, NewRouteContext())
	assert.NoError(t, err)
	assert.Equal(t, primary, writeResult.Source)

	readResult, err := s.Resolve(context.Background(), g, NewRouteContext().WithReadOnly(true))
	assert.NoError(t, err)
	assert.Equal(t, readOnly, readResult.Source)

	assert.Equal(t, "readwrite", s.Name())
}

func TestTenantStrategy(t *testing.T) {
	s := NewTenantStrategy(nil)
	g := &Group{Name: "test", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary"}
	primary.Healthy.Store(true)
	tenantDS := &DataSource{Name: "tenant_1", TenantID: "tenant_1"}
	tenantDS.Healthy.Store(true)
	g.AddSource(primary)
	g.AddSource(tenantDS)

	result, err := s.Resolve(context.Background(), g, NewRouteContext().WithTenantID("tenant_1"))
	assert.NoError(t, err)
	assert.Equal(t, tenantDS, result.Source)

	fallbackResult, err := s.Resolve(context.Background(), g, NewRouteContext())
	assert.NoError(t, err)
	assert.Equal(t, primary, fallbackResult.Source)

	assert.Equal(t, "tenant", s.Name())
}

func TestTenantStrategy_MultipleTenants(t *testing.T) {
	s := NewTenantStrategy(nil)
	g := &Group{Name: "test", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary"}
	primary.Healthy.Store(true)
	tenant1 := &DataSource{Name: "tenant_1", TenantID: "t1"}
	tenant1.Healthy.Store(true)
	tenant2 := &DataSource{Name: "tenant_2", TenantID: "t2"}
	tenant2.Healthy.Store(true)
	g.AddSource(primary)
	g.AddSource(tenant1)
	g.AddSource(tenant2)

	result1, err := s.Resolve(context.Background(), g, NewRouteContext().WithTenantID("t1"))
	assert.NoError(t, err)
	assert.Equal(t, tenant1, result1.Source)

	result2, err := s.Resolve(context.Background(), g, NewRouteContext().WithTenantID("t2"))
	assert.NoError(t, err)
	assert.Equal(t, tenant2, result2.Source)
}

func TestRoundRobinStrategy(t *testing.T) {
	s := NewRoundRobinStrategy()
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1"}
	ds1.Healthy.Store(true)
	ds2 := &DataSource{Name: "ds2"}
	ds2.Healthy.Store(true)
	g.AddSource(ds1)
	g.AddSource(ds2)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result.Source)

	assert.Equal(t, "roundrobin", s.Name())
}

func TestRoundRobinStrategy_NoHealthy(t *testing.T) {
	s := NewRoundRobinStrategy()
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1"}
	g.AddSource(ds1)
	ds1.MarkUnhealthy()
	g.InvalidateCache()

	_, err := s.Resolve(context.Background(), g, nil)
	assert.Error(t, err)
}

func TestRoundRobinStrategy_Sequential(t *testing.T) {
	s := NewRoundRobinStrategy()
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1"}
	ds1.Healthy.Store(true)
	ds2 := &DataSource{Name: "ds2"}
	ds2.Healthy.Store(true)
	g.AddSource(ds1)
	g.AddSource(ds2)

	names := map[string]bool{}
	for i := 0; i < 4; i++ {
		result, err := s.Resolve(context.Background(), g, nil)
		assert.NoError(t, err)
		names[result.Source.Name] = true
	}
	assert.True(t, names["ds1"])
	assert.True(t, names["ds2"])
}

func TestWeightedStrategy(t *testing.T) {
	s := NewWeightedStrategy()
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1", Weight: 3}
	ds1.Healthy.Store(true)
	ds2 := &DataSource{Name: "ds2", Weight: 1}
	ds2.Healthy.Store(true)
	g.AddSource(ds1)
	g.AddSource(ds2)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result.Source)

	assert.Equal(t, "weighted", s.Name())
}

func TestWeightedStrategy_SingleSource(t *testing.T) {
	s := NewWeightedStrategy()
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1", Weight: 1}
	ds1.Healthy.Store(true)
	g.AddSource(ds1)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.Equal(t, ds1, result.Source)
}

func TestWeightedStrategy_ZeroWeight(t *testing.T) {
	s := NewWeightedStrategy()
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1", Weight: 0}
	ds1.Healthy.Store(true)
	ds2 := &DataSource{Name: "ds2", Weight: 1}
	ds2.Healthy.Store(true)
	g.AddSource(ds1)
	g.AddSource(ds2)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result.Source)
}

func TestWeightedStrategy_NoHealthy(t *testing.T) {
	s := NewWeightedStrategy()
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1", Weight: 1}
	g.AddSource(ds1)
	ds1.MarkUnhealthy()
	g.InvalidateCache()

	_, err := s.Resolve(context.Background(), g, nil)
	assert.Error(t, err)
}

func TestHintStrategy(t *testing.T) {
	s := NewHintStrategy(nil)
	g := &Group{Name: "test", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary"}
	primary.Healthy.Store(true)
	hinted := &DataSource{Name: "hinted_ds"}
	hinted.Healthy.Store(true)
	g.AddSource(primary)
	g.AddSource(hinted)

	result, err := s.Resolve(context.Background(), g, NewRouteContext().WithRouteHint("hinted_ds"))
	assert.NoError(t, err)
	assert.Equal(t, hinted, result.Source)

	fallbackResult, err := s.Resolve(context.Background(), g, NewRouteContext())
	assert.NoError(t, err)
	assert.Equal(t, primary, fallbackResult.Source)

	assert.Equal(t, "hint", s.Name())
}

func TestHintStrategy_UnhealthyHint(t *testing.T) {
	s := NewHintStrategy(nil)
	g := &Group{Name: "test", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary"}
	hinted := &DataSource{Name: "hinted_ds"}
	g.AddSource(primary)
	g.AddSource(hinted)
	hinted.MarkUnhealthy()
	g.InvalidateCache()

	result, err := s.Resolve(context.Background(), g, NewRouteContext().WithRouteHint("hinted_ds"))
	assert.NoError(t, err)
	assert.Equal(t, primary, result.Source)
}

func TestFailoverStrategy(t *testing.T) {
	s := NewFailoverStrategy(3, nil)
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1", Weight: 1}
	ds1.Healthy.Store(true)
	ds2 := &DataSource{Name: "ds2", Weight: 1}
	ds2.Healthy.Store(true)
	g.AddSource(ds1)
	g.AddSource(ds2)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result.Source)
	assert.Equal(t, "failover", s.Name())
}

func TestFailoverStrategy_NoHealthySource(t *testing.T) {
	s := NewFailoverStrategy(3, nil)
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1", Weight: 1}
	g.AddSource(ds1)
	ds1.MarkUnhealthy()
	g.InvalidateCache()

	_, err := s.Resolve(context.Background(), g, nil)
	assert.Error(t, err)
}

func TestFailoverStrategy_DefaultRetries(t *testing.T) {
	s := NewFailoverStrategy(0, nil)
	assert.Equal(t, 3, s.maxRetries)
}

func TestFailoverStrategy_WithCustomFallback(t *testing.T) {
	s := NewFailoverStrategy(1, &PrimaryStrategy{})
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1", Weight: 1}
	ds1.Healthy.Store(true)
	g.AddSource(ds1)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result.Source)
}

func TestFailoverStrategy_FallbackIteration(t *testing.T) {
	s := NewFailoverStrategy(1, &failingStrategy{})
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1", Weight: 1}
	ds1.Healthy.Store(true)
	ds2 := &DataSource{Name: "ds2", Weight: 1}
	ds2.Healthy.Store(true)
	g.AddSource(ds1)
	g.AddSource(ds2)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result.Source)
}

type failingStrategy struct{}

func (s *failingStrategy) Resolve(_ context.Context, _ *Group, _ *RouteContext) (*RouteResult, error) {
	return nil, fmt.Errorf("always fails")
}
func (s *failingStrategy) Name() string { return "failing" }

func TestWeightedStrategy_FallbackToFirst(t *testing.T) {
	s := NewWeightedStrategy()
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1", Weight: 1}
	ds1.Healthy.Store(true)
	ds2 := &DataSource{Name: "ds2", Weight: 1}
	ds2.Healthy.Store(true)
	g.AddSource(ds1)
	g.AddSource(ds2)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result.Source)
}

func TestHintStrategy_NonexistentHint(t *testing.T) {
	s := NewHintStrategy(nil)
	g := &Group{Name: "test", StorageType: StorageDatabase}
	primary := &DataSource{Name: "primary"}
	primary.Healthy.Store(true)
	g.AddSource(primary)

	result, err := s.Resolve(context.Background(), g, NewRouteContext().WithRouteHint("nonexistent"))
	assert.NoError(t, err)
	assert.Equal(t, primary, result.Source)
}

func TestFailoverStrategy_AllSourcesUnhealthy(t *testing.T) {
	s := NewFailoverStrategy(3, &PrimaryStrategy{})
	g := &Group{Name: "test", StorageType: StorageDatabase}

	_, err := s.Resolve(context.Background(), g, nil)
	assert.Error(t, err)
}

func TestWeightedStrategy_Fallback(t *testing.T) {
	original := weightedRandInt
	defer func() { weightedRandInt = original }()

	weightedRandInt = func(min, max int) int {
		return max
	}

	s := NewWeightedStrategy()
	g := &Group{Name: "test", StorageType: StorageDatabase}
	ds1 := &DataSource{Name: "ds1", Weight: 1}
	ds1.Healthy.Store(true)
	ds2 := &DataSource{Name: "ds2", Weight: 1}
	ds2.Healthy.Store(true)
	g.AddSource(ds1)
	g.AddSource(ds2)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result.Source)
}

func TestFailoverStrategy_FinalFallback(t *testing.T) {
	s := NewFailoverStrategy(3, &failingStrategy{})
	g := &Group{Name: "test", StorageType: StorageDatabase}

	ds1 := &DataSource{Name: "ds1", Weight: 1}
	ds1.Healthy.Store(true)
	ds2 := &DataSource{Name: "ds2", Weight: 1}
	ds2.Healthy.Store(true)
	g.AddSource(ds1)
	g.AddSource(ds2)

	result, err := s.Resolve(context.Background(), g, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result.Source)
}
