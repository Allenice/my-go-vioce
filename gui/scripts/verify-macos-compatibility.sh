#!/bin/bash
# Check the actual executables, not just the Finder-facing Info.plist.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
[[ $# == 1 ]] || { echo "Usage: $0 <app-bundle>" >&2; exit 2; }
app="$1"
minimum="$(plutil -extract macos.minimumSystemVersion raw -o - "$root/mygo.json")"
fail() { echo "macOS compatibility: $*" >&2; exit 1; }
valid_version() { [[ "$1" =~ ^[0-9]+(\.[0-9]+){0,2}$ ]]; }
# Compare version components numerically, including patch versions.
version_at_most() {
  awk -v actual="$1" -v maximum="$2" 'BEGIN {
    split(actual, a, "."); split(maximum, b, ".")
    for (i = 1; i <= 3; i++) {
      if (a[i]+0 < b[i]+0) exit 0
      if (a[i]+0 > b[i]+0) exit 1
    }
    exit 0
  }'
}
valid_version "$minimum" || fail "Invalid configured minimum: $minimum"

check_bundle() {
  local bundle="$1" plist executable declared binary details platform minos
  plist="$bundle/Contents/Info.plist"
  executable="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$plist")"
  [[ -n "$executable" && "$executable" != */* && "$executable" != . && "$executable" != .. ]] ||
    fail "$bundle has an invalid CFBundleExecutable."
  declared="$(/usr/libexec/PlistBuddy -c 'Print :LSMinimumSystemVersion' "$plist")"
  valid_version "$declared" || fail "$bundle has an invalid minimum: $declared"
  version_at_most "$declared" "$minimum" ||
    fail "$bundle declares macOS $declared, above the supported $minimum."
  binary="$bundle/Contents/MacOS/$executable"
  [[ -x "$binary" ]] || fail "$binary is missing or not executable."
  lipo -verify_arch arm64 "$binary" || fail "$binary has no arm64 executable."
  details="$(xcrun vtool -arch arm64 -show-build "$binary")"
  platform="$(awk '$1 == "platform" {print $2}' <<< "$details")"
  minos="$(awk '$1 == "minos" {print $2}' <<< "$details")"
  [[ "$platform" == MACOS ]] || fail "$binary has no unique macOS build version."
  valid_version "$minos" || fail "$binary has no valid Mach-O minimum system version."
  version_at_most "$minos" "$declared" ||
    fail "$binary requires macOS $minos but its Info.plist declares $declared."
  printf 'macOS compatibility: %s arm64, binary %s, declared %s, supported %s\n' \
    "${bundle##*/}" "$minos" "$declared" "$minimum"
}

# The outer app must advertise the same supported baseline as the build config.
outer_minimum="$(/usr/libexec/PlistBuddy -c 'Print :LSMinimumSystemVersion' "$app/Contents/Info.plist")"
valid_version "$outer_minimum" || fail "Invalid application minimum: $outer_minimum"
version_at_most "$minimum" "$outer_minimum" && version_at_most "$outer_minimum" "$minimum" ||
  fail "Application minimum $outer_minimum does not match the configured $minimum."
check_bundle "$app"
for helper in BluetoothHelper SpeechBridge FirmwareHelper; do
  check_bundle "$app/Contents/Resources/native/$helper.app"
done
