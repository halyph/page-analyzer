package analyzer

import (
	"testing"

	"github.com/halyph/page-analyzer/internal/analyzer/collectors"
	"github.com/halyph/page-analyzer/internal/domain"
)

func TestRegistry_AllCollectorsRegistered(t *testing.T) {
	required := []string{"htmlversion", "title", "headings", "links", "loginform"}

	for _, name := range required {
		if !collectors.DefaultRegistry.Has(name) {
			t.Errorf("Collector %s not registered", name)
		}
	}
}

func TestRegistry_CreateLoginForm(t *testing.T) {
	config := domain.CollectorConfig{
		BaseURL:  "https://example.com",
		MaxItems: 1000,
	}

	collector, err := collectors.DefaultRegistry.Create("loginform", config)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if collector == nil {
		t.Fatal("collector is nil")
	}

	_, ok := collector.(*collectors.LoginFormCollector)
	if !ok {
		t.Errorf("collector type = %T, want *LoginFormCollector", collector)
	}
}
