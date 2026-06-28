package data

import (
	"context"
	"errors"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authn/casdoor"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/authz/spicedb"
	"github.com/aisphereio/kernel/cachex"
	_ "github.com/aisphereio/kernel/cachex/redis"
	"github.com/aisphereio/kernel/dbx"
	_ "github.com/aisphereio/kernel/dbx/postgres"
	"github.com/aisphereio/kernel/objectstorex"
	_ "github.com/aisphereio/kernel/objectstorex/minio"

	"github.com/aisphereio/kernel-layout/internal/conf"
)

type Resources struct {
	DB          dbx.DB
	Cache       cachex.Cache
	ObjectStore objectstorex.Client
	Audit       auditx.Recorder
	Authn       authn.Authenticator
	Authz       authz.Authorizer
	Access      accessx.Guard

	closers []func() error
}

type Data struct {
	Resources *Resources
}

func NewResources(ctx context.Context, cfg conf.Bootstrap) (*Resources, func(), error) {
	r := &Resources{
		Audit: auditx.NewMemoryStore(),
		Authz: authz.DenyAll(),
	}
	if !cfg.Audit.Enabled {
		r.Audit = auditx.Noop()
	}
	if cfg.Security.Authz.DevAllowAll {
		r.Authz = authz.AllowAllForDevOnly()
	}

	if cfg.Data.Database.Enabled {
		db, err := dbx.New(cfg.Data.Database.Config)
		if err != nil {
			return nil, nil, err
		}
		r.DB = db
		r.closers = append(r.closers, db.Close)
	}
	if cfg.Data.Cache.Enabled {
		cache, err := cachex.New(cfg.Data.Cache.Config)
		if err != nil {
			r.Close()
			return nil, nil, err
		}
		r.Cache = cache
		r.closers = append(r.closers, cache.Close)
	}
	if cfg.Data.ObjectStore.Enabled {
		store, err := objectstorex.New(cfg.Data.ObjectStore.Config)
		if err != nil {
			r.Close()
			return nil, nil, err
		}
		r.ObjectStore = store
		r.closers = append(r.closers, store.Close)
	}
	if cfg.Security.Authn.Enabled {
		authenticator, err := newAuthenticator(cfg.Security.Authn)
		if err != nil {
			r.Close()
			return nil, nil, err
		}
		r.Authn = authenticator
	}
	if cfg.Security.Authz.Enabled {
		authorizer, closeFn, err := newAuthorizer(cfg.Security.Authz)
		if err != nil {
			r.Close()
			return nil, nil, err
		}
		r.Authz = authorizer
		if closeFn != nil {
			r.closers = append(r.closers, closeFn)
		}
	}

	r.Access = accessx.New(r.Authn, r.Authz, r.Audit)
	return r, func() { _ = r.Close() }, pingEnabled(ctx, r)
}

func NewData(resources *Resources) *Data {
	return &Data{Resources: resources}
}

func newAuthenticator(cfg conf.AuthnConfig) (authn.Authenticator, error) {
	switch cfg.Provider {
	case "", "casdoor":
		return casdoor.New(cfg.Casdoor)
	default:
		return nil, errors.New("unsupported authn provider: " + cfg.Provider)
	}
}

func newAuthorizer(cfg conf.AuthzConfig) (authz.Authorizer, func() error, error) {
	switch cfg.Provider {
	case "", "spicedb":
		client, err := spicedb.New(cfg.SpiceDB)
		if err != nil {
			return nil, nil, err
		}
		return client, client.Close, nil
	default:
		return nil, nil, errors.New("unsupported authz provider: " + cfg.Provider)
	}
}

func pingEnabled(ctx context.Context, r *Resources) error {
	if r.DB != nil {
		if err := r.DB.PingContext(ctx); err != nil {
			return err
		}
	}
	if r.Cache != nil {
		if err := r.Cache.Ping(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resources) Close() error {
	var out error
	for i := len(r.closers) - 1; i >= 0; i-- {
		if err := r.closers[i](); err != nil && out == nil {
			out = err
		}
	}
	return out
}
