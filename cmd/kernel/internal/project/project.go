package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/aisphereio/kernel/cmd/kernel/internal/base"
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
	CmdNew.Flags().StringVar(&features, "features", strings.Join(defaults.Features, ","), "enabled scaffold features")
	CmdNew.Flags().StringVar(&dbDriver, "db-driver", defaults.DBDriver, "default dbx driver")
	CmdNew.Flags().StringVar(&cacheDriver, "cache-driver", defaults.CacheDriver, "default cachex driver")
	CmdNew.Flags().StringVar(&objectStoreDriver, "objectstore-driver", defaults.ObjectStoreDriver, "default objectstorex driver")
	CmdNew.Flags().StringVar(&authnProvider, "authn-provider", defaults.AuthnProvider, "default authn provider")
	CmdNew.Flags().StringVar(&authzProvider, "authz-provider", defaults.AuthzProvider, "default authz provider")
	CmdNew.Flags().StringVar(&kernelVersion, "kernel-version", defaults.KernelVersion, "kernel module/tool version for generated Makefile installs")
}

func run(_ *cobra.Command, args []string) {
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
	repoURL, err := resolveLayout(repo, wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31mERROR: failed to resolve layout(%s)\033[m\n", err.Error())
		return
	}
	opts := scaffoldOptionsFromFlags()
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
	Features          []string
	DBDriver          string
	CacheDriver       string
	ObjectStoreDriver string
	AuthnProvider     string
	AuthzProvider     string
	KernelVersion     string
}

func defaultScaffoldOptions() ScaffoldOptions {
	return ScaffoldOptions{
		Features:          []string{"dbx", "cachex", "objectstorex", "authn", "authz", "auditx", "metricsx", "logx", "configx"},
		DBDriver:          "postgres",
		CacheDriver:       "redis",
		ObjectStoreDriver: "minio",
		AuthnProvider:     "casdoor",
		AuthzProvider:     "spicedb",
		KernelVersion:     defaultKernelVersion(),
	}
}

func scaffoldOptionsFromFlags() ScaffoldOptions {
	version := strings.TrimSpace(kernelVersion)
	if version == "" {
		version = defaultKernelVersion()
	}
	return ScaffoldOptions{
		Features:          splitCSV(features),
		DBDriver:          strings.TrimSpace(dbDriver),
		CacheDriver:       strings.TrimSpace(cacheDriver),
		ObjectStoreDriver: strings.TrimSpace(objectStoreDriver),
		AuthnProvider:     strings.TrimSpace(authnProvider),
		AuthzProvider:     strings.TrimSpace(authzProvider),
		KernelVersion:     version,
	}
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

func resolveLayout(repo string, wd string) (string, error) {
	if strings.TrimSpace(repo) != "" {
		return strings.TrimSpace(repo), nil
	}
	if env := strings.TrimSpace(os.Getenv("KERNEL_LAYOUT")); env != "" {
		return env, nil
	}
	for _, start := range layoutSearchRoots(wd) {
		if layout, ok := findLocalLayout(start); ok {
			return layout, nil
		}
	}
	return "", fmt.Errorf("local layout not found; pass --repo or set KERNEL_LAYOUT")
}

func layoutSearchRoots(wd string) []string {
	roots := []string{wd}
	if _, file, _, ok := runtime.Caller(0); ok {
		roots = append(roots, filepath.Dir(file))
	}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	return roots
}

func findLocalLayout(start string) (string, bool) {
	if start == "" {
		return "", false
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(dir, "layout")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
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
