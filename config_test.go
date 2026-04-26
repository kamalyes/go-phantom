/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-01 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-25 00:00:00
 * @FilePath: \go-phantom\config_test.go
 * @Description: 测试配置
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

func TestDefaultPhantomConfig(t *testing.T) {
	config := DefaultPhantomConfig()
	assert.False(t, config.Enabled)
	assert.True(t, config.HealthCheck.Enabled)
	assert.Empty(t, config.Groups)
	assert.Empty(t, config.ShadowRules)
}

func TestConfigDrivenBuilder_Disabled(t *testing.T) {
	config := DefaultPhantomConfig()
	builder := NewConfigDrivenBuilder(config)
	p, err := builder.Build(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestConfigDrivenBuilder_WithGroups(t *testing.T) {
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups: []PhantomGroupConfig{
			{
				Name:        "db_group",
				StorageType: StorageDatabase,
				Strategy:    "primary",
				Sources: []PhantomSourceConfig{
					{Name: "primary_db", Weight: 1},
					{Name: "slave_db", ReadOnly: true, Weight: 1},
				},
			},
		},
		DefaultGroups: map[StorageType]string{
			StorageDatabase: "db_group",
		},
	}

	builder := NewConfigDrivenBuilder(config)
	p, err := builder.Build(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, p, builder.GetPhantom())
}

func TestConfigDrivenBuilder_WithShadowRules(t *testing.T) {
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups: []PhantomGroupConfig{
			{
				Name:        "db_group",
				StorageType: StorageDatabase,
				Sources: []PhantomSourceConfig{
					{Name: "primary_db"},
				},
			},
		},
		ShadowRules: []PhantomShadowConfig{
			{
				GroupName:   "db_group",
				Enabled:     true,
				Logic:       "or",
				ShadowDS:    "shadow_db",
				ShadowTable: "shadow_",
				MatchRules: []PhantomMatchRuleConfig{
					{Type: ShadowMatchTag, Values: []string{"test"}},
				},
			},
		},
	}

	builder := NewConfigDrivenBuilder(config)
	p, err := builder.Build(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestConfigDrivenBuilder_WithSourceFactory(t *testing.T) {
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups: []PhantomGroupConfig{
			{
				Name:        "db_group",
				StorageType: StorageDatabase,
				Sources: []PhantomSourceConfig{
					{Name: "primary_db", DSN: "mysql://localhost:3306/db"},
				},
			},
		},
	}

	factoryCalled := false
	builder := NewConfigDrivenBuilder(config).WithSourceFactory(func(ctx context.Context, cfg PhantomSourceConfig) (interface{}, error) {
		factoryCalled = true
		assert.Equal(t, "mysql://localhost:3306/db", cfg.DSN)
		return "mock_instance", nil
	})

	p, err := builder.Build(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.True(t, factoryCalled)
}

func TestConfigDrivenBuilder_SourceFactoryError(t *testing.T) {
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups: []PhantomGroupConfig{
			{
				Name:        "db_group",
				StorageType: StorageDatabase,
				Sources: []PhantomSourceConfig{
					{Name: "primary_db"},
				},
			},
		},
	}

	builder := NewConfigDrivenBuilder(config).WithSourceFactory(func(ctx context.Context, cfg PhantomSourceConfig) (interface{}, error) {
		return nil, fmt.Errorf("factory error")
	})

	_, err := builder.Build(context.Background())
	assert.Error(t, err)
}

func TestConfigDrivenBuilder_DuplicateGroup(t *testing.T) {
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups: []PhantomGroupConfig{
			{Name: "db_group", StorageType: StorageDatabase, Sources: []PhantomSourceConfig{{Name: "ds1"}}},
			{Name: "db_group", StorageType: StorageDatabase, Sources: []PhantomSourceConfig{{Name: "ds2"}}},
		},
	}

	builder := NewConfigDrivenBuilder(config)
	_, err := builder.Build(context.Background())
	assert.Error(t, err)
}

func TestConfigDrivenBuilder_WithLogger(t *testing.T) {
	log := logger.NewLogger()
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups: []PhantomGroupConfig{
			{Name: "db_group", StorageType: StorageDatabase, Sources: []PhantomSourceConfig{{Name: "ds1"}}},
		},
	}

	builder := NewConfigDrivenBuilder(config).WithLogger(log)
	p, err := builder.Build(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestConfigDrivenBuilder_WithNonILogger(t *testing.T) {
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups: []PhantomGroupConfig{
			{Name: "db_group", StorageType: StorageDatabase, Sources: []PhantomSourceConfig{{Name: "ds1"}}},
		},
	}

	builder := NewConfigDrivenBuilder(config).WithLogger("not_a_logger")
	p, err := builder.Build(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestConfigDrivenBuilder_DefaultHealthCheckTimeout(t *testing.T) {
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled: true,
		},
		Groups: []PhantomGroupConfig{
			{Name: "db_group", StorageType: StorageDatabase, Sources: []PhantomSourceConfig{{Name: "ds1"}}},
		},
	}

	builder := NewConfigDrivenBuilder(config)
	p, err := builder.Build(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestConfigDrivenBuilder_WithStrategy(t *testing.T) {
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups: []PhantomGroupConfig{
			{
				Name:        "db_group",
				StorageType: StorageDatabase,
				Strategy:    "readonly",
				Sources: []PhantomSourceConfig{
					{Name: "primary_db"},
					{Name: "slave_db", ReadOnly: true},
				},
			},
		},
	}

	builder := NewConfigDrivenBuilder(config)
	p, err := builder.Build(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestConfigDrivenBuilder_WithTenantSource(t *testing.T) {
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups: []PhantomGroupConfig{
			{
				Name:        "db_group",
				StorageType: StorageDatabase,
				Sources: []PhantomSourceConfig{
					{Name: "primary_db"},
					{Name: "tenant_db", TenantID: "t1"},
				},
			},
		},
	}

	builder := NewConfigDrivenBuilder(config)
	p, err := builder.Build(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestConfigDrivenBuilder_WithShadowSource(t *testing.T) {
	config := &PhantomConfig{
		Enabled: true,
		HealthCheck: PhantomHealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  3 * time.Second,
		},
		Groups: []PhantomGroupConfig{
			{
				Name:        "db_group",
				StorageType: StorageDatabase,
				Sources: []PhantomSourceConfig{
					{Name: "primary_db"},
					{Name: "shadow_db", Shadow: true},
				},
			},
		},
	}

	builder := NewConfigDrivenBuilder(config)
	p, err := builder.Build(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestConfigDrivenBuilder_GetPhantom_NoBuild(t *testing.T) {
	config := DefaultPhantomConfig()
	builder := NewConfigDrivenBuilder(config)
	assert.Nil(t, builder.GetPhantom())
}
