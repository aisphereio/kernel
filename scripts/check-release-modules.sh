#!/usr/bin/env sh
set -eu

version="${1:-}"
repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
root_module="github.com/aisphereio/kernel"
command_modules="
cmd/kernel
cmd/protoc-gen-go-authz
cmd/protoc-gen-go-errors
cmd/protoc-gen-go-http
"

fail() {
  echo "$1" >&2
  exit 1
}

module_path() {
  sed -n 's/^module[[:space:]][[:space:]]*//p' "$1" | head -n 1
}

cd "$repo_root"

for dir in $command_modules; do
  gomod="$dir/go.mod"
  [ -f "$gomod" ] || fail "missing command module go.mod: $gomod"

  want="$root_module/$dir"
  got="$(module_path "$gomod")"
  [ "$got" = "$want" ] || fail "wrong module path in $gomod: want $want, got $got"
  echo "ok module $want"

  if [ -n "$version" ]; then
    tag="$dir/$version"
    git rev-parse -q --verify "refs/tags/$tag" >/dev/null || fail "missing command module tag: $tag"
    echo "ok tag $tag"
  fi
done

if [ -n "$version" ]; then
  git rev-parse -q --verify "refs/tags/$version" >/dev/null || fail "missing root module tag: $version"
  echo "ok tag $version"
fi
