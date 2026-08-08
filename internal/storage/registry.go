package storage

import (
	"context"
	"sync"
)

type Registry struct {
	mu       sync.RWMutex
	backends map[string]Backend
}

func NewRegistry() *Registry {
	return &Registry{backends: make(map[string]Backend)}
}

func (r *Registry) Register(id string, backend Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends[id] = backend
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
