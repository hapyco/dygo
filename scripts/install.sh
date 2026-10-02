#!/usr/bin/env sh
set -eu

repo="hapyco/dygo"
version="${DYGO_VERSION:-latest}"
install_dir="${DYGO_INSTALL_DIR:-$HOME/.dygo/bin}"
download_base_url="${DYGO_DOWNLOAD_BASE_URL:-}"
spinner_pid=""
spinner_message=""
tmp_dir=""
staged_binary=""

start_spinner() {
  spinner_message="$1"
  if [ ! -t 2 ] || [ -n "${CI:-}" ] || [ "${TERM:-}" = dumb ]; then
    printf '%s...\n' "$spinner_message" >&2
    return
  fi
  (
    sleep 0.3
    set -- '|' '/' '-' '\'
    while :; do
      for frame do
        printf '\r\033[2K%s %s' "$frame" "$spinner_message" >&2
        sleep 0.1
      done
    done
  ) &
  spinner_pid=$!
}

stop_spinner() {
  if [ -z "$spinner_pid" ]; then
    return
  fi
  kill "$spinner_pid" 2>/dev/null || true
  wait "$spinner_pid" 2>/dev/null || true
  printf '\r\033[2K' >&2
  spinner_pid=""
}

cleanup() {
  stop_spinner
  if [ -n "$tmp_dir" ]; then
    rm -rf "$tmp_dir"
  fi
  if [ -n "$staged_binary" ]; then
    rm -f "$staged_binary"
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

fail() {
  stop_spinner
  printf '%s\n' "$1" >&2
  exit 1
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "required command is unavailable: $1"
  fi
}

for command in awk curl grep head install mktemp sed sleep tar; do
  require_command "$command"
done

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
  start_spinner "Resolving latest dygo release"
  version="$(curl -fsSL -H "Accept: application/vnd.github+json" -H "User-Agent: dygo-installer" "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  stop_spinner
elif [ "${version#v}" = "$version" ]; then
  version="v$version"
fi
if [ -z "$version" ]; then
  fail "could not resolve dygo version"
fi
if ! printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$'; then
  fail "invalid dygo version: $version"
fi

asset="dygo_${version}_${goos}_${goarch}.tar.gz"
base_url="${download_base_url:-https://github.com/$repo/releases/download/$version}"
tmp_dir="$(mktemp -d)"

start_spinner "Downloading dygo $version"
curl -fsSL "$base_url/$asset" -o "$tmp_dir/$asset"
curl -fsSL "$base_url/checksums.txt" -o "$tmp_dir/checksums.txt"
stop_spinner

start_spinner "Installing dygo $version"
expected="$(awk -v file="$asset" '$2 == file || $2 == "*" file { print $1 }' "$tmp_dir/checksums.txt")"
if [ -z "$expected" ]; then
  fail "checksums.txt does not contain $asset"
fi
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp_dir/$asset" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmp_dir/$asset" | awk '{ print $1 }')"
else
  fail "required checksum command is unavailable: sha256sum or shasum"
fi
if [ "$actual" != "$expected" ]; then
  fail "checksum mismatch for $asset"
fi

tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"
if [ ! -f "$tmp_dir/dygo" ]; then
  fail "release archive does not contain dygo"
fi
chmod 0755 "$tmp_dir/dygo"
if ! reported_version="$("$tmp_dir/dygo" version)"; then
  fail "downloaded binary could not report its version"
fi
if [ "$reported_version" != "dygo $version" ]; then
  fail "downloaded binary version does not match $version"
fi

mkdir -p "$install_dir"
if [ -d "$install_dir/dygo" ]; then
  fail "installation target is a directory: $install_dir/dygo"
fi
staged_binary="$(mktemp "$install_dir/.dygo-install-XXXXXX")"
install -m 0755 "$tmp_dir/dygo" "$staged_binary"
mv -f "$staged_binary" "$install_dir/dygo"
staged_binary=""
stop_spinner

echo "dygo $version installed to $install_dir/dygo"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "Add this to your shell profile: export PATH=\"$install_dir:\$PATH\"" ;;
esac
