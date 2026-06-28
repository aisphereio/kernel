package base

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

var unExpandVarPath = []string{"~", ".", ".."}

// Repo is git repository manager.
type Repo struct {
	url    string
	home   string
	branch string
}

func repoDir(url string) string {
	vcsURL, err := ParseVCSUrl(url)
	if err != nil {
		return url
	}
	// check host contains port
	host, _, err := net.SplitHostPort(vcsURL.Host)
	if err != nil {
		host = vcsURL.Host
	}
	for _, p := range unExpandVarPath {
		host = strings.TrimLeft(host, p)
	}
	dir := path.Base(path.Dir(vcsURL.Path))
	url = fmt.Sprintf("%s/%s", host, dir)
	return url
}

// NewRepo new a repository manager.
func NewRepo(url string, branch string) *Repo {
	return &Repo{
		url:    url,
		home:   kernelHomeWithDir("repo/" + repoDir(url)),
		branch: branch,
	}
}

// Path returns the repository cache path.
func (r *Repo) Path() string {
	if local, ok := r.localPath(); ok {
		return local
	}
	start := strings.LastIndex(r.url, "/")
	end := strings.LastIndex(r.url, ".git")
	if end == -1 {
		end = len(r.url)
	}
	var branch string
	if r.branch == "" {
		branch = "@main"
	} else {
		branch = "@" + r.branch
	}
	return path.Join(r.home, r.url[start+1:end]+branch)
}

func (r *Repo) localPath() (string, bool) {
	raw := strings.TrimSpace(os.ExpandEnv(r.url))
	if raw == "" {
		return "", false
	}
	if strings.HasPrefix(raw, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		raw = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(raw, "~"), string(os.PathSeparator)))
	}
	if !filepath.IsAbs(raw) {
		abs, err := filepath.Abs(raw)
		if err != nil {
			return "", false
		}
		raw = abs
	}
	info, err := os.Stat(raw)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return raw, true
}

// Pull fetch the repository from remote url.
func (r *Repo) Pull(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "HEAD")
	cmd.Dir = r.Path()
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	cmd = exec.CommandContext(ctx, "git", "pull")
	cmd.Dir = r.Path()
	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		return err
	}
	return err
}

// Clone clones the repository to cache path.
func (r *Repo) Clone(ctx context.Context) error {
	if _, ok := r.localPath(); ok {
		return nil
	}
	if _, err := os.Stat(r.Path()); !os.IsNotExist(err) {
		return r.Pull(ctx)
	}
	var cmd *exec.Cmd
	if r.branch == "" {
		cmd = exec.CommandContext(ctx, "git", "clone", r.url, r.Path())
	} else {
		cmd = exec.CommandContext(ctx, "git", "clone", "-b", r.branch, r.url, r.Path())
	}
	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		return err
	}
	return nil
}

// CopyTo copies the repository to project path.
func (r *Repo) CopyTo(ctx context.Context, to string, modPath string, ignores []string) error {
	if err := r.Clone(ctx); err != nil {
		return err
	}
	src := r.Path()
	mod, err := ModulePath(filepath.Join(src, "go.mod"))
	if err != nil {
		return err
	}
	return copyDir(src, to, []string{mod, modPath}, ignores)
}

// CopyToV2 copies the repository to project path
func (r *Repo) CopyToV2(ctx context.Context, to string, modPath string, ignores, replaces []string) error {
	if err := r.Clone(ctx); err != nil {
		return err
	}
	src := r.Path()
	mod, err := ModulePath(filepath.Join(src, "go.mod"))
	if err != nil {
		return err
	}
	replaces = append([]string{mod, modPath}, replaces...)
	return copyDir(src, to, replaces, ignores)
}
