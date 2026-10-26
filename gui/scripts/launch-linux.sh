#!/usr/bin/env bash
set -euo pipefail
app_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ "$EUID" == 0 ]]; then
  echo 'Run go-voice as your logged-in desktop user, without sudo.' >&2
  exit 1
fi
missing=()
for tool in secret-tool; do command -v "$tool" >/dev/null || missing+=("$tool"); done
if [[ -n "${WAYLAND_DISPLAY:-}" || "${XDG_SESSION_TYPE:-}" == wayland ]]; then
  command -v wl-copy >/dev/null || missing+=(wl-copy)
else
  command -v xclip >/dev/null || missing+=(xclip)
  command -v xdotool >/dev/null || echo 'xdotool is missing: text will remain in the clipboard.' >&2
fi
if (( ${#missing[@]} )); then
  echo "Missing desktop tools: ${missing[*]}" >&2
  echo 'Ubuntu/Debian: sudo apt install libsecret-tools gnome-keyring xclip xdotool wl-clipboard' >&2
  exit 1
fi
exec "$app_dir/go-voice" "$@"
