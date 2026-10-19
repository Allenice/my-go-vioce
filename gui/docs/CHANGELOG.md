<p align="right">
  <a href="CHANGELOG.zh_CN.md">简体中文</a> · <strong>English</strong>
</p>

# go-voice release notes

Each `## <version>` section is the release note of one signed `go-voice` release. `scripts/release.sh` requires the section in both language files; the Simplified Chinese section is signed into the update manifest and shown in the in-app update window, because the app interface is Chinese. Write user-visible changes only and keep both sections aligned. See [updates and releases](updates.md).

## 0.1.0

- First signed and notarized release of go-voice for Apple Silicon Macs running macOS 26 or later.
- Receive voice from a selected FoloToy AI Passport over Bluetooth, transcribe it on this Mac, optionally refine it, and insert it into the application you dictated into. A floating capsule previews the text and lets you cancel or insert it directly.
- Manage vocabulary and local replacement rules, and import or export them together as JSON. Replacements also work with AI refinement turned off.
- Choose Apple FoundationModels or an OpenAI-compatible refinement engine with Chat Completions or Responses. Correct vocabulary terms first, remove meaningless fillers in light mode, and test remote settings before saving.
- Follow the permissions guide and install the bundled application firmware over USB while preserving device identity and permanent Recovery.
- Check for Updates… in the menu bar now installs signed updates; release builds also check once a day.

Install the Apple Silicon DMG, drag go-voice to Applications, open it, and complete the permissions guide before selecting your FoloToy AI Passport. See the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.1.0/gui/README.md) for setup and the [release guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.1.0/gui/docs/updates.md) for building and signing.
