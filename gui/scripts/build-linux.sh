#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ "$(uname -s)" != Linux ]]; then
  echo 'Run this script on Linux; gojieba requires a native C++ toolchain.' >&2
  exit 1
fi
for tool in go cc c++ tar; do
  command -v "$tool" >/dev/null || { echo "Missing build tool: $tool" >&2; exit 1; }
done
arch="$(go env GOARCH)"
case "$arch" in amd64|arm64) ;; *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; esac
export CGO_ENABLED=1
# Temporary staging means a failed build leaves the previous package usable.
mkdir -p build/linux
staging="$(mktemp -d "$PWD/build/linux/.staging.XXXXXX")"
trap 'rm -rf -- "$staging"' EXIT
app="$staging/go-voice"
mkdir -p "$app/native" "$app/resources/jieba" "$app/docs"
go build -trimpath -o "$app/go-voice" .
go build -trimpath -o "$app/native/BluetoothHelper" ./cmd/linux-bluetooth
go build -trimpath -o "$app/native/SpeechBridge" ./cmd/linux-text
jieba_module="$(go list -m -f '{{.Dir}}' github.com/yanyiwu/gojieba)"
for dictionary in jieba.dict.utf8 hmm_model.utf8 user.dict.utf8 idf.utf8 stop_words.utf8; do
  install -m 0644 "$jieba_module/deps/cppjieba/dict/$dictionary" "$app/resources/jieba/$dictionary"
done
cp -R licenses "$app/"
cp docs/THIRD_PARTY_NOTICES*.md docs/linux*.md "$app/docs/"
install -m 0644 resources/icon.png "$app/resources/icon.png"
install -m 0755 scripts/launch-linux.sh "$app/launch.sh"
install -m 0755 scripts/install-linux.sh "$app/install.sh"
GOVOICE_JIEBA_TEST_DIR="$app/resources/jieba" go test -count=1 ./internal/segmentation -run TestPackagedDictionaries
package="go-voice-linux-$arch"
tar -C "$staging" -czf "$staging/$package.tar.gz" go-voice
rm -rf -- "build/linux/$package"
mv "$app" "build/linux/$package"
mv "$staging/$package.tar.gz" "build/linux/$package.tar.gz"
echo "Built: $PWD/build/linux/$package.tar.gz"
echo "Run: $PWD/build/linux/$package/launch.sh"
