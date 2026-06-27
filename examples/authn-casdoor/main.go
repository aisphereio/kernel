// Command authn-casdoor exercises Kernel authn against a real local Casdoor.
//
// The example intentionally uses only Kernel contracts after bootstrapping the
// provider adapter. Application code should not call Casdoor SDK methods.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authn/callback"
	casdoorauthn "github.com/aisphereio/kernel/authn/casdoor"
	"github.com/aisphereio/kernel/configx"
	"github.com/aisphereio/kernel/configx/file"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

type rootConfig struct {
	Log      logx.Config         `json:"log" yaml:"log"`
	Casdoor  legacyCasdoorConfig `json:"casdoor" yaml:"casdoor"`
	Security struct {
		Authn struct {
			Casdoor casdoorauthn.Config `json:"casdoor" yaml:"casdoor"`
		} `json:"authn" yaml:"authn"`
	} `json:"security" yaml:"security"`
	Example exampleConfig `json:"authn_example" yaml:"authn_example"`
}

// legacyCasdoorConfig accepts the older flat config shape used by the current
// dev environment. New kernel configs should prefer security.authn.casdoor.
type legacyCasdoorConfig struct {
	Endpoint     string `json:"endpoint" yaml:"endpoint"`
	ClientID     string `json:"client_id" yaml:"client_id"`
	ClientSecret string `json:"client_secret" yaml:"client_secret"`

	Organization string `json:"organization" yaml:"organization"`
	Application  string `json:"application" yaml:"application"`

	JWTCertificate     string `json:"jwt_certificate" yaml:"jwt_certificate"`
	JWTCertificateFile string `json:"jwt_certificate_file" yaml:"jwt_certificate_file"`
	Certificate        string `json:"certificate" yaml:"certificate"`

	HTTPTimeout  string `json:"http_timeout" yaml:"http_timeout"`
	DefaultScope string `json:"default_scope" yaml:"default_scope"`

	// Existing Casdoor/Casbin policy fields are intentionally parsed but not used
	// in authn tests. They belong to authz/Casbin compatibility, not login.
	PolicyEnforcer    string `json:"policy_enforcer" yaml:"policy_enforcer"`
	PolicyAdapterID   string `json:"policy_adapter_id" yaml:"policy_adapter_id"`
	PolicyBindingMode string `json:"policy_binding_mode" yaml:"policy_binding_mode"`
}

type exampleConfig struct {
	RedirectURI          string `json:"redirect_uri" yaml:"redirect_uri"`
	State                string `json:"state" yaml:"state"`
	DefaultScope         string `json:"default_scope" yaml:"default_scope"`
	Username             string `json:"username" yaml:"username"`
	Email                string `json:"email" yaml:"email"`
	ListenAddr           string `json:"listen_addr" yaml:"listen_addr"`
	RunInvalidTokenCheck bool   `json:"run_invalid_token_check" yaml:"run_invalid_token_check"`
	RunAdminRead         bool   `json:"run_admin_read" yaml:"run_admin_read"`
	ListGroups           bool   `json:"list_groups" yaml:"list_groups"`
}

type options struct {
	configPath   string
	redirectURI  string
	state        string
	code         string
	accessToken  string
	refreshToken string
	serve        bool
	adminRead    bool
	listGroups   bool
	username     string
	email        string
	listenAddr   string
	openBrowser  bool
}

type authnSuite struct {
	login         authn.LoginService
	tokens        authn.TokenService
	authenticator authn.Authenticator
	profiles      authn.ProfileService
	users         authn.UserDirectory
	groups        authn.GroupAdmin
}

