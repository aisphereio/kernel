package change

import "testing"

func TestParseGithubURL(t *testing.T) {
	urls := []struct {
		url   string
		owner string
		repo  string
	}{
		{"https://github.com/aisphereio/kernel.git", "aisphereio", "kernel"},
		{"https://github.com/aisphereio/kernel", "aisphereio", "kernel"},
		{"git@github.com:aisphereio/kernel.git", "aisphereio", "kernel"},
		{"https://github.com/aisphereio/kernel.aisphere.io.git", "aisphereio", "kernel.aisphere.io"},
	}
	for _, url := range urls {
		owner, repo := ParseGithubURL(url.url)
		if owner != url.owner {
			t.Fatalf("owner want: %s, got: %s", owner, url.owner)
		}
		if repo != url.repo {
			t.Fatalf("repo want: %s, got: %s", repo, url.repo)
		}
	}
}
