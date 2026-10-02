#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d)"
cleanup() {
  rm -rf "$test_root"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

case "$(uname -s)" in
  Darwin) goos="darwin" ;;
  Linux) goos="linux" ;;
  *) echo "installer test is unsupported on this operating system" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) goarch="amd64" ;;
  arm64 | aarch64) goarch="arm64" ;;
  *) echo "installer test is unsupported on this architecture" >&2; exit 1 ;;
esac

install_dir="$test_root/install with spaces"
version="v9.8.7-test"
previous_version="v9.8.6-test"

make_release() {
  local target="$1" reported="${2:-$1}" status="${3:-0}"
  local release_dir="$test_root/releases/$target"
  local asset="dygo_${target}_${goos}_${goarch}.tar.gz"
  mkdir -p "$release_dir/payload"
  cat >"$release_dir/payload/dygo" <<PAYLOAD
#!/usr/bin/env sh
if [ "\${1:-}" = "version" ]; then
  echo "dygo $reported"
  exit $status
fi
exit 1
PAYLOAD
  chmod 0755 "$release_dir/payload/dygo"
  tar -C "$release_dir/payload" -czf "$release_dir/$asset" dygo
  (
    cd "$release_dir"
    if command -v sha256sum >/dev/null 2>&1; then
      sha256sum "$asset" >checksums.txt
    else
      shasum -a 256 "$asset" >checksums.txt
    fi
  )
}

run_install() {
  DYGO_VERSION="$1" \
  DYGO_INSTALL_DIR="$install_dir" \
  DYGO_DOWNLOAD_BASE_URL="file://$test_root/releases/${2:-$1}" \
    "$repo_root/scripts/install.sh" >"$test_root/stdout" 2>"$test_root/stderr"
}

assert_preserved() {
  cmp "$test_root/original-dygo" "$install_dir/dygo"
  test "$("$install_dir/dygo" version)" = "dygo $previous_version"
  if compgen -G "$install_dir/.dygo-install-*" >/dev/null; then
    echo "installer left a staged binary behind" >&2
    exit 1
  fi
}

expect_failure() {
  local diagnostic="$1"
  shift
  if run_install "$@"; then
    echo "installer accepted invalid installation: $diagnostic" >&2
    exit 1
  fi
  grep -q "$diagnostic" "$test_root/stderr"
  assert_preserved
}

make_release "$previous_version"
make_release "$version"
run_install "$previous_version"
test "$("$install_dir/dygo" version)" = "dygo $previous_version"
run_install "$version"
test "$("$install_dir/dygo" version)" = "dygo $version"
run_install "${previous_version#v}" "$previous_version"
test "$("$install_dir/dygo" version)" = "dygo $previous_version"
cp "$install_dir/dygo" "$test_root/original-dygo"

# Redirected output stays plain, and progress does not pollute stdout.
grep -q "Downloading dygo" "$test_root/stderr"
grep -q "Installing dygo" "$test_root/stderr"
if grep -Eq 'Downloading|Installing' "$test_root/stdout" || grep -q $'\033' "$test_root/stderr"; then
  echo "installer progress polluted redirected output" >&2
  exit 1
fi

expect_failure 'invalid dygo version' 'not-a-version'
expect_failure 'curl:' 'v9.8.8-missing'
printf '%s' 'corrupt' >>"$test_root/releases/$version/dygo_${version}_${goos}_${goarch}.tar.gz"
expect_failure 'checksum mismatch' "$version"
make_release "$version" "$previous_version"
expect_failure 'version does not match' "$version"
make_release "$version" "$version" 42
expect_failure 'could not report its version' "$version"
make_release "$version"

# A failed replacement must keep the original executable and remove staging.
mkdir "$test_root/fake-bin"
cat >"$test_root/fake-bin/mv" <<'FAKE'
#!/usr/bin/env sh
echo 'simulated replacement failure' >&2
exit 13
FAKE
chmod 0755 "$test_root/fake-bin/mv"
PATH="$test_root/fake-bin:$PATH" expect_failure 'simulated replacement failure' "$version"
rm "$test_root/fake-bin/mv"

# mv treats a directory (including a directory symlink) as a container, not a file.
original_install_dir="$install_dir"
for target_type in directory directory-symlink; do
  install_dir="$test_root/$target_type"
  mkdir -p "$install_dir"
  if [ "$target_type" = directory ]; then
    mkdir "$install_dir/dygo"
  else
    mkdir "$test_root/unrelated"
    ln -s "$test_root/unrelated" "$install_dir/dygo"
  fi
  printf 'keep' >"$install_dir/dygo/unrelated-file"
  if run_install "$version"; then
    echo "installer accepted a directory destination" >&2
    exit 1
  fi
  grep -q 'installation target is a directory' "$test_root/stderr"
  test "$(cat "$install_dir/dygo/unrelated-file")" = keep
  if compgen -G "$install_dir/dygo/.dygo-install-*" >/dev/null || compgen -G "$install_dir/.dygo-install-*" >/dev/null; then
    echo "installer wrote a candidate into a directory destination" >&2
    exit 1
  fi
  if [ "$target_type" = directory-symlink ]; then test -L "$install_dir/dygo"; fi
done
install_dir="$original_install_dir"

# Signal delivery during a download must exit, not run the next download after cleanup.
cat >"$test_root/fake-bin/curl" <<'FAKE'
#!/usr/bin/env sh
if [ -f "$DYGO_TEST_SIGNAL_LOG" ]; then
  echo continued >>"$DYGO_TEST_SIGNAL_LOG"
  exit 22
fi
printf '%s\n' "$*" >"$DYGO_TEST_SIGNAL_LOG"
kill -"$DYGO_TEST_SIGNAL" "$PPID"
FAKE
chmod 0755 "$test_root/fake-bin/curl"
for signal in INT TERM; do
  rm -f "$test_root/signal-log"
  status=0
  PATH="$test_root/fake-bin:$PATH" DYGO_TEST_SIGNAL_LOG="$test_root/signal-log" DYGO_TEST_SIGNAL="$signal" \
    run_install "$version" || status=$?
  expected_status=130
  if [ "$signal" = TERM ]; then expected_status=143; fi
  test "$status" = "$expected_status"
  test "$(wc -l <"$test_root/signal-log" | tr -d ' ')" = 1
  # The download target's parent directory was removed by the exit trap.
  download_path="$(sed 's/.* -o //' "$test_root/signal-log")"
  test ! -d "$(dirname "$download_path")"
  assert_preserved
done

python3 "$repo_root/scripts/test-install-cancel.py" "$repo_root/scripts/install.sh"

echo "installer lifecycle passed: fresh install, upgrade, downgrade, verification, replacement failure, cancellation"