func main() {
	opts := parseFlags()
	ctx := context.Background()

	cfg, loadErr := loadRootConfig(opts.configPath)
	logger, closeLogger := mustLogger(cfg.Log)
	defer closeLogger()

	logger.Info("authn casdoor example starting",
		logx.String("config_path", opts.configPath),
		logx.Bool("serve", opts.serve),
	)
	if loadErr != nil {
		logFailure(logger, "load authn casdoor example config failed", loadErr)
		os.Exit(1)
	}

	if err := run(ctx, logger, cfg, opts); err != nil {
		logFailure(logger, "authn casdoor example failed", err)
		os.Exit(1)
	}
	logger.Info("authn casdoor example finished")
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.configPath, "config", "examples/authn-casdoor/config.local.yaml", "YAML config path")
	flag.StringVar(&opts.redirectURI, "redirect-uri", "", "OAuth redirect URI. Overrides authn_example.redirect_uri")
	flag.StringVar(&opts.state, "state", "", "OAuth state. Overrides authn_example.state")
	flag.StringVar(&opts.code, "code", "", "Authorization code copied from Casdoor callback")
	flag.StringVar(&opts.accessToken, "access-token", "", "Access token to verify")
	flag.StringVar(&opts.refreshToken, "refresh-token", "", "Refresh token to refresh")
	flag.BoolVar(&opts.serve, "serve", false, "Start a local callback server and exchange code automatically")
	flag.BoolVar(&opts.adminRead, "admin-read", false, "Run non-mutating admin read checks")
	flag.BoolVar(&opts.listGroups, "list-groups", false, "List Casdoor groups during admin-read")
	flag.StringVar(&opts.username, "username", "", "Username for admin read. Overrides authn_example.username")
	flag.StringVar(&opts.email, "email", "", "Email for admin read. Overrides authn_example.email")
	flag.StringVar(&opts.listenAddr, "listen", "", "Callback server listen address. Overrides authn_example.listen_addr")
	flag.BoolVar(&opts.openBrowser, "open-browser", true, "When -serve is set, open the local login page in the default browser")
	flag.Parse()
	return opts
}

func run(ctx context.Context, logger logx.Logger, root rootConfig, opts options) error {
	casdoorCfg, defaultScope, err := effectiveCasdoorConfig(root)
	if err != nil {
		return err
	}
	logger.Info("resolved casdoor authn config", casdoorConfigFields(casdoorCfg, defaultScope)...)
	client, err := casdoorauthn.New(casdoorCfg)
	if err != nil {
		return errorx.Wrap(err, errorx.CodeBadRequest,
			errorx.WithMessage("create casdoor kernel authn adapter failed"),
			errorx.WithMetadata("cause", err.Error()),
		)
	}
	services := authnSuite{
		login:         client,
		tokens:        client,
		authenticator: client,
		profiles:      client,
		users:         client,
		groups:        client,
	}

	ex := mergeExample(root.Example, opts)
	redirectURI := firstNonEmpty(ex.RedirectURI, "http://localhost:3000/callback")
	state := firstNonEmpty(ex.State, "kernel-authn-dev")
	scope := firstNonEmpty(ex.DefaultScope, defaultScope, "openid profile email")
	loginURL, err := services.login.BuildLoginURL(ctx, authn.LoginURLRequest{RedirectURI: redirectURI, State: state, Scope: scope, OrgID: casdoorCfg.OrganizationName, AppID: casdoorCfg.ApplicationName})
	if err != nil {
		return errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("build login url failed"))
	}

	logger.Info("kernel authn adapter ready",
		logx.String("provider", "casdoor"),
		logx.String("endpoint", casdoorCfg.Endpoint),
		logx.String("organization", casdoorCfg.OrganizationName),
		logx.String("application", casdoorCfg.ApplicationName),
		logx.String("client_id", casdoorCfg.ClientID),
		logx.Duration("timeout", casdoorCfg.Timeout),
		logx.Bool("has_jwt_certificate", casdoorCfg.JWTCertificate != "" || casdoorCfg.Certificate != "" || casdoorCfg.JWTCertificateFile != ""),
	)
	logger.Info("open this url to login",
		logx.String("login_url", loginURL.URL),
		logx.String("redirect_uri", loginURL.RedirectURI),
		logx.String("scope", loginURL.Scope),
		logx.String("state", loginURL.State),
	)

	if err := runNegativeChecks(ctx, logger, services, root.Example.RunInvalidTokenCheck); err != nil {
		return err
	}
	if opts.serve {
		return serveCallback(ctx, logger, services, ex.ListenAddr, redirectURI, state, loginURL.URL, opts.openBrowser, ex, casdoorCfg.OrganizationName, casdoorCfg.ApplicationName)
	}
	if opts.code != "" {
		if _, _, err := runCodeFlow(ctx, logger, services, opts.code, state, redirectURI, casdoorCfg.OrganizationName, casdoorCfg.ApplicationName, "flag-code"); err != nil {
			return err
		}
	}
	if opts.accessToken != "" {
		principal, err := runTokenFlow(ctx, logger, services, opts.accessToken, casdoorCfg.OrganizationName, casdoorCfg.ApplicationName, "flag-token")
		if err != nil {
			return err
		}
		if err := runProfileFlow(ctx, logger, services, authn.IdentityProfileRequest{Principal: principal, OrgID: casdoorCfg.OrganizationName, AppID: casdoorCfg.ApplicationName, AllowPartial: true}); err != nil {
			return err
		}
	}
	if opts.refreshToken != "" {
		if err := runRefreshFlow(ctx, logger, services, opts.refreshToken, casdoorCfg.OrganizationName, casdoorCfg.ApplicationName); err != nil {
			return err
		}
	}
	if ex.RunAdminRead {
		if err := runAdminRead(ctx, logger, services, ex, casdoorCfg.OrganizationName); err != nil {
			return err
		}
	}
	return nil
}

