package project

import (
	"os"
	"path/filepath"
	"strings"
)

func applyScaffoldOptions(root string, opts ScaffoldOptions) error {
	replacements := map[string]string{
		"__KERNEL_FEATURES__":           strings.Join(opts.Features, ","),
		"__KERNEL_DB_DRIVER__":          opts.DBDriver,
		"__KERNEL_CACHE_DRIVER__":       opts.CacheDriver,
		"__KERNEL_OBJECTSTORE_DRIVER__": opts.ObjectStoreDriver,
		"__KERNEL_AUTHN_PROVIDER__":     opts.AuthnProvider,
		"__KERNEL_AUTHZ_PROVIDER__":     opts.AuthzProvider,
	}
	for _, name := range []string{
		filepath.Join(root, "configs", "config.yaml"),
		filepath.Join(root, "README.md"),
	} {
		if err := replaceInFile(name, replacements); err != nil {
			return err
		}
	}
	return nil
}

func replaceInFile(name string, replacements map[string]string) error {
	data, err := os.ReadFile(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
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
