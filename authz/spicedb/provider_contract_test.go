package spicedb

import "github.com/aisphereio/kernel/authz"

var _ authz.AdminProvider = (*Client)(nil)