func runCodeFlow(ctx context.Context, logger logx.Logger, services authnSuite, code, state, redirectURI, orgID, appID, source string) (authn.TokenSet, authn.Principal, error) {
	logger.Info("handling oauth callback through kernel authn.LoginService",
		logx.String("source", source),
		logx.String("code_hint", tokenHint(code)),
		logx.String("state", state),
		logx.String("redirect_uri", redirectURI),
	)
	result, err := services.login.HandleCallback(ctx, authn.CallbackRequest{Code: code, State: state, ExpectedState: state, RedirectURI: redirectURI, OrgID: orgID, AppID: appID})
	if err != nil {
		return authn.TokenSet{}, authn.Principal{}, errorx.Wrap(err, errorx.CodeUnauthorized, errorx.WithMessage("kernel authn callback failed"))
	}
	logTokenSet(logger, "callback_token_success", result.Tokens)
	logPrincipal(logger, "principal_from_callback", result.Principal)

	if err := runPostLoginChecks(ctx, logger, services, result.Tokens, orgID, appID); err != nil {
		return result.Tokens, result.Principal, err
	}
	return result.Tokens, result.Principal, nil
}

func runPostLoginChecks(ctx context.Context, logger logx.Logger, services authnSuite, tokens authn.TokenSet, orgID, appID string) error {
	verified, err := services.tokens.VerifyToken(ctx, authn.VerifyTokenRequest{Token: tokens.AccessToken, TokenType: "access_token", OrgID: orgID, AppID: appID})
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnauthorized, errorx.WithMessage("verify exchanged access token failed"))
	}
	logPrincipal(logger, "principal_from_verify", verified)

	authenticated, err := services.authenticator.Authenticate(ctx, authn.Credential{Scheme: authn.CredentialBearer, Token: tokens.AccessToken})
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnauthorized, errorx.WithMessage("authenticate bearer token failed"))
	}
	logPrincipal(logger, "principal_from_authenticate", authenticated)

	if err := runProfileFlow(ctx, logger, services, authn.IdentityProfileRequest{Principal: verified, OrgID: orgID, AppID: appID, AllowPartial: true}); err != nil {
		return err
	}

	if tokens.RefreshToken != "" {
		if err := runRefreshFlow(ctx, logger, services, tokens.RefreshToken, orgID, appID); err != nil {
			logger.Warn("refresh token check failed", logx.Err(err))
		}
	}
	return nil
}

func runTokenFlow(ctx context.Context, logger logx.Logger, services authnSuite, accessToken, orgID, appID, source string) (authn.Principal, error) {
	logger.Info("verifying access token through kernel authn.TokenService",
		logx.String("source", source),
		logx.String("access_token_hint", tokenHint(accessToken)),
	)
	principal, err := services.tokens.VerifyToken(ctx, authn.VerifyTokenRequest{Token: accessToken, TokenType: "access_token", OrgID: orgID, AppID: appID})
	if err != nil {
		return authn.Principal{}, errorx.Wrap(err, errorx.CodeUnauthorized, errorx.WithMessage("kernel authn token verify failed"))
	}
	logPrincipal(logger, "principal_from_access_token", principal)
	return principal, nil
}

