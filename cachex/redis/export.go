package redis

import "github.com/aisphereio/kernel/cachex"

// NewDirectClient creates a cachex.Cache from the given Config without
// going through the driver registry. This is useful for tests where the
// Redis address is dynamic (e.g., miniredis).
//
// In production code, use cachex.New(cfg) instead.
func NewDirectClient(cfg cachex.Config) (cachex.Cache, error) {
	return open(cfg)
}
