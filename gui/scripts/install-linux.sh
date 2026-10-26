#!/usr/bin/env bash
set -euo pipefail
source_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
data_dir="${XDG_DATA_HOME:-$HOME/.local/share}"
destination="$data_dir/go-voice"
if [[ "$EUID" == 0 ]]; then echo 'Install as your desktop user, without sudo.' >&2; exit 1; fi
mkdir -p "$destination" "$data_dir/applications"
if [[ "$source_dir" != "$(cd "$destination" && pwd)" ]]; then
  cp -R "$source_dir/." "$destination/"
fi
# Desktop Exec field quoting: quote the full path, including percent literals.
escaped="${destination//\\/\\\\}"
escaped="${escaped//\"/\\\"}"
escaped="${escaped//\$/\\\$}"
escaped="${escaped//\`/\\\`}"
escaped="${escaped//%/%%}"
cat > "$data_dir/applications/yz.sh.mygo-go-voice.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Go Voice
Comment=AI Passport voice input
Exec="$escaped/launch.sh"
Icon=$destination/resources/icon.png
Terminal=false
Categories=Utility;Accessibility;
EOF
echo "Installed to $destination"
echo 'Launch Go Voice from your desktop application menu.'