func runProfileFlow(ctx context.Context, logger logx.Logger, services authnSuite, req authn.IdentityProfileRequest) error {
	logger.Info("loading post-login identity profile through kernel authn.ProfileService",
		logx.String("org", firstNonEmpty(req.OrgID, req.Principal.OrgID)),
		logx.String("app", firstNonEmpty(req.AppID, req.Principal.AppID)),
		logx.String("subject_id", req.Principal.SubjectID),
		logx.String("username", req.Principal.Username),
		logx.Bool("allow_partial", req.AllowPartial),
	)
	profile, err := services.profiles.GetIdentityProfile(ctx, req)
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("kernel authn identity profile load failed"))
	}
	logIdentityProfile(logger, "identity_profile_loaded", profile)
	return nil
}

func runRefreshFlow(ctx context.Context, logger logx.Logger, services authnSuite, refreshToken, orgID, appID string) error {
	logger.Info("refreshing token through kernel authn.TokenService", logx.String("refresh_token_hint", tokenHint(refreshToken)))
	tokens, err := services.tokens.RefreshToken(ctx, authn.RefreshTokenRequest{RefreshToken: refreshToken, OrgID: orgID, AppID: appID})
	if err != nil {
		return errorx.Wrap(err, errorx.CodeUnauthorized, errorx.WithMessage("kernel authn refresh token failed"))
	}
	logTokenSet(logger, "refresh_success", tokens)
	return nil
}

func runNegativeChecks(ctx context.Context, logger logx.Logger, services authnSuite, enabled bool) error {
	if !enabled {
		return nil
	}
	logger.Info("running expected negative checks through kernel authn contracts")
	_, err := services.tokens.VerifyToken(ctx, authn.VerifyTokenRequest{Token: "not-a-valid-jwt"})
	if err == nil {
		return errorx.New(errorx.CodeInternal, errorx.WithMessage("invalid token unexpectedly passed verification"))
	}
	logger.Info("invalid token rejected as expected", logx.Err(err), logx.String("error_code", errorx.CodeOf(err).String()))
	_, err = services.authenticator.Authenticate(ctx, authn.Credential{Scheme: "unsupported", Token: "x"})
	if err == nil {
		return errorx.New(errorx.CodeInternal, errorx.WithMessage("unsupported credential unexpectedly passed authentication"))
	}
	logger.Info("unsupported credential rejected as expected", logx.Err(err), logx.String("error_code", errorx.CodeOf(err).String()))
	return nil
}

func runAdminRead(ctx context.Context, logger logx.Logger, services authnSuite, ex exampleConfig, orgID string) error {
	logger.Info("running non-mutating identity read checks through kernel authn interfaces",
		logx.String("org", orgID),
		logx.String("username", ex.Username),
		logx.String("email", ex.Email),
		logx.Bool("list_groups", ex.ListGroups),
	)
	if ex.Username != "" {
		user, err := services.users.GetUser(ctx, orgID, ex.Username)
		if err != nil {
			logger.Warn("get user failed", logx.Err(err), logx.String("username", ex.Username))
		} else {
			logUser(logger, "get_user_success", user)
		}
	}
	if ex.Email != "" {
		users, err := services.users.FindUsers(ctx, authn.UserFilter{OrgID: orgID, Email: ex.Email, Limit: 10})
		if err != nil {
			logger.Warn("find users by email failed", logx.Err(err), logx.String("email", ex.Email))
		} else {
			logger.Info("find users by email success", logx.String("email", ex.Email), logx.Int("count", len(users)))
			for _, user := range users {
				logUser(logger, "found_user", user)
			}
		}
	}
	if ex.ListGroups {
		groups, err := services.groups.ListGroups(ctx, authn.GroupFilter{OrgID: orgID})
		if err != nil {
			logger.Warn("list groups failed", logx.Err(err))
		} else {
			logger.Info("list groups success", logx.Int("count", len(groups)))
			for _, group := range groups {
				logger.Info("group",
					logx.String("id", group.ID),
					logx.String("name", group.Name),
					logx.String("display_name", group.DisplayName),
					logx.String("org", group.OrgID),
					logx.String("parent", group.ParentID),
					logx.String("type", group.Type),
					logx.Int("users_count", len(group.Users)),
				)
			}
		}
	}
	return nil
}

