<p align="right">
  <a href="CHANGELOG.zh_CN.md">简体中文</a> · <strong>English</strong>
</p>

# go-voice release notes

Each `## <version>` section is the release note of one signed `go-voice` release. `scripts/release.sh` requires the section in both language files; the Simplified Chinese section is signed into the update manifest and shown in the in-app update window, because the app interface is Chinese. Write user-visible changes only and keep both sections aligned. See [updates and releases](updates.md).

## 0.1.0

- First signed and notarized release of go-voice for Apple Silicon Macs running macOS 26 or later.
- Receive voice from a selected FoloToy AI Passport over Bluetooth, transcribe it on this Mac, optionally refine it, and insert it into the application you dictated into.
- Manage vocabulary, choose the AI refinement engine, and install the bundled application firmware over USB.
- Check for Updates… in the menu bar now installs signed updates; release builds also check once a day.
