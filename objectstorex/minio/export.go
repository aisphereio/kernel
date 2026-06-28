package minio

import "github.com/aisphereio/kernel/objectstorex"

// NewDirectClient creates an objectstorex.Client from the given Config
// without going through the driver registry. Useful for tests.
//
// In production, use objectstorex.New(cfg) instead.
func NewDirectClient(cfg objectstorex.Config) (objectstorex.Client, error) {
	return open(cfg)
}
