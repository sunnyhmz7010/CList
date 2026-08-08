package storage

import (
	"context"
	"sync"
)

type Registry struct {
	mu       sync.RWMutex
	backends map[string]Backend
	factory  BackendFactory
}

type BackendFactory func(kind string, config map[string]string) (Backend, error)

func NewRegistry() *Registry {
	return &Registry{backends: make(map[string]Backend)}
}

func (r *Registry) Register(id string, backend Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends[id] = backend
}

func (r *Registry) SetFactory(factory BackendFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factory = factory
}

func (r *Registry) Build(kind string, config map[string]string) (Backend, error) {
	r.mu.RLock()
	factory := r.factory
	r.mu.RUnlock()
	if factory == nil {
		return nil, ErrBackendNotFound
	}
	return factory(kind, config)
}

func (r *Registry) RegisterConfig(id, kind string, config map[string]string) error {
	backend, err := r.Build(kind, config)
	if err != nil {
		return err
	}
	r.Register(id, backend)
	return nil
}

func (r *Registry) Resolve(id string) (Backend, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backend, ok := r.backends[id]
	if !ok {
		return nil, ErrBackendNotFound
	}
	return backend, nil
}

func (r *Registry) HealthCheck(ctx context.Context) error {
	r.mu.RLock()
	backends := make([]Backend, 0, len(r.backends))
	for _, backend := range r.backends {
		backends = append(backends, backend)
	}
	r.mu.RUnlock()
	for _, backend := range backends {
		if err := backend.HealthCheck(ctx); err != nil {
			return err
		}
	}
	return nil
}
