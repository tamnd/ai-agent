#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Usage: scripts/release.sh <tag> [notes file]
# It builds the archives of each target into dist/, and then makes or updates the GitHub release of the tag.
# CI runs it on each tag. A maintainer can run it on a computer when the CI queue is slow; CI then replaces the files with attested copies.
set -eu
tag=$1
notes=${2:-}
case "$tag" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo "the tag $tag is not vX.Y.Z" >&2; exit 2 ;; esac
git rev-parse -q --verify "refs/tags/$tag" >/dev/null || { echo "the tag $tag does not exist" >&2; exit 2; }
out=$(pwd)
rm -rf dist
mkdir -p dist
# Build from a clean tree of the tag, so that local changes never go into a release.
src=$(mktemp -d)
trap 'git -C "$out" worktree remove --force "$src" >/dev/null 2>&1 || rm -rf "$src"' EXIT
git worktree add -q --detach "$src" "$tag"
cd "$src"
mkdir -p build/release
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os=${target%/*}
  arch=${target#*/}
  name="aik_${os}_${arch}"
  ext=""
  if [ "$os" = windows ]; then ext=".exe"; fi
  mkdir -p "build/release/$name"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w -X main.version=$tag" -o "build/release/$name/aik$ext" ./cmd/aik
  cp LICENSE README.md THIRD_PARTY.md "build/release/$name/"
  if [ "$os" = windows ]; then
    (cd build/release && zip -qr "$out/dist/$name.zip" "$name")
  else
    COPYFILE_DISABLE=1 tar -C build/release -czf "$out/dist/$name.tar.gz" "$name"
  fi
done
if command -v sha256sum >/dev/null; then sum="sha256sum"; else sum="shasum -a 256"; fi
(cd "$out/dist" && $sum ./*.tar.gz ./*.zip > SHA256SUMS)
cd "$out"
[ "${BUILD_ONLY:-}" = 1 ] && exit 0
repo=${GITHUB_REPOSITORY:-tamnd/ai-agent}
if gh release view "$tag" --repo "$repo" >/dev/null 2>&1; then
  gh release upload "$tag" dist/* --repo "$repo" --clobber
elif [ -n "$notes" ]; then
  gh release create "$tag" dist/* --repo "$repo" --title "$tag" --notes-file "$notes" --generate-notes --verify-tag
else
  gh release create "$tag" dist/* --repo "$repo" --title "$tag" --generate-notes --verify-tag
fi
