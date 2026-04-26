/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @FilePath: \go-phantom\router_group_test.go
 * @Description: 测试路由组
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

func TestRouter_SetAndGetStrategy(t *testing.T) {
	r := NewRegistry(nil)
	router := NewRouter(r, nil)

	strategy := &PrimaryStrategy{}
	router.SetStrategy("test_group", strategy)

	found := router.GetStrategy("test_group")
	assert.Equal(t, strategy, found)

	notFound := router.GetStrategy("nonexistent")
	assert.Nil(t, notFound)
}

func TestRouter_Route_GroupNotFound(t *testing.T) {
	r := NewRegistry(nil)
	router := NewRouter(r, nil)

	_, err := router.Route(context.Background(), "nonexistent", nil)
	assert.Error(t, err)
}

func TestRouter_Route_DefaultStrategy(t *testing.T) {
	r := NewRegistry(nil)
	router := NewRouter(r, nil)

	g := NewGroup("test", StorageDatabase)
	primary := &DataSource{Name: "primary", StorageType: StorageDatabase}
	primary.Healthy.Store(true)
	g.AddSource(primary)
	r.RegisterGroup(g)

	source, err := router.Route(context.Background(), "test", nil)
	assert.NoError(t, err)
	assert.Equal(t, primary, source)
}

func TestRouter_Route_NilRouteResult(t *testing.T) {
	r := NewRegistry(nil)
	router := NewRouter(r, nil)

	g := NewGroup("test", StorageDatabase)
	ds := &DataSource{Name: "ds", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	r.RegisterGroup(g)

	router.SetStrategy("test", &nilResultStrategy{})

	_, err := router.Route(context.Background(), "test", nil)
	assert.Error(t, err)
}

type nilResultStrategy struct{}

func (s *nilResultStrategy) Resolve(_ context.Context, _ *Group, _ *RouteContext) (*RouteResult, error) {
	return &RouteResult{Source: nil, GroupName: "test"}, nil
}
func (s *nilResultStrategy) Name() string { return "nil_result" }

func TestRouter_Route_NilSource(t *testing.T) {
	r := NewRegistry(nil)
	router := NewRouter(r, nil)

	g := NewGroup("test", StorageDatabase)
	ds := &DataSource{Name: "ds", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	r.RegisterGroup(g)

	router.SetStrategy("test", &nilSourceStrategy{})

	_, err := router.Route(context.Background(), "test", nil)
	assert.Error(t, err)

	var groupErr *GroupError
	assert.ErrorAs(t, err, &groupErr)
	assert.Equal(t, "test", groupErr.Group)
}

type nilSourceStrategy struct{}

func (s *nilSourceStrategy) Resolve(_ context.Context, _ *Group, _ *RouteContext) (*RouteResult, error) {
	return &RouteResult{Source: nil, GroupName: "test"}, nil
}
func (s *nilSourceStrategy) Name() string { return "nil_source" }

func TestRouter_Route_StrategyError(t *testing.T) {
	r := NewRegistry(nil)
	router := NewRouter(r, nil)

	g := NewGroup("test", StorageDatabase)
	ds := &DataSource{Name: "ds", StorageType: StorageDatabase}
	ds.Healthy.Store(true)
	g.AddSource(ds)
	r.RegisterGroup(g)

	router.SetStrategy("test", &errorStrategy{})

	_, err := router.Route(context.Background(), "test", nil)
	assert.Error(t, err)
	assert.Equal(t, "strategy error", err.Error())
}

type errorStrategy struct{}

func (s *errorStrategy) Resolve(_ context.Context, _ *Group, _ *RouteContext) (*RouteResult, error) {
	return nil, fmt.Errorf("strategy error")
}
func (s *errorStrategy) Name() string { return "error" }
