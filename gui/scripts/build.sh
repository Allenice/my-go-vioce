#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
# --dev keeps the staging for the whole session, then rebuilds the signed
# debug app and restarts it whenever the Go sources or configuration change.
# `mygo dev` is not used: it spawns the app as its own child, so TCC treats
# the launching terminal as responsible and kills BluetoothHelper. Launching
# through LaunchServices (`open`) keeps go-voice as its own TCC identity.
# --release builds the production app into build/release: signed with
# MYGO_SIGN_IDENTITY (a Developer ID), notarized with macos.notarize, and with
# MYGO_UPDATER_PRIVATE_KEY the signed update files (scripts/release.sh).
mode=build
case "${1:-}" in
  --dev) mode=dev ;;
  --release) mode=release ;;
  '') ;;
  *) echo "Usage: $0 [--dev|--release]" >&2; exit 2 ;;
esac
if [[ "$mode" == release && "${MYGO_SIGN_IDENTITY:-}" != "Developer ID Application: "* ]]; then
  echo 'A release needs MYGO_SIGN_IDENTITY set to a Developer ID Application identity.' >&2
  exit 1
fi
swift_config=debug
if [[ "$mode" == release ]]; then
  swift_config=release
fi
if [[ "$(uname -m)" != arm64 ]]; then
  echo 'MyGo requires Apple Silicon.' >&2
  exit 1
fi
bun_bin="${WAKU_BUN_EXECUTABLE:-bun}"
# Never replace an unknown directory: this build owns and removes its staging only.
if [[ -e firmware || -L firmware ]]; then
  echo 'Remove the existing gui/firmware staging directory before building.' >&2
  exit 1
fi
native_staging=""
mkdir firmware
cleanup_staging() {
  rm -r -- firmware
  if [[ -n "${native_staging:-}" && -d "$native_staging" ]]; then
    rm -r -- "$native_staging"
  fi
}
trap cleanup_staging EXIT
"$bun_bin" scripts/package-firmware.mjs
cargo build --manifest-path firmware-helper/Cargo.toml --release --locked --target aarch64-apple-darwin
swift build --package-path macos -c "$swift_config"

native_staging="$(mktemp -d "${TMPDIR:-/tmp}/go-voice-native.XXXXXX")"
for helper in BluetoothHelper SpeechBridge FirmwareHelper; do
  bundle="$native_staging/$helper.app"
  mkdir -p "$bundle/Contents/MacOS"
  if [[ "$helper" == FirmwareHelper ]]; then
    install -m 0755 "${CARGO_TARGET_DIR:-firmware-helper/target}/aarch64-apple-darwin/release/FirmwareHelper" "$bundle/Contents/MacOS/$helper"
    install -m 0644 resources/FirmwareHelper-Info.plist "$bundle/Contents/Info.plist"
  else
    install -m 0755 "macos/.build/$swift_config/$helper" "$bundle/Contents/MacOS/$helper"
    install -m 0644 "macos/$helper/Info.plist" "$bundle/Contents/Info.plist"
  fi
  printf 'APPL????' > "$bundle/Contents/PkgInfo"
  # Ad hoc for now; a Developer ID build signs the helpers again with the app.
  codesign --force --sign - --timestamp=none "$bundle"
done

if [[ -e resources/native ]]; then
  rm -r -- resources/native
fi
mv "$native_staging" resources/native
native_staging=""

mygo() { go run github.com/egoist/mygo/cmd/mygo "$@"; }
if [[ "$mode" == release ]]; then
  # Out of the way of the debug app, which dev sessions rebuild.
  rm -rf build/release
  mygo build -platform darwin/arm64 -o build/release -sign "$MYGO_SIGN_IDENTITY"
  exit
fi

# Debug builds never notarize, nor make disk images or update files, and
# never update themselves.
build_app() {
  env -u MYGO_UPDATER_PRIVATE_KEY go run github.com/egoist/mygo/cmd/mygo build -debug -skip-dmg -skip-notarize -platform darwin/arm64 ${MYGO_SIGN_IDENTITY:+-sign "$MYGO_SIGN_IDENTITY"}
}
if [[ "$mode" != dev ]]; then
  build_app
  exit
fi

# TCC keys an ad-hoc signed app by its code hash, which every rebuild changes,
# so Accessibility granted to one build is refused for the next. A certificate
# keeps the designated requirement (identifier + certificate) stable.
if [[ -z "${MYGO_SIGN_IDENTITY:-}" ]]; then
  MYGO_SIGN_IDENTITY="$(security find-identity -v -p codesigning 2>/dev/null | awk '/"Apple Development: /{print $2; exit}')"
fi
if [[ -n "$MYGO_SIGN_IDENTITY" ]]; then
  echo "Signing with code-signing identity $MYGO_SIGN_IDENTITY"
else
  echo 'No Apple Development identity found: signing ad hoc; Accessibility must be re-granted after each rebuild.' >&2
fi

app="$PWD/build/debug/darwin-arm64/go-voice.app"
exe="$app/Contents/MacOS/go-voice"
log="$PWD/build/dev.log"
fingerprint() {
  find main.go go.mod go.sum mygo.json internal resources/icon.png resources/entitlements.plist -type f \
    -exec stat -f '%m %z %N' {} + | shasum
}
app_pid() { pgrep -xf "$exe" | head -n 1 || true; }
stop_app() {
  local pid
  pid="$(app_pid)"
  [[ -z "$pid" ]] && return
  kill -TERM "$pid" 2>/dev/null || true
  for _ in $(seq 1 50); do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 0.1
  done
  kill -KILL "$pid" 2>/dev/null || true
}
start_app() {
  open -n "$app" --stdout "$log" --stderr "$log"
  for _ in $(seq 1 50); do
    [[ -n "$(app_pid)" ]] && return 0
    sleep 0.1
  done
  echo 'go-voice did not start; see build/dev.log' >&2
}
trap 'stop_app; cleanup_staging' EXIT
trap 'exit 130' INT TERM

stop_app
build_app
: > "$log"
start_app
seen="$(fingerprint)"
echo "Watching for changes (app log: build/dev.log). Quit the app or press Ctrl+C to stop."
while true; do
  sleep 1
  now="$(fingerprint)"
  if [[ "$now" != "$seen" ]]; then
    seen="$now"
    echo "$(date +%T) change detected, rebuilding"
    # A build that fails to compile keeps the running app.
    if build_app; then
      stop_app
      start_app
    fi
  elif [[ -z "$(app_pid)" ]]; then
    echo 'go-voice quit; ending the session.'
    exit 0
  fi
done
