#!/bin/sh
set -eu

flake=flake.nix
fake_hash=sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=

npm_hash="$(nix run github:NixOS/nixpkgs/nixpkgs-unstable#prefetch-npm-deps -- package-lock.json | tail -n 1)"
case "$npm_hash" in
  sha256-*) ;;
  *) echo "Invalid npm dependency hash: $npm_hash" >&2; exit 1 ;;
esac
sed -E "s|npmDepsHash = \"sha256-[^\"]+\";|npmDepsHash = \"$npm_hash\";|" "$flake" > "$flake.tmp"
mv "$flake.tmp" "$flake"

sed -E "s|vendorHash = \"sha256-[^\"]+\";|vendorHash = \"$fake_hash\";|" "$flake" > "$flake.tmp"
mv "$flake.tmp" "$flake"
system="$(nix eval --impure --raw --expr builtins.currentSystem)"
if output="$(nix build ".#packages.$system.default.goModules" --no-link 2>&1)"; then
  echo "Expected the fake Go vendor hash to fail" >&2
  exit 1
fi
vendor_hash="$(printf '%s\n' "$output" | sed -n 's/^[[:space:]]*got: \(sha256-[^[:space:]]*\)$/\1/p' | tail -n 1)"
case "$vendor_hash" in
  sha256-*) ;;
  *) printf '%s\n' "$output" >&2; echo "Could not determine Go vendor hash" >&2; exit 1 ;;
esac
sed -E "s|vendorHash = \"sha256-[^\"]+\";|vendorHash = \"$vendor_hash\";|" "$flake" > "$flake.tmp"
mv "$flake.tmp" "$flake"

grep -Fq "npmDepsHash = \"$npm_hash\";" "$flake"
grep -Fq "vendorHash = \"$vendor_hash\";" "$flake"
