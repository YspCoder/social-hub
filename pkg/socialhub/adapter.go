package socialhub

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Metadata documents the API product implemented by an adapter.
type Metadata struct {
	Name       string
	Product    string
	APIVersion string
	SDKVersion string
	DocURL     string
	VerifiedAt time.Time
}

// Adapter owns platform-wide dependencies and creates account clients.
type Adapter interface {
	Name() string
	Metadata() Metadata
	Init(context.Context, AdapterConfig, ...Option) error
	Client(context.Context, AccountID, ...Option) (Client, error)
	Close() error
}

// Factory creates a new uninitialized adapter.
type Factory func() Adapter

var adapterRegistry = struct {
	sync.RWMutex
	factories map[string]Factory
}{factories: make(map[string]Factory)}

// Register adds an adapter factory. It panics for invalid or duplicate
// registrations because those are build-time programming errors.
func Register(name string, factory Factory) {
	if name == "" || factory == nil {
		panic("socialhub: invalid adapter registration")
	}
	adapterRegistry.Lock()
	defer adapterRegistry.Unlock()
	if _, exists := adapterRegistry.factories[name]; exists {
		panic("socialhub: adapter already registered: " + name)
	}
	adapterRegistry.factories[name] = factory
}

// Open initializes a registered adapter.
func Open(ctx context.Context, name string, config AdapterConfig, options ...Option) (Adapter, error) {
	adapterRegistry.RLock()
	factory := adapterRegistry.factories[name]
	adapterRegistry.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("%w: %s", ErrAdapterNotFound, name)
	}
	adapter := factory()
	if err := adapter.Init(ctx, config, options...); err != nil {
		return nil, err
	}
	return adapter, nil
}

// Adapters returns registered adapter names in stable order.
func Adapters() []string {
	adapterRegistry.RLock()
	defer adapterRegistry.RUnlock()
	names := make([]string, 0, len(adapterRegistry.factories))
	for name := range adapterRegistry.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
