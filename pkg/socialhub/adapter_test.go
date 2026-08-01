package socialhub

import (
	"context"
	"testing"
)

type testAdapter struct {
	name        string
	initialized bool
}

func (a *testAdapter) Name() string       { return a.name }
func (a *testAdapter) Metadata() Metadata { return Metadata{Name: a.name} }
func (a *testAdapter) Init(_ context.Context, _ AdapterConfig, _ ...Option) error {
	a.initialized = true
	return nil
}
func (a *testAdapter) Client(context.Context, AccountID, ...Option) (Client, error) {
	return nil, ErrUnsupported
}
func (a *testAdapter) Close() error { return nil }

func TestRegistryOpen(t *testing.T) {
	name := "test/registry-open"
	Register(name, func() Adapter { return &testAdapter{name: name} })

	adapter, err := Open(context.Background(), name, AdapterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !adapter.(*testAdapter).initialized {
		t.Fatal("adapter was not initialized")
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	name := "test/registry-duplicate"
	Register(name, func() Adapter { return &testAdapter{name: name} })
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate registration should panic")
		}
	}()
	Register(name, func() Adapter { return &testAdapter{name: name} })
}
