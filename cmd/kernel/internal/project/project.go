package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/aisphereio/kernel/cmd/kernel/internal/base"
)

const (
	layoutProfileFull = "full"
	layoutProfileMVP  = "mvp"
)

var projects = map[string]string{
	"service": "https://github.com/aisphereio/kernel-layout.git",
	"admin":   "https://github.com/aisphereio/kernel-admin.git",
}

// CmdNew represents the new command.
var CmdNew = &cobra.Command{
	Use:   "new",
	Short: "Create a service template",
	Long:  "Create a service project using the repository template. Example: kernel new helloworld",
	Run:   run,
}

var (
	nomod             bool
	repo              string
	branch            string
	timeout           = "60s"
	features          string
	disableFeatures   string
	profile           string
	mvp               bool
	dbDriver          string
	cacheDriver       string
	objectStoreDriver string
	authnProvider     string
	authzProvider     string
	kernelVersion     string
)

func init() {
	defaults := defaultScaffoldOptions()
	CmdNew.Flags().StringVarP(&repo, "repo", "r", repo, "custom repo url or local layout path")
	CmdNew.Flags().StringVarP(&branch, "branch", "b", branch, "repo branch")
	CmdNew.Flags().StringVarP(&timeout, "timeout", "t", timeout, "time out")
	CmdNew.Flags().BoolVarP(&nomod, "nomod", "", nomod, "retain go mod")
	CmdNew.Flags().StringVar(&profile, "profile", defaults.Profile, "layout profile to apply after copy: full or mvp")
	CmdNew.Flags().BoolVar(&mvp, "mvp", false, "shortcut for --profile mvp")
	CmdNew.Flags().StringVar(&features, "features", strings.Join(defaults.Features, ","), "enabled scaffold features")
	CmdNew.Flags().StringVar(&disableFeatures, "disable", "", "comma-separated features to disable after applying the layout, e.g. iam,gateway,dtmx")
	CmdNew.Flags().StringVar(&dbDriver, "db-driver", defaults.DBDriver, "default dbx driver")
	CmdNew.Flags().StringVar(&cacheDriver, "cache-driver", defaults.CacheDriver, "default cachex driver")
	CmdNew.Flags().StringVar(&objectStoreDriver, "objectstore-driver", defaults.ObjectStoreDriver, "default objectstorex driver")
	CmdNew.Flags().StringVar(&authnProvider, "authn-provider", defaults.AuthnProvider, "default authn provider")
	CmdNew.Flags().StringVar(&authzProvider, "authz-provider", defaults.AuthzProvider, "default authz provider")
	CmdNew.Flags().StringVar(&kernelVersion, "kernel-version", defaults.KernelVersion, "kernel module/tool version for generated Makefile installs")
}

func run(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	t, err := time.ParseDuration(timeout)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()
	name := ""
	if len(args) == 0 {
		prompt := &survey.Input{
			Message: "What is project name ?",
			Help:    "Created project name.",
		}
		err = survey.AskOne(prompt, &name)
		if err != nil || name == "" {
			return
		}
	} else {
		name = args[0]
	}
	projectName, workingDir := processProjectParams(name, wd)
	p := &Project{Name: projectName}
	done := make(chan error, 1)
	opts := scaffoldOptionsFromFlags(cmd)
	repoURL, err := resolveLayout(repo, wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31mERROR: failed to resolve layout(%s)\033[m\n", err.Error())
		return
	}
	go func() {
		if !nomod {
			done <- p.New(ctx, workingDir, repoURL, branch, opts)
			return
		}
		projectRoot := getgomodProjectRoot(workingDir)
		if gomodIsNotExistIn(projectRoot) {
			done <- fmt.Errorf("🚫 go.mod don't exists in %s", projectRoot)
			return
		}

		packagePath, e := filepath.Rel(projectRoot, filepath.Join(workingDir, projectName))
		if e != nil {
			done <- fmt.Errorf("🚫 failed to get relative path: %v", e)
			return
		}
		packagePath = strings.ReplaceAll(packagePath, "\\", "/")

		mod, e := base.ModulePath(filepath.Join(projectRoot, "go.mod"))
		if e != nil {
			done <- fmt.Errorf("🚫 failed to parse `go.mod`: %v", e)
			return
		}
		// Get the relative path for adding a project based on Go modules
		p.Path = filepath.Join(strings.TrimPrefix(workingDir, projectRoot+"/"), p.Name)
		done <- p.Add(ctx, workingDir, repoURL, branch, mod, packagePath, opts)
	}()
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			fmt.Fprint(os.Stderr, "\033[31mERROR: project creation timed out\033[m\n")
			return
		}
		fmt.Fprintf(os.Stderr, "\033[31mERROR: failed to create project(%s)\033[m\n", ctx.Err().Error())
	case err = <-done:
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mERROR: Failed to create project(%s)\033[m\n", err.Error())
		}
	}
}

