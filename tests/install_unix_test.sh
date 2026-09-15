#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/zipit-install-test.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM

fixture="$work/releases"
fakebin="$work/fakebin"
mkdir -p "$fixture" "$fakebin"

cat >"$work/zipit" <<'EOF'
#!/bin/sh
echo "Zipit v0.1.0"
EOF
chmod 0755 "$work/zipit"

for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  tar -czf "$fixture/zipit_0.1.0_${target}.tar.gz" -C "$work" zipit
done
(
  cd "$fixture"
  sha256sum zipit_0.1.0_*.tar.gz >checksums.txt
)

cat >"$fakebin/curl" <<'EOF'
#!/bin/sh
set -eu
output=
write_out=
url=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      output=$2
      shift 2
      ;;
    -w)
      write_out=$2
      shift 2
      ;;
    -*) shift ;;
    *)
      url=$1
      shift
      ;;
  esac
done
case "$url" in
  https://github.com/acme/zipit-test/releases/latest)
    [ -n "$write_out" ]
    printf '%s' 'https://github.com/acme/zipit-test/releases/tag/v0.1.0'
    ;;
  https://github.com/acme/zipit-test/releases/download/v0.1.0/*)
    [ -n "$output" ]
    cp "$FIXTURE_DIR/${url##*/}" "$output"
    ;;
  *)
    echo "unexpected URL: $url" >&2
    exit 1
    ;;
esac
EOF
chmod 0755 "$fakebin/curl"

cat >"$fakebin/uname" <<'EOF'
#!/bin/sh
case "${1:-}" in
  -s) printf '%s\n' "${FAKE_UNAME_S:-Linux}" ;;
  -m) printf '%s\n' "${FAKE_UNAME_M:-x86_64}" ;;
  *) exit 1 ;;
esac
EOF
chmod 0755 "$fakebin/uname"

run_installer() {
  test_home=$1
  shell_path=$2
  shift 2
  mkdir -p "$test_home"
  env \
    HOME="$test_home" \
    SHELL="$shell_path" \
    PATH="$fakebin:/usr/bin:/bin" \
    FIXTURE_DIR="$fixture" \
    ZIPIT_GITHUB_REPOSITORY=acme/zipit-test \
    "$@" \
    sh "$repo_root/install.sh"
}

assert_once() {
  needle=$1
  file=$2
  [ "$(grep -Fxc "$needle" "$file")" -eq 1 ]
}

test_bash_profiles_and_fresh_shells() {
  test_home="$work/bash-home"
  run_installer "$test_home" /bin/bash env
  run_installer "$test_home" /bin/bash env

  managed_line='case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac # Added by Zipit installer'
  assert_once "$managed_line" "$test_home/.bashrc"
  assert_once "$managed_line" "$test_home/.profile"

  HOME="$test_home" PATH=/usr/bin:/bin \
    bash --noprofile --rcfile "$test_home/.bashrc" -ic \
    'command -v zipit >/dev/null && zipit version | grep -F "Zipit v0.1.0" >/dev/null'

  HOME="$test_home" PATH=/usr/bin:/bin \
    bash --login -c \
    'command -v zipit >/dev/null && zipit version | grep -F "Zipit v0.1.0" >/dev/null'
}

test_existing_bash_profile() {
  test_home="$work/bash-profile-home"
  mkdir -p "$test_home"
  printf '%s\n' '# existing content' '. "$HOME/.bashrc"' >"$test_home/.bash_profile"
  run_installer "$test_home" /bin/bash env

  managed_line='case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac # Added by Zipit installer'
  assert_once "$managed_line" "$test_home/.bashrc"
  assert_once "$managed_line" "$test_home/.bash_profile"
  grep -F '# existing content' "$test_home/.bash_profile" >/dev/null
  [ ! -e "$test_home/.profile" ]

  path_count=$(HOME="$test_home" PATH=/usr/bin:/bin bash --login -ic \
    'printf "%s\n" "$PATH"' 2>/dev/null |
    awk -v path="$test_home/.local/bin" -F: '{ count = 0; for (i = 1; i <= NF; i++) if ($i == path) count++; print count }')
  [ "$path_count" -eq 1 ]
}

test_zsh_fresh_shell() {
  test_home="$work/zsh-home"
  run_installer "$test_home" /bin/zsh env
  run_installer "$test_home" /bin/zsh env

  managed_line='case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac # Added by Zipit installer'
  assert_once "$managed_line" "$test_home/.zshrc"
  HOME="$test_home" ZDOTDIR="$test_home" PATH=/usr/bin:/bin \
    zsh -ic 'command -v zipit >/dev/null && zipit version | grep -F "Zipit v0.1.0" >/dev/null'
}

test_fish_fresh_shell() {
  test_home="$work/fish-home"
  run_installer "$test_home" /usr/bin/fish env
  run_installer "$test_home" /usr/bin/fish env

  fish_config="$test_home/.config/fish/conf.d/zipit.fish"
  assert_once 'fish_add_path "$HOME/.local/bin"' "$fish_config"
  HOME="$test_home" XDG_CONFIG_HOME="$test_home/.config" PATH=/usr/bin:/bin \
    fish -c 'type -q zipit; and zipit version | grep -F "Zipit v0.1.0" >/dev/null'
}

test_platform_mapping() {
  cases='Linux x86_64 linux_amd64
Linux aarch64 linux_arm64
Darwin x86_64 darwin_amd64
Darwin arm64 darwin_arm64'
  printf '%s\n' "$cases" | while read -r os arch expected; do
    test_home="$work/map-$expected"
    FAKE_UNAME_S="$os" FAKE_UNAME_M="$arch" \
      run_installer "$test_home" /bin/bash env ZIPIT_VERSION=v0.1.0
    "$test_home/.local/bin/zipit" version | grep -F 'Zipit v0.1.0' >/dev/null
  done
}

test_invalid_version_rejected() {
  test_home="$work/invalid-version-home"
  if run_installer "$test_home" /bin/bash env ZIPIT_VERSION=v0.1.0-rc.1; then
    echo 'prerelease version was accepted' >&2
    exit 1
  fi
  [ ! -e "$test_home/.local/bin/zipit" ]
}

test_failed_upgrade_preserves_binary() {
  test_home="$work/upgrade-home"
  run_installer "$test_home" /bin/bash env ZIPIT_VERSION=v0.1.0
  before=$(sha256sum "$test_home/.local/bin/zipit")

  printf '%s\n' 'corrupt archive' >"$fixture/zipit_0.1.0_linux_amd64.tar.gz"
  if run_installer "$test_home" /bin/bash env ZIPIT_VERSION=v0.1.0; then
    echo 'checksum mismatch was accepted' >&2
    exit 1
  fi
  after=$(sha256sum "$test_home/.local/bin/zipit")
  [ "$before" = "$after" ]
  "$test_home/.local/bin/zipit" version | grep -F 'Zipit v0.1.0' >/dev/null
}

test_bash_profiles_and_fresh_shells
test_existing_bash_profile
test_zsh_fresh_shell
test_fish_fresh_shell
test_platform_mapping
test_invalid_version_rejected
test_failed_upgrade_preserves_binary

echo 'Unix installer tests passed.'
