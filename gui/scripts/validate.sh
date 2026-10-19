#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
bun_bin="${WAKU_BUN_EXECUTABLE:-bun}"
"$bun_bin" test scripts/package-firmware.test.mjs
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
app=build/debug/darwin-arm64/go-voice.app
"$bun_bin" scripts/verify-packaged-firmware.mjs "$app/Contents/Resources"
for helper in BluetoothHelper SpeechBridge FirmwareHelper; do
  bundle="$app/Contents/Resources/native/$helper.app"
  test -x "$bundle/Contents/MacOS/$helper"
  codesign --verify --strict "$bundle"
  lipo -verify_arch arm64 "$bundle/Contents/MacOS/$helper"
done
helper_id=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app/Contents/Resources/native/FirmwareHelper.app/Contents/Info.plist")
[[ "$helper_id" == yz.sh.mygo-go-voice.firmware-helper ]]
main_executable=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$app/Contents/Info.plist")
lipo -verify_arch arm64 "$app/Contents/MacOS/$main_executable"
codesign --verify --deep --strict "$app"
test ! -e firmware
