#!/bin/sh
set -eu

dist=${1:-dist}

fail() {
  echo "release package test: $*" >&2
  exit 1
}

[ "$(find "$dist" -maxdepth 1 -type f \( -name '*.tar.gz' -o -name '*.zip' \) | wc -l)" -eq 6 ] ||
  fail "expected six direct archives"
[ "$(find "$dist" -maxdepth 1 -type f -name '*.deb' | wc -l)" -eq 2 ] ||
  fail "expected two Debian packages"
[ "$(find "$dist" -maxdepth 1 -type f -name '*.rpm' | wc -l)" -eq 2 ] ||
  fail "expected two RPM packages"
[ "$(wc -l <"$dist/checksums.txt")" -eq 10 ] ||
  fail "expected checksums for six archives and four packages"

check_direct_archive() {
  target=$1
  extension=$2
  expected=$3
  archive=$(find "$dist" -maxdepth 1 -type f -name "zipit_*_${target}.${extension}")
  [ "$(printf '%s\n' "$archive" | sed '/^$/d' | wc -l)" -eq 1 ] ||
    fail "expected one direct archive for $target"

  case "$extension" in
    tar.gz) contents=$(tar -tzf "$archive") ;;
    zip) contents=$(bsdtar -tf "$archive") ;;
    *) fail "unsupported direct archive format: $extension" ;;
  esac
  [ "$contents" = "$expected" ] ||
    fail "$(basename "$archive") must contain only $expected; found: $(printf '%s' "$contents" | tr '\n' ' ')"
}

check_direct_archive linux_amd64 tar.gz zipit
check_direct_archive linux_arm64 tar.gz zipit
check_direct_archive darwin_amd64 tar.gz zipit
check_direct_archive darwin_arm64 tar.gz zipit
check_direct_archive windows_amd64 zip zipit.exe
check_direct_archive windows_arm64 zip zipit.exe

check_payload_listing() {
  listing=$1
  printf '%s\n' "$listing" | sed -e 's#^\./##' -e 's#^/##' | while IFS= read -r entry; do
    case "$entry" in
      ""|usr|usr/|usr/bin|usr/bin/|usr/bin/zipit) ;;
      *) fail "unexpected package payload entry: $entry" ;;
    esac
  done
  printf '%s\n' "$listing" | sed -e 's#^\./##' -e 's#^/##' | grep -Fx 'usr/bin/zipit' >/dev/null ||
    fail "package does not install /usr/bin/zipit"
}

for package in "$dist"/*.deb; do
  members=$(ar t "$package")
  data_member=$(printf '%s\n' "$members" | awk '/^data\.tar\./ { print; exit }')
  control_member=$(printf '%s\n' "$members" | awk '/^control\.tar\./ { print; exit }')
  [ -n "$data_member" ] || fail "missing Debian data archive in $package"
  [ -n "$control_member" ] || fail "missing Debian control archive in $package"

  payload=$(ar p "$package" "$data_member" | bsdtar -tf -)
  check_payload_listing "$payload"
  ar p "$package" "$data_member" | bsdtar -tvf - |
    grep -E -- '^-rwxr-xr-x[[:space:]]+0[[:space:]]+root[[:space:]]+root[[:space:]].*usr/bin/zipit$' >/dev/null ||
    fail "Debian binary mode or ownership is incorrect in $package"

  control=$(ar p "$package" "$control_member" | bsdtar -xOf - ./control)
  printf '%s\n' "$control" | grep -Fx 'Package: zipit' >/dev/null || fail "wrong Debian package name"
  printf '%s\n' "$control" | grep -E '^Version: [0-9]+\.[0-9]+\.[0-9]+' >/dev/null || fail "wrong Debian version"
  printf '%s\n' "$control" | grep -E '^Architecture: (amd64|arm64)$' >/dev/null || fail "wrong Debian architecture"
done

for package in "$dist"/*.rpm; do
  payload=$(bsdtar -tf "$package")
  check_payload_listing "$payload"
  bsdtar -tvf "$package" |
    grep -E -- '^-rwxr-xr-x[[:space:]]+1[[:space:]]+0[[:space:]]+0[[:space:]].*/usr/bin/zipit$' >/dev/null ||
    fail "RPM binary mode or ownership is incorrect in $package"
  case "$package" in
    *_linux_amd64.rpm) rpm_arch=x86_64 ;;
    *_linux_arm64.rpm) rpm_arch=aarch64 ;;
    *) fail "unexpected RPM filename: $package" ;;
  esac
  strings "$package" | grep -Fx "$rpm_arch" >/dev/null ||
    fail "wrong RPM architecture in $package"
  strings "$package" | grep -E '^0\.[0-9]+\.[0-9]+.*SNAPSHOT' >/dev/null ||
    fail "RPM version is missing or has an unexpected leading v in $package"
done

(
  cd "$dist"
  sha256sum -c checksums.txt >/dev/null
) || fail "release checksums do not verify"

echo 'Release package tests passed.'