type ScaffoldOptions struct {
	Profile           string
	Features          []string
	DisabledFeatures  []string
	DBDriver          string
	CacheDriver       string
	ObjectStoreDriver string
	AuthnProvider     string
	AuthzProvider     string
	KernelVersion     string
}

func defaultScaffoldOptions() ScaffoldOptions {
	return ScaffoldOptions{
		Profile:           layoutProfileFull,
		Features:          []string{"configx", "logx", "errorx", "metricsx", "dbx", "cachex", "objectstorex", "dtmx", "auditx", "authn", "authz", "access", "gateway", "http", "grpc"},
		DBDriver:          "postgres",
		CacheDriver:       "redis",
		ObjectStoreDriver: "minio",
		AuthnProvider:     "casdoor",
		AuthzProvider:     "spicedb",
		KernelVersion:     defaultKernelVersion(),
	}
}

func mvpScaffoldOptions() ScaffoldOptions {
	defaults := defaultScaffoldOptions()
	defaults.Profile = layoutProfileMVP
	defaults.Features = []string{"configx", "logx", "errorx", "http", "grpc"}
	defaults.DisabledFeatures = []string{"iam", "authn", "authz", "access", "gateway", "dbx", "cachex", "objectstorex", "dtmx", "auditx", "metricsx"}
	return defaults
}

func scaffoldOptionsFromFlags(cmd *cobra.Command) ScaffoldOptions {
	opts := defaultScaffoldOptions()
	if mvp {
		opts = mvpScaffoldOptions()
	}
	if strings.TrimSpace(profile) != "" {
		opts.Profile = strings.ToLower(strings.TrimSpace(profile))
	}
	if mvp {
		opts.Profile = layoutProfileMVP
	}
	if cmd != nil && cmd.Flags().Changed("features") {
		opts.Features = splitCSV(features)
	}
	opts.DisabledFeatures = mergeDisabledFeatures(opts.DisabledFeatures, splitCSV(disableFeatures))
	opts.Features = removeDisabledFromFeatures(opts.Features, opts.DisabledFeatures)
	version := strings.TrimSpace(kernelVersion)
	if version == "" {
		version = defaultKernelVersion()
	}
	opts.DBDriver = strings.TrimSpace(dbDriver)
	opts.CacheDriver = strings.TrimSpace(cacheDriver)
	opts.ObjectStoreDriver = strings.TrimSpace(objectStoreDriver)
	opts.AuthnProvider = strings.TrimSpace(authnProvider)
	opts.AuthzProvider = strings.TrimSpace(authzProvider)
	opts.KernelVersion = version
	return opts
}

func defaultKernelVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "latest"
	}
	version := strings.TrimSpace(info.Main.Version)
	if version == "" || version == "(devel)" {
		return "latest"
	}
	return version
}

func (o ScaffoldOptions) HasFeature(feature string) bool {
	for _, got := range o.Features {
		if strings.EqualFold(got, feature) {
			return true
		}
	}
	return false
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key := strings.ToLower(part)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, part)
	}
	return out
}

