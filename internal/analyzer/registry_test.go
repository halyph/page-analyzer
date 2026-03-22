package analyzer

import (
	"testing"

	"github.com/halyph/page-analyzer/internal/analyzer/collectors"
	"github.com/halyph/page-analyzer/internal/config"
	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_AllCollectorsRegistered(t *testing.T) {
	for _, name := range config.DefaultCollectors {
		assert.True(t, collectors.DefaultRegistry.Has(name), "Collector %s not registered", name)
	}
}

func TestRegistry_CreateLoginForm(t *testing.T) {
	cfg := domain.CollectorConfig{
		BaseURL:  "https://example.com",
		MaxItems: 1000,
	}

	collector, err := collectors.DefaultRegistry.Create(config.CollectorLoginForm, cfg)
	require.NoError(t, err)
	require.NotNil(t, collector)

	_, ok := collector.(*collectors.LoginFormCollector)
	assert.True(t, ok, "collector type = %T, want *LoginFormCollector", collector)
}
