package lifecycle

import (
	"context"
	"fmt"
	"sync"

	"flux/internal/services"
)

// Manager orchestrates initialization and shutdown of registered services.
type Manager struct {
	mu        sync.Mutex
	services  []services.Service
	started   []services.Service
	isRunning bool
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Register(svc services.Service) {
	if svc == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services = append(m.services, svc)
}

func (m *Manager) Services() []services.Service {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]services.Service, len(m.services))
	copy(out, m.services)
	return out
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isRunning {
		return nil
	}
	m.started = nil
	for _, svc := range m.services {
		if err := svc.Initialize(ctx); err != nil {
			for i := len(m.started) - 1; i >= 0; i-- {
				_ = m.started[i].Shutdown(ctx)
			}
			m.started = nil
			return fmt.Errorf("initialize service %q: %w", svc.Name(), err)
		}
		m.started = append(m.started, svc)
	}
	m.isRunning = true
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.isRunning {
		return nil
	}
	var stopErr error
	for i := len(m.started) - 1; i >= 0; i-- {
		svc := m.started[i]
		if err := svc.Shutdown(ctx); err != nil && stopErr == nil {
			stopErr = fmt.Errorf("shutdown service %q: %w", svc.Name(), err)
		}
	}
	m.started = nil
	m.isRunning = false
	return stopErr
}
