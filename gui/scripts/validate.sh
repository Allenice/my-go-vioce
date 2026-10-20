#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
bun_bin="${WAKU_BUN_EXECUTABLE:-bun}"
"$bun_bin" test scripts/package-firmware.test.mjs scripts/stage-debug-project.test.mjs
cargo fmt --manifest-path firmware-helper/Cargo.toml --all -- --check
cargo test --manifest-path firmware-helper/Cargo.toml --locked
"$bun_bin" scripts/test-helper-manifest-contract.mjs
cargo clippy --manifest-path firmware-helper/Cargo.toml --locked --all-targets -- -D warnings
if [[ -n "$(gofmt -l main.go internal)" ]]; then
  echo 'Run gofmt before validation.' >&2
  exit 1
fi
swift build --package-path macos -c debug
swift test --package-path macos
go test ./...
go vet ./...
./scripts/build.sh
app=build/debug/darwin-arm64/go-voice-debug.app
"$bun_bin" scripts/verify-packaged-firmware.mjs "$app/Contents/Resources"
for helper in BluetoothHelper SpeechBridge FirmwareHelper; do
  bundle="$app/Contents/Resources/native/$helper.app"
  test -x "$bundle/Contents/MacOS/$helper"
  case "$helper" in
    BluetoothHelper) suffix=bluetooth ;;
    SpeechBridge) suffix=speech ;;
    FirmwareHelper) suffix=firmware-helper ;;
  esac
  helper_id=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$bundle/Contents/Info.plist")
  [[ "$helper_id" == "yz.sh.mygo-go-voice.$suffix.debug" ]]
  codesign --verify --strict "$bundle"
  lipo -verify_arch arm64 "$bundle/Contents/MacOS/$helper"
done
app_id=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app/Contents/Info.plist")
[[ "$app_id" == yz.sh.mygo-go-voice.debug ]]
app_name=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleName' "$app/Contents/Info.plist")
[[ "$app_name" == go-voice-debug ]]
main_executable=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$app/Contents/Info.plist")
lipo -verify_arch arm64 "$app/Contents/MacOS/$main_executable"
codesign --verify --deep --strict "$app"
test ! -e firmware
