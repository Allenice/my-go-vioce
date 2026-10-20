<p align="right">
  <a href="CHANGELOG.zh_CN.md">简体中文</a> · <strong>English</strong>
</p>

# go-voice release notes

Each `## <version>` section is the release note of one signed `go-voice` release. `scripts/release.sh` requires the section in both language files; the Simplified Chinese section is signed into the update manifest and shown in the in-app update window, because the app interface is Chinese. Write user-visible changes only and keep both sections aligned. See [updates and releases](updates.md).

## 0.4.0

- Show the device and bundled firmware versions on the Firmware page, with an update reminder and a sidebar badge when the connected device is older than the copy packaged in go-voice. Bundled firmware is 0.2.0.
- Correct Chinese vocabulary and replacements on local word boundaries. Unique replacement targets fill unused Top 100 slots; final correction matches configured Chinese sources as complete tokens without crossing punctuation, including when AI refinement and Jev are off. Live previews keep literal replacement.
- Improve Jev vocabulary checks with local segmentation and the full transcript. Each occurrence compares overlapping homophone candidates against keeping the original; a replacement or protection applies only at 70% confidence. Requests send the full transcript, segmentation and matching candidates, never audio or unrelated vocabulary. Long requests are skipped. Live model quality remains unverified.
- Lower firmware standby draw: key scanning slows while the screen is off, the microphone path closes after five quiet seconds, and the display sleeps with the backlight. The first key after screen-off can take about 50 ms longer.

Install the Apple Silicon DMG on macOS 26 or later, or choose Check for Updates… in an installed release. After connecting a badge, compare versions on the Firmware page and install 0.2.0 when prompted. Build and setup instructions are in the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.4.0/gui/README.md). Live Jev quality, word-boundary correction on real dictation, measured standby current and a physical firmware update remain unverified.

## 0.3.0

- Choose Alibaba Cloud Omni Realtime WebSocket on the Speech Recognition page. Set its endpoint, model and API key; audio streams while recording and is submitted on key release. Only input transcription is inserted, with credentials stored separately from Alipay MaaS.
- Local builds are named `go-voice-debug` and use separate macOS permission identities for the main app and all helpers, so development builds do not replace the release app's permission records.
- Start a signed watch session with `bun run dev` from the repository root. Go source and configuration changes rebuild and restart the debug app automatically.
- Visit the [product website](https://go-voice.yzsaas.net/) for the latest Apple Silicon download, setup information and release history.

Install the Apple Silicon DMG on macOS 26 or later, or choose Check for Updates… in an installed release. Select Alibaba Cloud Omni Realtime on the Speech Recognition page and enter an API key for the chosen endpoint and model. Build and setup instructions are in the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.3.0/gui/README.md). Live Omni recognition, physical-device dictation and permission retention across an installed update remain unverified.

## 0.2.0

- Choose online speech recognition on the new Speech Recognition page: Alipay MaaS Fun-ASR and Paraformer v2 stream text while you speak; Qwen3-ASR-Flash transcribes the complete recording after you release the key. Apple on-device recognition remains the default. Online recognition sends session audio to Alipay MaaS, keeps its API key in Keychain, and reports failures without switching engines.
- See live recognition in the floating capsule: new words slide in from the right, and Apple's local recognizer now updates about once a second. The preview bubble appears during AI refinement so you can insert the ASR original directly.
- Optionally enable Jev on the vocabulary page to check Chinese homophone corrections before refinement, including when AI refinement is off. Supports TypeSafe and compatible System One providers such as Alipay MaaS. Only matching text spans, surrounding context and candidate terms are sent; API keys stay in Keychain. The check is off by default and is skipped after four seconds or on failure.
- Enable Open at Login on the new General page. go-voice starts in the menu bar and opens the permissions guide only when a required permission is missing; the switch follows macOS Login Items settings.

Install the Apple Silicon DMG on macOS 26 or later, or choose Check for Updates… in an installed release. See the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.2.0/gui/README.md) for setup and the [release guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.2.0/gui/docs/updates.md) for building and signing. Physical-device dictation, live provider quality, login-item behavior and an installed update remain unverified for this release.

## 0.1.0

- First signed and notarized release of go-voice for Apple Silicon Macs running macOS 26 or later.
- Receive voice from a selected FoloToy AI Passport over Bluetooth, transcribe it on this Mac, optionally refine it, and insert it into the application you dictated into. A floating capsule previews the text and lets you cancel or insert it directly.
- Manage vocabulary and local replacement rules, and import or export them together as JSON. Replacements also work with AI refinement turned off.
- Choose Apple FoundationModels or an OpenAI-compatible refinement engine with Chat Completions or Responses. Correct vocabulary terms first, remove meaningless fillers in light mode, and test remote settings before saving.
- Follow the permissions guide and install the bundled application firmware over USB while preserving device identity and permanent Recovery.
- Check for Updates… in the menu bar now installs signed updates; release builds also check once a day.

Install the Apple Silicon DMG, drag go-voice to Applications, open it, and complete the permissions guide before selecting your FoloToy AI Passport. See the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.1.0/gui/README.md) for setup and the [release guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.1.0/gui/docs/updates.md) for building and signing.