func serveCallback(ctx context.Context, logger logx.Logger, services authnSuite, listenAddr, redirectURI, state, loginURL string, openBrowser bool, ex exampleConfig, orgID, appID string) error {
	if listenAddr == "" {
		listenAddr = ":3000"
	}
	done := make(chan error, 1)
	finish := func(err error) {
		select {
		case done <- err:
		default:
		}
	}
	h := &callback.Handler{
		Login:         services.login,
		Logger:        logger,
		LoginURL:      loginURL,
		RedirectURI:   redirectURI,
		ExpectedState: state,
		OrgID:         orgID,
		AppID:         appID,
		SuccessText:   "Kernel authn integration flow completed. Check the terminal logs for token, principal, verify, authenticate, and refresh results.",
		OnSuccess: func(ctx context.Context, result authn.CallbackResult) error {
			logTokenSet(logger, "callback_token_success", result.Tokens)
			logPrincipal(logger, "principal_from_callback", result.Principal)
			if err := runPostLoginChecks(ctx, logger, services, result.Tokens, orgID, appID); err != nil {
				return err
			}
			if ex.RunAdminRead {
				if ex.Username == "" {
					ex.Username = result.Principal.Username
				}
				if ex.Email == "" {
					ex.Email = result.Principal.Email
				}
				if err := runAdminRead(ctx, logger, services, ex, orgID); err != nil {
					return err
				}
			}
			go func() {
				time.Sleep(300 * time.Millisecond)
				finish(nil)
			}()
			return nil
		},
	}
	baseURL := localServerURL(listenAddr)
	mux := http.NewServeMux()
	mux.Handle("/callback", h)
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		logger.Info("redirecting local /login to casdoor", logx.String("login_url", loginURL))
		http.Redirect(w, r, loginURL, http.StatusFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		writeLandingPage(w, baseURL+"/login", redirectURI, state)
	})
	server := &http.Server{Addr: listenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	localLoginURL := baseURL + "/login"
	logger.Info("kernel authn callback server starting",
		logx.String("listen", listenAddr),
		logx.String("local_login_url", localLoginURL),
		logx.String("callback_path", "/callback"),
		logx.Bool("open_browser", openBrowser),
	)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			finish(errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("callback server failed")))
		}
	}()
	go func() {
		<-ctx.Done()
		finish(ctx.Err())
	}()
	if openBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			if err := openURL(localLoginURL); err != nil {
				logger.Warn("open browser failed; copy local_login_url manually", logx.Err(err), logx.String("local_login_url", localLoginURL))
			}
		}()
	}

	err := <-done
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	return err
}

func localServerURL(listenAddr string) string {
	host := strings.TrimSpace(listenAddr)
	if host == "" {
		return "http://localhost:3000"
	}
	if strings.HasPrefix(host, ":") {
		return "http://localhost" + host
	}
	if strings.HasPrefix(host, "0.0.0.0:") {
		return "http://localhost:" + strings.TrimPrefix(host, "0.0.0.0:")
	}
	if strings.HasPrefix(host, "[::]:") {
		return "http://localhost:" + strings.TrimPrefix(host, "[::]:")
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.TrimRight(host, "/")
	}
	return "http://" + host
}