func mergeDisabledFeatures(base []string, extra []string) []string {
	out := make([]string, 0, len(base)+len(extra))
	seen := map[string]struct{}{}
	for _, feature := range append(base, extra...) {
		feature = strings.ToLower(strings.TrimSpace(feature))
		if feature == "" {
			continue
		}
		if feature == "iam" {
			for _, expanded := range []string{"iam", "authn", "authz", "access", "gateway"} {
				if _, ok := seen[expanded]; !ok {
					seen[expanded] = struct{}{}
					out = append(out, expanded)
				}
			}
			continue
		}
		if _, ok := seen[feature]; ok {
			continue
		}
		seen[feature] = struct{}{}
		out = append(out, feature)
	}
	return out
}

func removeDisabledFromFeatures(features []string, disabled []string) []string {
	disabledSet := map[string]struct{}{}
	for _, feature := range disabled {
		disabledSet[strings.ToLower(strings.TrimSpace(feature))] = struct{}{}
	}
	out := make([]string, 0, len(features))
	for _, feature := range features {
		key := strings.ToLower(strings.TrimSpace(feature))
		if key == "" {
			continue
		}
		if _, ok := disabledSet[key]; ok {
			continue
		}
		out = append(out, feature)
	}
	return out
}

func resolveLayout(repo string, _ string) (string, error) {
	if strings.TrimSpace(repo) != "" {
		return strings.TrimSpace(repo), nil
	}
	if env := strings.TrimSpace(os.Getenv("KERNEL_LAYOUT")); env != "" {
		return env, nil
	}
	if layout := strings.TrimSpace(projects["service"]); layout != "" {
		return layout, nil
	}
	return "", fmt.Errorf("layout repo not configured; pass --repo or set KERNEL_LAYOUT")
}

func processProjectParams(projectName string, workingDir string) (projectNameResult, workingDirResult string) {
	_projectDir := projectName
	_workingDir := workingDir
	// Process ProjectName with system variable
	if strings.HasPrefix(projectName, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// cannot get user home return fallback place dir
			return _projectDir, _workingDir
		}
		_projectDir = filepath.Join(homeDir, projectName[2:])
	}

	// check path is relative
	if !filepath.IsAbs(projectName) {
		absPath, err := filepath.Abs(projectName)
		if err != nil {
			return _projectDir, _workingDir
		}
		_projectDir = absPath
	}

	return filepath.Base(_projectDir), filepath.Dir(_projectDir)
}

func getgomodProjectRoot(dir string) string {
	if dir == filepath.Dir(dir) {
		return dir
	}
	if gomodIsNotExistIn(dir) {
		return getgomodProjectRoot(filepath.Dir(dir))
	}
	return dir
}

func gomodIsNotExistIn(dir string) bool {
	_, e := os.Stat(filepath.Join(dir, "go.mod"))
	return os.IsNotExist(e)
}

func selectRepo() (string, error) {
	var (
		choice    string
		customURL string
	)
	form := huh.NewForm(
		// 1) Select group (always visible)
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a template").
				Options(
					huh.NewOption("Service", "service"),
					huh.NewOption("Admin", "admin"),
					huh.NewOption("Custom (enter repo URL)", "custom"),
				).
				Value(&choice),
		),
		// 2) Input group (only visible when choice == "custom")
		huh.NewGroup(
			huh.NewInput().
				Title("Enter custom repository URL").
				Placeholder("https://github.com/owner/repo.git").
				Value(&customURL).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("repo URL cannot be empty")
					}
					return nil
				}),
		).WithHideFunc(func() bool {
			return choice != "custom"
		}),
	)
	if err := form.Run(); err != nil {
		panic(err)
	}
	if choice == "custom" {
		return strings.TrimSpace(customURL), nil
	}
	return projects[choice], nil
}
