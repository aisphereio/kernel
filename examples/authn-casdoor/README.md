# Authn Casdoor Example

This example tests only Kernel `authn` against a real Casdoor instance. It does **not** call the Casdoor SDK from example/business code. The only provider-specific line is the bootstrapping constructor:

```go
client, err := casdoor.New(cfg)
```

After that, the demo uses Kernel contracts only:

```go
authn.LoginService
authn.TokenService
authn.Authenticator
authn.UserDirectory
authn.GroupAdmin
```

It covers:

- config loading from YAML through `configx`
- log output through `logx`
- canonical error wrapping/inspection through `errorx`
- login URL generation through `authn.LoginService.BuildLoginURL`
- reusable HTTP callback handling through `authn/callback.Handler`
- OAuth callback code exchange through `authn.LoginService.HandleCallback`
- access token verification through `authn.TokenService.VerifyToken`
- bearer-token authentication through `authn.Authenticator.Authenticate`
- refresh token flow, when Casdoor returns a refresh token
- expected negative authn checks
- optional read-only user/group checks through Kernel identity interfaces

## 1. Prepare config

Copy the sample:

```powershell
Copy-Item .\examples\authn-casdoor\config.example.yaml .\examples\authn-casdoor\config.local.yaml
```

Then edit `config.local.yaml` with your local Casdoor values. Your current flat config shape is supported:

```yaml
casdoor:
  endpoint: "http://localhost:18000"
  client_id: "aisphere-auth"
  client_secret: "..."
  organization: "aisphere"
  application: "aisphere"
  default_scope: "openid profile email"
  certificate: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

For new configs prefer:

```yaml
casdoor:
  jwt_certificate_file: "./configs/casdoor-app.crt"
```

`jwt_certificate` / `certificate` is the Casdoor application public certificate used to verify JWTs. It is not the HTTPS/TLS certificate and not a private key.

## 2. Automatic callback integration flow

This is the recommended local flow. It starts a local web app using `authn/callback.Handler`, opens your browser automatically, redirects `/login` to Casdoor, receives `code`, exchanges it for tokens, verifies the access token, and logs the normalized `authn.Principal`.

```powershell
go run .\examples\authn-casdoor -config .\examples\authn-casdoor\config.local.yaml -serve
```

By default the example opens:

```text
http://localhost:3000/login
```

The local routes are:

```text
GET /          friendly landing page
GET /login     redirects to Casdoor login
GET /callback  receives Casdoor callback; if code is missing, redirects back to /login
```

After successful callback, the CLI exits automatically. To disable browser opening:

```powershell
go run .\examples\authn-casdoor -config .\examples\authn-casdoor\config.local.yaml -serve -open-browser=false
```

## 3. Show login URL only

```powershell
go run .\examples\authn-casdoor -config .\examples\authn-casdoor\config.local.yaml
```

This only prints `login_url`; it does not start the callback server. For a full browser test, use `-serve`.

## 4. Manual callback code flow

After logging in, copy the `code` query parameter from the callback URL and run:

```powershell
go run .\examples\authn-casdoor -config .\examples\authn-casdoor\config.local.yaml -code "PASTE_CODE_HERE"
```

This still uses `authn.LoginService.HandleCallback`; it just feeds the callback code manually.

## 5. Verify an existing access token

```powershell
go run .\examples\authn-casdoor -config .\examples\authn-casdoor\config.local.yaml -access-token "PASTE_ACCESS_TOKEN_HERE"
```

## 6. Refresh token

```powershell
go run .\examples\authn-casdoor -config .\examples\authn-casdoor\config.local.yaml -refresh-token "PASTE_REFRESH_TOKEN_HERE"
```

## 7. Optional read-only identity checks

These checks call Kernel identity interfaces only. They do not create, update, or delete users/groups.

```powershell
go run .\examples\authn-casdoor `
  -config .\examples\authn-casdoor\config.local.yaml `
  -admin-read `
  -username test4 `
  -list-groups
```

If this fails with a Casdoor permission error, keep authn login tests first and later provide a dedicated admin application under `security.authn.casdoor.admin`.

## Notes

- Do not log full tokens in production. This demo logs only token hints.
- Keep `authz` / SpiceDB tests separate until this authn example passes.
- A production service should store and validate `state` from a server-side session; this demo uses a fixed value only for local integration testing.
