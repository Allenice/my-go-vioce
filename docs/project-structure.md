[简体中文](project-structure.zh_CN.md) · English

# Project structure

The repository root builds ESP-IDF firmware for AI Passport. `gui/` is an independent Go module for the MyGo-based macOS application. Neither build requires nesting or moving the other project.

```text
main/                         Firmware application, BLE audio, buttons and status UI
components/bsp/               Reusable board drivers and public hardware interfaces
bootloader_components/        Permanent-Recovery bootloader hook
assets/                       Brand artwork, firmware images and startup sound
sdkconfig.defaults            ESP32-C3, Bluetooth and power defaults
partitions.csv                Application and protected identity/Recovery layout
dependencies.lock             Pinned ESP-IDF managed components
tests/                        Firmware host tests and ESP-IDF/LVGL test doubles
tools/                        Shared checks, firmware verification and asset generators
gui/
  main.go                     Desktop entry point
  go.mod, go.sum              Independent Go module and pinned dependencies
  mygo.json                   Application identity and packaging configuration
  internal/
    app/                      Composition, helper lifecycle and UI event dispatch
    device/                   BLE session coordinator and cancellation
    protocol/                 Control validation, ADPCM, fragments and stream ordering
    vocabulary/               Persistent terms, provenance and user locks
    refinement/               Local/remote configuration, fidelity and fallback
    firmware/                 USB helper protocol and installer process management
    permission/               Permission state and guidance
    platform/bridge/          Native helper processes and bounded message transport
    platform/keychain/        macOS Keychain access
    ui/                       Native pages, permission flow, HUD and waveform
  macos/
    BluetoothHelper/          CoreBluetooth subprocess
    SpeechBridge/             Speech, FoundationModels and TextInjector subprocess
    BridgeCore/               Shared native message and permission contracts
    Tests/                    Native contract tests
  firmware-helper/            Rust app-only USB installer, pinned espflash dependency
  resources/                  Desktop and menu-bar icons
  scripts/                    Builds, development launch, packaging and validation
  docs/                       Desktop acceptance and dependency notices
website/                      Product and download site (Cloudflare Workers static assets)
docs/                         Product, protocol, hardware and development documentation
.github/                      CI, issue forms and community documents
LICENSE, LICENSES/            Project and third-party licenses and notices
```

## Ownership

`main/vokie_ble.c` owns badge audio sessions and BLE transport; the `vokie_` implementation names retain the imported source structure. Public identity is Go Voice. Hardware facts belong in BSP headers, particularly `components/bsp/include/bsp_pins.h`. Application state, buttons, animations and workers belong in `main/`.

On the Mac, `internal/app` composes services. The device coordinator owns session identity, cancellation and terminal state; pure Go protocol code validates and decodes audio. UI code emits actions and receives main-thread updates rather than owning helpers or Speech sessions. Swift provides CoreBluetooth and on-device Speech; Rust provides the USB firmware installer. Credentials belong only in Keychain. Audio and transcript state remain transient; there is no transcript-history module.

The authoritative [BLE V1 protocol](ai-passport-ble-protocol.md) is shared by firmware and desktop code. Do not duplicate its UUID or framing specification in GUI documentation.

## Build products

- `build/FoloToy-AI-Passport-full.bin`: verified merged firmware for the documented mini-program/Recovery flow.
- `build/FoloToy-AI-Passport.bin`: verified application image used by desktop packaging.
- `gui/build/debug/darwin-arm64/go-voice-debug.app`: development desktop bundle with separate `.debug` permission identities, signed native helpers and packaged app-only firmware.

Build directories, staged helpers, temporary firmware packaging and generated image experiments are excluded from source control. See [build and test](development/engineering/build-and-test.md), the [desktop build guide](../gui/README.md#build-and-validate), and [system architecture](passport-audio-input-architecture.md).
