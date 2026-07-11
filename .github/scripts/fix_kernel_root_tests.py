#!/usr/bin/env python3
from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    content = p.read_text(encoding="utf-8")
    if old not in content:
        raise RuntimeError(f"expected text not found in {path}: {old[:160]!r}")
    p.write_text(content.replace(old, new, 1), encoding="utf-8")


replace_once(
    "cmd/kernel/internal/project/project.go",
    '''func resolveLayout(repo string, _ string) (string, error) {
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
''',
    '''func resolveLayout(repo string, workingDir string) (string, error) {
	if strings.TrimSpace(repo) != "" {
		return strings.TrimSpace(repo), nil
	}
	if env := strings.TrimSpace(os.Getenv("KERNEL_LAYOUT")); env != "" {
		return env, nil
	}
	if wd := strings.TrimSpace(workingDir); wd != "" {
		for _, candidate := range []string{
			filepath.Join(filepath.Dir(wd), "layout"),
			filepath.Join(wd, "layout"),
		} {
			info, err := os.Stat(candidate)
			if err == nil && info.IsDir() {
				absolute, err := filepath.Abs(candidate)
				if err != nil {
					return "", fmt.Errorf("resolve local layout %q: %w", candidate, err)
				}
				return filepath.Clean(absolute), nil
			}
		}
	}
	if layout := strings.TrimSpace(projects["service"]); layout != "" {
		return layout, nil
	}
	return "", fmt.Errorf("layout repo not configured; pass --repo or set KERNEL_LAYOUT")
}
''',
)

replace_once(
    "cmd/protoc-gen-aisphere-gateway/main.go",
    '''			access, ok := accessPolicy(method)
			if !ok || access.Gateway.Publish == protooptions.GatewayPublishDisabled {
				continue
			}
''',
    '''			access, ok := accessPolicy(method)
			if !ok {
				continue
			}
''',
)

print("Kernel root test repairs applied")
