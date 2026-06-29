#!/usr/bin/env sh
set -eu

version="${1:-}"
repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
root_module="github.com/aisphereio/kernel"
command_packages="
cmd/kernel
cmd/protoc-gen-go-authz
cmd/protoc-gen-go-errors
cmd/protoc-gen-go-http
"

fail() {
  echo "$1" >&2
  exit 1
}

cd "$repo_root"

got_root_module="$(sed -n 's/^module[[:space:]][[:space:]]*//p' go.mod | head -n 1)"
[ "$got_root_module" = "$root_module" ] || fail "wrong root module path: want $root_module, got $got_root_module"
echo "ok root module $root_module"

for dir in $command_packages; do
  [ -d "$dir" ] || fail "missing command package directory: $dir"

  gomod="$dir/go.mod"
  [ ! -f "$gomod" ] || fail "command package must stay in root module; remove nested go.mod: $gomod"
  echo "ok root-owned package $root_module/$dir"
done

if [ -n "$version" ]; then
  git diff --quiet || fail "worktree has unstaged changes; commit before checking release tag $version"
  git diff --cached --quiet || fail "worktree has staged changes; commit before checking release tag $version"
  git rev-parse -q --verify "refs/tags/$version" >/dev/null || fail "missing root module tag: $version"
  tag_commit="$(git rev-list -n 1 "$version")"
  head_commit="$(git rev-parse HEAD)"
  [ "$tag_commit" = "$head_commit" ] || fail "root tag $version does not point to HEAD"
  echo "ok root tag $version"
fi
