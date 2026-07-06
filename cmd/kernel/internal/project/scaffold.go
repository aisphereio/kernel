package project

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const layoutControlDir = ".kernel"

func applyScaffoldOptions(root string, opts ScaffoldOptions) error {
	if opts.Profile != "" && !strings.EqualFold(opts.Profile, layoutProfileFull) {
		if err := applyLayoutControl(root, filepath.Join(layoutControlDir, "profiles", strings.ToLower(opts.Profile))); err != nil {
			return err
		}
	}
	for _, feature := range opts.DisabledFeatures {
		feature = strings.ToLower(strings.TrimSpace(feature))
		if feature == "" {
			continue
		}
		if err := applyLayoutControl(root, filepath.Join(layoutControlDir, "features", feature)); err != nil {
			return err
		}
	}
	_ = os.RemoveAll(filepath.Join(root, layoutControlDir))

	serviceName := filepath.Base(root)
	replacements := map[string]string{
		"__KERNEL_SERVICE_NAME__":       serviceName,
		"__KERNEL_FEATURES__":           strings.Join(opts.Features, ","),
		"__KERNEL_DISABLED_FEATURES__":  strings.Join(opts.DisabledFeatures, ","),
		"__KERNEL_PROFILE__":            opts.Profile,
		"__KERNEL_DB_DRIVER__":          opts.DBDriver,
		"__KERNEL_CACHE_DRIVER__":       opts.CacheDriver,
		"__KERNEL_OBJECTSTORE_DRIVER__": opts.ObjectStoreDriver,
		"__KERNEL_AUTHN_PROVIDER__":     opts.AuthnProvider,
		"__KERNEL_AUTHZ_PROVIDER__":     opts.AuthzProvider,
		"__KERNEL_VERSION__":            opts.KernelVersion,
	}
	return replaceInLayoutFiles(root, replacements)
}

func applyLayoutControl(root string, controlRel string) error {
	controlDir := filepath.Join(root, controlRel)
	info, err := os.Stat(controlDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	if err := applyRemoveList(root, filepath.Join(controlDir, "remove.txt")); err != nil {
		return err
	}
	overlay := filepath.Join(controlDir, "overlay")
	if info, err := os.Stat(overlay); err == nil && info.IsDir() {
		return copyOverlay(overlay, root)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func applyRemoveList(root string, filename string) error {
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = filepath.Clean(filepath.FromSlash(line))
		if line == "." || strings.HasPrefix(line, "..") || filepath.IsAbs(line) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, line)); err != nil {
			return err
		}
	}
	return nil
}

func copyOverlay(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}

func replaceInLayoutFiles(root string, replacements map[string]string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			switch name {
			case ".git", ".bin", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".pb.go") {
			return nil
		}
		return replaceInFile(path, replacements)
	})
}

func replaceInFile(name string, replacements map[string]string) error {
	data, err := os.ReadFile(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if bytes.Contains(data, []byte{0}) {
		return nil
	}
	out := string(data)
	for old, next := range replacements {
		out = strings.ReplaceAll(out, old, next)
	}
	if out == string(data) {
		return nil
	}
	return os.WriteFile(name, []byte(out), 0o644)
}
