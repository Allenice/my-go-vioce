[简体中文](linux.zh_CN.md) · English

# Linux desktop preview

This source-build preview targets Ubuntu/Debian desktops on amd64 and arm64.
Keep the existing Go Voice firmware on the AI Passport. The Linux host uses the
same UUIDs, BLE V1 framing, Go decoder, session limits, vocabulary and online ASR
and remote refinement implementations as the macOS app.

This is not yet a hardware-validated Linux release. Host tests and cross-built
helpers cannot verify a Bluetooth adapter, BlueZ permissions, desktop focus,
Secret Service or a real recognition provider. Complete the checklist below on
the intended computer before relying on it for dictation.

## Prerequisites

Install Go **1.27.1** from the [official Go distribution](https://go.dev/dl/),
then check `go version`. A distribution's default Go package may be older.
The build uses CGO for gojieba and needs a native C/C++ compiler. Run it on Linux.
It does not need Bun, Xcode, Swift, Rust, ESP-IDF or a freshly built firmware.

On an Ubuntu/Debian desktop:

```bash
sudo apt update
sudo apt install build-essential pkg-config libgtk-3-dev libgl1 libepoxy0 \
  libayatana-appindicator3-1 bluez dbus-user-session \
  libsecret-tools gnome-keyring xclip xdotool wl-clipboard
```

Use an ordinary logged-in desktop session, with Bluetooth enabled and a running,
unlocked Secret Service keyring. GNOME normally manages its login keyring; other
desktops must provide a compatible Secret Service. Do not launch the app with
`sudo`. No extra Bluetooth capability, system-wide D-Bus policy or `uinput`
access is required by this implementation. Distribution policy may still deny
BlueZ access; fix that through the desktop's normal Bluetooth configuration.

## Build and install

From the repository root:

```bash
./tools/validate.sh --static
./tools/validate.sh --gui
```

The second command selects the Linux gate: Go tests, vet, native compilation,
packaged-dictionary verification and archive creation. For a build alone:

```bash
cd gui
./scripts/build-linux.sh
```

For amd64, outputs are `gui/build/linux/go-voice-linux-amd64/` and the adjacent
`go-voice-linux-amd64.tar.gz`; arm64 uses the analogous suffix. From the root:

```bash
./gui/build/linux/go-voice-linux-amd64/launch.sh
# Optional: copy to the current user's applications directory and add a launcher.
./gui/build/linux/go-voice-linux-amd64/install.sh
```

Adjust the suffix for arm64. Keep the entire directory together: `go-voice`,
`native/`, `resources/`, `licenses/` and `docs/`. The installer copies it to
`${XDG_DATA_HOME:-$HOME/.local/share}/go-voice` and writes a desktop entry in its
`applications` sibling. Build on the same distribution version as the target,
or an older compatible one: CGO binaries depend on the target's libc/C++ runtime.
Automatic updates and the macOS development watcher are not offered in this
preview; rebuild and rerun the installer after closing the app to update it.

The repository's full `./tools/validate.sh` additionally builds firmware and
still requires ESP-IDF 5.5.3. It is not required just to package the Linux host.

## First use

1. Start the app, enable system Bluetooth, scan and explicitly select the badge.
   Devices are filtered by the Go Voice service. The selected BlueZ device path
   is saved for reconnecting; the first scanned badge is never selected for you.
2. In Speech Recognition, choose an online provider, enter its API key and save.
   The initial unselected state sends no audio to any service. There is no Apple
   local recognizer on Linux. A failed request never selects another provider.
3. Leave AI refinement off, or explicitly configure an OpenAI-compatible
   endpoint. Only the selected online recognizer receives audio; enabled remote
   refinement receives the transcript and relevant vocabulary. Audio and
   transcript history are not written to disk.
4. Focus an input field, use UP to start/stop dictation, and check the result.

Settings and vocabulary use `${XDG_CONFIG_HOME:-$HOME/.config}/MyGo-GoVoice/`.
Credentials are stored in Secret Service through `secret-tool`, with secrets
passed on stdin. Missing or locked keyrings cause an error, never plaintext
storage. The saved-key hint searches item metadata without reading the secret.

## Desktop behavior and limits

| Capability | X11 | Wayland |
| --- | --- | --- |
| Clipboard | `xclip` | `wl-copy` |
| Automatic paste | `xdotool`, only while the original target remains focused | Manual paste after copying |
| Badge Enter/Delete/Clear | Sent to the focused app with `xdotool` | Not implemented |
| Local Apple models | Unavailable | Unavailable |
| USB firmware installation | Unavailable | Unavailable |

Text stays in the clipboard after output; this preview does not restore earlier
clipboard content. When an X11 user changes windows during dictation, the app
copies the result without switching windows or automatically pasting it. Apps
whose paste shortcut is not Ctrl+V, including some terminals, may require manual
paste. Wayland users also paste manually; XWayland does not enable X11 injection
into native Wayland applications. Missing `xdotool` degrades to clipboard output.
Keep the window open if the desktop does not show the tray icon.

The Bluetooth helper uses BlueZ D-Bus, acknowledged Control writes, and
`AcquireNotify` for audio. It requires a negotiated MTU of at least 185, validates
the info `hello`, subscribes to Control and Audio before sending `host_ready`,
and uses the existing bounded frame assembler. It does not change adapter power,
pair a device, install policies, or modify firmware. The firmware's BLE security
limitations remain those in the [protocol](../../docs/ai-passport-ble-protocol.md).

## Verify on the Linux computer

- Record distribution, CPU architecture, `echo "$XDG_SESSION_TYPE"`, `go version`
  and `bluetoothctl --version`.
- Build the archive, launch the extracted app and the installed launcher, and
  verify dictionaries still work after the build directory/module cache is gone.
- Check explicit device selection, handshake, multiple recording sessions,
  cancellation, disconnect/reconnect and quitting during recording.
- Check key save/reuse/delete, a locked keyring, missing provider credentials and
  provider failures. Live provider tests can incur usage charges.
- Check X11 same-target paste, changed-target clipboard fallback, or Wayland
  copy/manual paste; also test missing clipboard tools and non-Ctrl+V targets.
- Confirm the Linux pages do not offer Apple permission prompts or USB writes.

Report build, host tests and device tests separately. Do not call a cross-build
or mocked helper test a successful Linux hardware test.
