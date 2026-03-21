package collectors

import (
	"fmt"
	"sync"

	"github.com/halyph/page-analyzer/internal/domain"
)

// Registry manages collector factories
type Registry struct {
	factories map[string]domain.CollectorFactory
	mu        sync.RWMutex
}

// NewRegistry creates a new collector registry
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]domain.CollectorFactory),
	}
}

// Register adds a collector factory to the registry
func (r *Registry) Register(name string, factory domain.CollectorFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Create instantiates a collector by name
func (r *Registry) Create(name string, config domain.CollectorConfig) (domain.Collector, error) {
	r.mu.RLock()
	factory, ok := r.factories[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown collector: %s", name)
	}

	return factory.Create(config)
}

// List returns metadata for all registered collectors
func (r *Registry) List() []domain.CollectorMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata := make([]domain.CollectorMetadata, 0, len(r.factories))
	for _, factory := range r.factories {
		metadata = append(metadata, factory.Metadata())
	}
	return metadata
}

// Has checks if a collector is registered
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[name]
	return ok
}

// DefaultRegistry is the global registry used by the application
var DefaultRegistry = NewRegistry()

// Register is a convenience function to register with the default registry
func Register(name string, factory domain.CollectorFactory) {
	DefaultRegistry.Register(name, factory)
}

// init registers all core collectors
func init() {
	Register("htmlversion", &HTMLVersionFactory{})
	Register("title", &TitleFactory{})
	Register("headings", &HeadingsFactory{})
	Register("links", &LinksFactory{})
	Register("loginform", &LoginFormFactory{})
}