func writeLandingPage(w http.ResponseWriter, loginURL, redirectURI, state string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><title>Kernel Authn Casdoor Example</title></head>
<body style="font-family: sans-serif; max-width: 760px; margin: 48px auto; line-height: 1.6;">
  <h1>Kernel Authn Casdoor Example</h1>
  <p>这个页面只调用 Kernel authn callback 集成，不暴露 Casdoor SDK 给业务层。</p>
  <p><a href="%s" style="font-size: 18px;">登录 Casdoor 并自动完成 code -> token -> principal 测试</a></p>
  <hr>
  <p><b>redirect_uri:</b> %s</p>
  <p><b>state:</b> %s</p>
  <p>登录成功后，终端会输出 token hint、principal、VerifyToken、Authenticate、RefreshToken 等测试日志。</p>
</body>
</html>`, htmlEscape(loginURL), htmlEscape(redirectURI), htmlEscape(state))
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return replacer.Replace(s)
}

func openURL(rawURL string) error {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

func loadRootConfig(path string) (rootConfig, error) {
	if path == "" {
		return rootConfig{}, errorx.BadRequest(errorx.CodeBadRequest, "config path is required")
	}
	cfg := configx.New(configx.WithSource(file.NewSource(path)))
	if err := cfg.Load(); err != nil {
		return rootConfig{}, errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("load config failed"))
	}
	defer cfg.Close()
	var root rootConfig
	if err := cfg.Scan(&root); err != nil {
		return rootConfig{}, errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("scan config failed"))
	}
	return root, nil
}

func effectiveCasdoorConfig(root rootConfig) (casdoorauthn.Config, string, error) {
	defaultScope := root.Example.DefaultScope

	// Prefer the current flat development config when it exists because many
	// local deployments already have a top-level `casdoor:` block. If it is not
	// present, fall back to the new `security.authn.casdoor` shape.
	cfg := root.Security.Authn.Casdoor
	if hasLegacyCasdoorConfig(root.Casdoor) {
		cfg.Endpoint = firstNonEmpty(root.Casdoor.Endpoint, cfg.Endpoint)
		cfg.OrganizationName = firstNonEmpty(root.Casdoor.Organization, cfg.OrganizationName)
		cfg.ApplicationName = firstNonEmpty(root.Casdoor.Application, cfg.ApplicationName)
		cfg.ClientID = firstNonEmpty(root.Casdoor.ClientID, cfg.ClientID)
		cfg.ClientSecret = firstNonEmpty(root.Casdoor.ClientSecret, cfg.ClientSecret)
		cfg.JWTCertificate = firstNonEmpty(root.Casdoor.JWTCertificate, root.Casdoor.Certificate, cfg.JWTCertificate, cfg.Certificate)
		cfg.JWTCertificateFile = firstNonEmpty(root.Casdoor.JWTCertificateFile, cfg.JWTCertificateFile)
		cfg.Certificate = firstNonEmpty(root.Casdoor.Certificate, cfg.Certificate)
		if root.Casdoor.HTTPTimeout != "" {
			d, err := time.ParseDuration(root.Casdoor.HTTPTimeout)
			if err != nil {
				return cfg, defaultScope, errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("invalid casdoor.http_timeout"))
			}
			cfg.Timeout = d
		}
		defaultScope = firstNonEmpty(defaultScope, root.Casdoor.DefaultScope)
	}
	return cfg, defaultScope, nil
}

func hasLegacyCasdoorConfig(cfg legacyCasdoorConfig) bool {
	return firstNonEmpty(cfg.Endpoint, cfg.ClientID, cfg.ClientSecret, cfg.Organization, cfg.Application, cfg.JWTCertificate, cfg.Certificate, cfg.JWTCertificateFile) != ""
}

func mergeExample(ex exampleConfig, opts options) exampleConfig {
	if opts.redirectURI != "" {
		ex.RedirectURI = opts.redirectURI
	}
	if opts.state != "" {
		ex.State = opts.state
	}
	if opts.username != "" {
		ex.Username = opts.username
	}
	if opts.email != "" {
		ex.Email = opts.email
	}
	if opts.listenAddr != "" {
		ex.ListenAddr = opts.listenAddr
	}
	if opts.adminRead {
		ex.RunAdminRead = true
	}
	if opts.listGroups {
		ex.ListGroups = true
	}
	return ex
}

func logFailure(logger logx.Logger, message string, err error) {
	fields := []logx.Field{
		logx.Err(err),
		logx.String("error_code", errorx.CodeOf(err).String()),
		logx.String("safe_message", errorx.MessageOf(err)),
	}
	if cause := errorx.CauseOf(err); cause != nil {
		fields = append(fields, logx.String("cause", cause.Error()))
	}
	if md := errorx.MetadataOf(err); len(md) > 0 {
		fields = append(fields, logx.Any("metadata", md))
	}
	logger.Error(message, fields...)
}

func casdoorConfigFields(cfg casdoorauthn.Config, defaultScope string) []logx.Field {
	return []logx.Field{
		logx.String("endpoint", cfg.Endpoint),
		logx.String("organization", cfg.OrganizationName),
		logx.String("application", cfg.ApplicationName),
		logx.String("client_id", cfg.ClientID),
		logx.Bool("has_client_secret", cfg.ClientSecret != ""),
		logx.Bool("has_jwt_certificate", cfg.JWTCertificate != "" || cfg.Certificate != "" || cfg.JWTCertificateFile != ""),
		logx.Int("jwt_certificate_len", len(strings.TrimSpace(firstNonEmpty(cfg.JWTCertificate, cfg.Certificate)))),
		logx.String("jwt_certificate_file", cfg.JWTCertificateFile),
		logx.Duration("timeout", cfg.Timeout),
		logx.String("default_scope", defaultScope),
	}
}

func mustLogger(cfg logx.Config) (logx.Logger, func()) {
	if cfg.ServiceName == "" {
		cfg = logx.DefaultConfig("dev")
		cfg.ServiceName = "authn-casdoor-example"
		cfg.Level = logx.DebugLevel.String()
		cfg.Format = logx.FormatConsole
		cfg.Output = string(logx.OutputStdout)
	}
	logger, _, err := logx.New(cfg)
	if err != nil {
		fallback := logx.Noop()
		fmt.Fprintf(os.Stderr, "failed to initialize logx: %v\n", err)
		return fallback, func() {}
	}
	return logger, func() { _ = logger.Sync() }
}

func logTokenSet(logger logx.Logger, event string, tokens authn.TokenSet) {
	logger.Info(event,
		logx.String("access_token_hint", tokenHint(tokens.AccessToken)),
		logx.String("refresh_token_hint", tokenHint(tokens.RefreshToken)),
		logx.String("id_token_hint", tokenHint(tokens.IDToken)),
		logx.String("token_type", tokens.TokenType),
		logx.String("scope", tokens.Scope),
		logx.Time("expires_at", tokens.ExpiresAt),
		logx.Bool("has_access_token", tokens.AccessToken != ""),
		logx.Bool("has_refresh_token", tokens.RefreshToken != ""),
		logx.Bool("has_id_token", tokens.IDToken != ""),
	)
}

func logPrincipal(logger logx.Logger, event string, p authn.Principal) {
	logger.Info(event,
		logx.String("subject_id", p.SubjectID),
		logx.String("subject_type", p.SubjectType),
		logx.String("provider", p.Provider),
		logx.String("external_id", p.ExternalID),
		logx.String("org", p.OrgID),
		logx.String("app", p.AppID),
		logx.String("username", p.Username),
		logx.String("name", p.Name),
		logx.String("email", p.Email),
		logx.Int("roles_count", len(p.Roles)),
		logx.Int("groups_count", len(p.Groups)),
		logx.Int("scopes_count", len(p.Scopes)),
		logx.Time("issued_at", p.IssuedAt),
		logx.Time("expires_at", p.ExpiresAt),
	)
}

func logIdentityProfile(logger logx.Logger, event string, profile authn.IdentityProfile) {
	fields := []logx.Field{
		logx.String("subject_id", profile.Principal.SubjectID),
		logx.String("username", profile.Principal.Username),
		logx.String("org", profile.Principal.OrgID),
		logx.String("app", profile.Principal.AppID),
		logx.Bool("partial", profile.IsPartial()),
		logx.Int("warnings_count", len(profile.Warnings)),
		logx.Bool("has_user", profile.HasFullUser()),
		logx.Int("principal_groups_count", len(profile.Principal.Groups)),
		logx.Int("groups_count", len(profile.Groups)),
		logx.String("group_ids", strings.Join(profile.GroupIDs(), ",")),
		logx.String("current_application", firstNonEmpty(profile.CurrentApplication.Name, profile.CurrentApplication.ID)),
		logx.String("current_application_client_id", profile.CurrentApplication.ClientID),
	}
	if len(profile.Warnings) > 0 {
		fields = append(fields, logx.Any("warnings", profile.Warnings))
	}
	logger.Info(event, fields...)
	if profile.HasFullUser() {
		logUser(logger, "identity_profile_user", profile.User)
	}
	for _, group := range profile.Groups {
		logger.Info("identity_profile_group",
			logx.String("id", group.ID),
			logx.String("name", group.Name),
			logx.String("display_name", group.DisplayName),
			logx.String("org", group.OrgID),
			logx.String("parent", group.ParentID),
			logx.String("type", group.Type),
			logx.String("path", group.Path),
			logx.Int("users_count", len(group.Users)),
		)
	}
}

func logUser(logger logx.Logger, event string, user authn.User) {
	logger.Info(event,
		logx.String("id", user.ID),
		logx.String("external_id", user.ExternalID),
		logx.String("org", user.OrgID),
		logx.String("username", user.Username),
		logx.String("display_name", user.DisplayName),
		logx.String("email", user.Email),
		logx.Bool("enabled", user.Enabled),
		logx.Int("roles_count", len(user.Roles)),
		logx.Int("groups_count", len(user.Groups)),
	)
}

func tokenHint(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 12 {
		return token[:2] + "***"
	}
	return token[:6] + "..." + token[len(token)-6:]
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
