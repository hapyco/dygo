#!/usr/bin/env sh
set -eu

repo="hapyco/dygo"
version="${DYGO_VERSION:-latest}"
install_dir="${DYGO_INSTALL_DIR:-$HOME/.dygo/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$os" in
  darwin) goos="darwin" ;;
  linux) goos="linux" ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac
case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

if [ "$version" = "latest" ]; then
  version="$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
fi
if [ -z "$version" ]; then
  echo "could not resolve dygo version" >&2
  exit 1
fi

asset="dygo_${version}_${goos}_${goarch}.tar.gz"
base_url="https://github.com/$repo/releases/download/$version"
tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

curl -fsSL "$base_url/$asset" -o "$tmp_dir/$asset"
curl -fsSL "$base_url/checksums.txt" -o "$tmp_dir/checksums.txt"

expected="$(awk -v file="$asset" '$2 == file { print $1 }' "$tmp_dir/checksums.txt")"
if [ -z "$expected" ]; then
  echo "checksums.txt does not contain $asset" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp_dir/$asset" | awk '{ print $1 }')"
else
  actual="$(shasum -a 256 "$tmp_dir/$asset" | awk '{ print $1 }')"
fi
if [ "$actual" != "$expected" ]; then
  echo "checksum mismatch for $asset" >&2
  exit 1
fi

tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"
mkdir -p "$install_dir"
install "$tmp_dir/dygo" "$install_dir/dygo"

echo "dygo $version installed to $install_dir/dygo"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "Add this to your shell profile: export PATH=\"$install_dir:\$PATH\"" ;;
esac
