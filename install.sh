#!/bin/sh
set -eu

fail() {
  echo "zipit installer: $*" >&2
  exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

: "${HOME:?HOME must be set}"

repository=${ZIPIT_GITHUB_REPOSITORY:-poizdev/zipit}
case "$repository" in
  */*) ;;
  *) fail "ZIPIT_GITHUB_REPOSITORY must be in owner/repository form" ;;
esac
owner=${repository%%/*}
name=${repository#*/}
case "$owner$name" in
  ""|*[!A-Za-z0-9_.-]*) fail "invalid GitHub repository: $repository" ;;
esac
case "$name" in
  */*) fail "invalid GitHub repository: $repository" ;;
esac

version=${ZIPIT_VERSION:-}
if [ -z "$version" ]; then
  latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' \
    "https://github.com/$repository/releases/latest") || fail "could not find the latest stable release"
  version=${latest_url##*/}
fi

if ! printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'; then
  fail "version must be a stable vMAJOR.MINOR.PATCH tag: $version"
fi
artifact_version=${version#v}

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac

asset="zipit_${artifact_version}_${os}_${arch}.tar.gz"
base_url="https://github.com/$repository/releases/download/$version"
tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/zipit-install.XXXXXX") || fail "could not create temporary directory"
stage=
cleanup() {
  [ -z "$stage" ] || rm -f "$stage"
  rm -rf "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

archive="$tmp_dir/$asset"
checksums="$tmp_dir/checksums.txt"

echo "Installing Zipit $version for $os/$arch..."
curl -fsSL -o "$archive" "$base_url/$asset" || fail "could not download $asset"
curl -fsSL -o "$checksums" "$base_url/checksums.txt" || fail "could not download checksums.txt"

expected=$(awk -v file="$asset" '$2 == file { print $1 }' "$checksums")
case "$expected" in
  *'\n'*) fail "checksums.txt contains duplicate entries for $asset" ;;
esac
[ "${#expected}" -eq 64 ] || fail "checksums.txt has no valid SHA-256 entry for $asset"
case "$expected" in
  *[!0-9A-Fa-f]*) fail "checksums.txt has an invalid SHA-256 entry for $asset" ;;
esac

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$archive" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$archive" | awk '{ print $1 }')
else
  fail "sha256sum or shasum is required to verify the download"
fi

[ "$actual" = "$expected" ] || fail "checksum verification failed for $asset"

entries=$(tar -tzf "$archive") || fail "could not inspect $asset"
[ "$entries" = "zipit" ] || fail "$asset contains unexpected files"
tar -xzf "$archive" -C "$tmp_dir" zipit || fail "could not extract $asset"
[ -f "$tmp_dir/zipit" ] || fail "$asset does not contain zipit"

install_dir="$HOME/.local/bin"
install_path="$install_dir/zipit"
mkdir -p "$install_dir" || fail "could not create $install_dir"
stage="$install_dir/.zipit.tmp.$$"
cp "$tmp_dir/zipit" "$stage" || fail "could not stage zipit"
chmod 0755 "$stage" || fail "could not make zipit executable"
mv -f "$stage" "$install_path" || fail "could not install zipit"
stage=

append_managed_line() {
  file=$1
  line=$2
  if [ -f "$file" ] && grep -Fqx "$line" "$file"; then
    return
  fi
  if [ -s "$file" ]; then
    printf '\n' >>"$file"
  fi
  printf '%s\n' "$line" >>"$file"
}

path_updated=false
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *)
    shell_name=${SHELL##*/}
    unix_line='case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac # Added by Zipit installer'
    case "$shell_name" in
      bash)
        append_managed_line "$HOME/.bashrc" "$unix_line"
        if [ -e "$HOME/.bash_profile" ]; then
          login_file="$HOME/.bash_profile"
        elif [ -e "$HOME/.bash_login" ]; then
          login_file="$HOME/.bash_login"
        else
          login_file="$HOME/.profile"
        fi
        append_managed_line "$login_file" "$unix_line"
        ;;
      zsh)
        append_managed_line "$HOME/.zshrc" "$unix_line"
        ;;
      fish)
        fish_dir="$HOME/.config/fish/conf.d"
        mkdir -p "$fish_dir"
        append_managed_line "$fish_dir/zipit.fish" 'fish_add_path "$HOME/.local/bin"'
        ;;
      *)
        echo "Installed binary, but unsupported shell '$shell_name' was not configured." >&2
        path_updated=false
        ;;
    esac
    if [ "$shell_name" = bash ] || [ "$shell_name" = zsh ] || [ "$shell_name" = fish ]; then
      path_updated=true
    fi
    ;;
esac

echo "Installed Zipit to $install_path"
if [ "$path_updated" = true ]; then
  echo "PATH configured for future $shell_name shells."
  if [ "$shell_name" = fish ]; then
    echo 'Open a new terminal, or run: fish_add_path "$HOME/.local/bin"'
  else
    echo 'Open a new terminal, or run: export PATH="$HOME/.local/bin:$PATH"'
  fi
fi
