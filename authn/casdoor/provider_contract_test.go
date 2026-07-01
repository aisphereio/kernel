package casdoor

import "github.com/aisphereio/kernel/authn"

var _ authn.ManagementProvider = (*Client)(nil)
