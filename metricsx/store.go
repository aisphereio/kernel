package metricsx

import (
	"sync"

	"go.opentelemetry.io/otel/metric"
)

// store holds registered metric instruments by name. Thread-safe.
type store struct {
	mu            sync.RWMutex
	counter       map[string]metric.Int64Counter
	upDownCounter map[string]metric.Float64UpDownCounter
	histogram     map[string]metric.Float64Histogram
	gauge         map[string]*float64Gauge
}

func newStore() *store {
	return &store{
		counter:       make(map[string]metric.Int64Counter),
		upDownCounter: make(map[string]metric.Float64UpDownCounter),
		histogram:     make(map[string]metric.Float64Histogram),
		gauge:         make(map[string]*float64Gauge),
	}
}

func (s *store) hasCounter(name string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.counter[name]
	return ok
}

func (s *store) hasUpDownCounter(name string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.upDownCounter[name]
	return ok
}

func (s *store) hasHistogram(name string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.histogram[name]
	return ok
}

func (s *store) hasGauge(name string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.gauge[name]
	return ok
}

func (s *store) getCounter(name string) (metric.Int64Counter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.counter[name]
	if !ok {
		return nil, &ErrNotRegistered{Name: name}
	}
	return m, nil
}

func (s *store) getUpDownCounter(name string) (metric.Float64UpDownCounter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.upDownCounter[name]
	if !ok {
		return nil, &ErrNotRegistered{Name: name}
	}
	return m, nil
}

func (s *store) getHistogram(name string) (metric.Float64Histogram, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.histogram[name]
	if !ok {
		return nil, &ErrNotRegistered{Name: name}
	}
	return m, nil
}

func (s *store) getGauge(name string) (*float64Gauge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.gauge[name]
	if !ok {
		return nil, &ErrNotRegistered{Name: name}
	}
	return m, nil
}

func (s *store) setCounter(name string, m metric.Int64Counter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.counter[name]; exists {
		return &ErrAlreadyRegistered{Name: name}
	}
	s.counter[name] = m
	return nil
}

func (s *store) setUpDownCounter(name string, m metric.Float64UpDownCounter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.upDownCounter[name]; exists {
		return &ErrAlreadyRegistered{Name: name}
	}
	s.upDownCounter[name] = m
	return nil
}

func (s *store) setHistogram(name string, m metric.Float64Histogram) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.histogram[name]; exists {
		return &ErrAlreadyRegistered{Name: name}
	}
	s.histogram[name] = m
	return nil
}

func (s *store) setGauge(name string, m *float64Gauge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.gauge[name]; exists {
		return &ErrAlreadyRegistered{Name: name}
	}
	s.gauge[name] = m
	return nil
}
