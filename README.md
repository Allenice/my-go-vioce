[简体中文](README.zh_CN.md) · English

# Go Voice (my-ai-audio)

AI-powered macOS voice input. The AI Passport firmware and MyGo-based `go-voice` macOS desktop application are now implemented in one repository and pass local firmware builds, GUI builds, and host tests.

The AI Passport / ESP32-C3 badge is the sole audio capture device. It continuously streams ES8311 audio to the Mac over IMA-ADPCM / BLE; the `go-voice` app, built with MyGo, runs system on-device ASR, injects dynamic vocabulary, refines text with Apple local models or an explicitly selected OpenAI-compatible service, and inserts it into the active input field.

The repository follows the Vokie firmware project layout: the root builds ESP-IDF firmware; `gui/` is an independent Go module containing the MyGo-based `go-voice` macOS application for pairing, live waveform/frame diagnostics, vocabulary, AI refinement, Speech, and cross-application insertion. The Mac microphone is never used, and badge audio is neither saved nor replayed.

## Phase-one firmware

The product and BLE advertisement name are **Go Voice**. Firmware is imported from Vokie commit `ce26ea043251102155e54bc9503eb8133d5cafa8`, retaining the FoloToy MIT attribution. It uses the user's Go Voice logo and its own UUIDs, while preserving the reference button interactions, BLE V1 audio framing, status UI, power behavior, startup sound/preemption, and protected Recovery layout.

Go Voice no longer requires the Vokie desktop host. A compatible host using the [Go Voice UUIDs and BLE V1 protocol](docs/ai-passport-ble-protocol.md) is required for capture and text input; the device does not perform ASR itself. Existing unmodified Vokie integrations are not compatible with the new UUIDs. The five-minute session limit remains the reference host's responsibility.

- [Firmware scope, provenance and validation](docs/go-voice-firmware.md)
- [Brand assets and reproduction](assets/README.md)
- [Third-party notices](docs/THIRD_PARTY_NOTICES.md): obtain independent authorization for the reference startup sound before release.
- Validation: `./tools/validate.sh --static`; with ESP-IDF 5.5.3 active, `./tools/validate.sh --firmware`. Do not erase device identity or permanent Recovery.

## go-voice macOS application

`go-voice` targets macOS 26+ on Apple Silicon, is built with MyGo, uses the product-owned Go Voice BLE UUIDs, and never auto-connects the first discovered device. Its four native pages cover device/current speech, dynamic vocabulary, AI refinement, and safe USB firmware installation. API keys live only in macOS Keychain, and local-model failures never silently switch to cloud processing.

The firmware page uses a signed Rust `FirmwareHelper` backed by pinned `espflash` 4.6.0. It recognizes the ESP32-C3 native USB Serial/JTAG port, performs fail-closed chip, security, flash-size, partition-table, image-hash and permanent-Recovery checks, and writes only the bundled factory application at `0x10000`. It has no erase-all, bootloader, partition-table, NVS, PHY, `cardid`, or Recovery write path.

```bash
./tools/validate.sh --gui
open gui/build/debug/darwin-arm64/go-voice.app
```

- [go-voice usage, build, and privacy](gui/README.md)
- [GUI implementation and acceptance record](gui/docs/acceptance.md)
- Build product: `gui/build/debug/darwin-arm64/go-voice.app`
- The bundle launch, four pages, zh-CN model warmup, and real USB Serial/JTAG enumeration are verified. Actual installer preflight/write/reset, badge BLE, Speech/Accessibility/Automation grants, ASR, refinement, and insertion remain interactive hardware acceptance items.

## Documentation

- [Documentation index](docs/README.md) — use, build, validation, hardware and contribution guides.
- [Project structure](docs/project-structure.md) — current firmware and desktop source layout.
- [System architecture](docs/passport-audio-input-architecture.md) — audio transport, session ownership, recognition and insertion.
