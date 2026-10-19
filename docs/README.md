[简体中文](README.zh_CN.md) · English

# Go Voice documentation

Go Voice combines AI Passport firmware with a MyGo-based macOS voice-input application. The badge captures audio; the Mac performs on-device speech recognition, vocabulary injection, optional AI refinement, and text insertion. Start with the [project README](../README.md) for supported hardware and software, then use the guides below.

## Use and build

- [macOS application](../gui/README.md): prerequisites, build commands, permissions, vocabulary, refinement, and privacy.
- [Firmware](go-voice-firmware.md): source attribution, Go Voice identity, display assets, protected storage, and recorded validation.
- [Environment setup](development/engineering/environment-setup.md): ESP-IDF 5.5.3 setup.
- [Build and test](development/engineering/build-and-test.md): shared validation entry points and firmware artifacts.
- [Hardware guide](hardware-design/AI_HARDWARE_DEVELOPMENT_GUIDE.md) and [specifications](hardware-design/specifications.md): board interfaces and resource limits. Pin definitions live in `components/bsp/include/bsp_pins.h`.

## Architecture and contracts

- [System architecture](passport-audio-input-architecture.md): badge-to-Mac audio flow, session ownership, vocabulary, refinement, and text insertion.
- [Project structure](project-structure.md): current firmware and desktop source layout.
- [BLE V1 protocol](ai-passport-ble-protocol.md): authoritative UUIDs, controls, audio framing, and compatibility.
- [Recovery compatibility](development/engineering/ble-recovery-compatibility.md): 3 MB application limit, `cardid` at `0x356000`, permanent Recovery at `0x700000`, and the five-second UP-key bootloader hook.
- [Standby power](development/engineering/standby-power-contract.md): lifecycle ownership, design rationale, failure policy, and device acceptance.

## Validation and distribution

```bash
./tools/validate.sh --static    # repository checks and firmware host tests
./tools/validate.sh --firmware  # ESP-IDF build and merged-image verification
./tools/validate.sh --gui       # desktop host tests, build and package checks
./tools/validate.sh             # complete gate
```

Firmware validation requires an activated ESP-IDF 5.5.3 environment. GUI validation requires the macOS toolchain and verified firmware artifacts described in the [application guide](../gui/README.md). Compilation and host tests do not establish hardware acceptance; the [desktop acceptance record](../gui/docs/acceptance.md) and [firmware validation record](go-voice-firmware.md#validation) list remaining checks.

- [Third-party notices](THIRD_PARTY_NOTICES.md), [desktop dependency notices](../gui/docs/THIRD_PARTY_NOTICES.md), and [asset provenance](../assets/README.md): redistribution terms and reproduction steps. Independent authorization for the retained reference startup sound remains required before distribution.
- [Changelog](CHANGELOG.md).
- [CI validation](development/ci/CI-validation.md) and [firmware releases](development/ci/CI-build-and-release.md).

## Contribute

- [Contribution guidelines](../.github/CONTRIBUTING.md), [code of conduct](../.github/CODE_OF_CONDUCT.md), [security](../.github/SECURITY.md), and [support](../.github/SUPPORT.md).
- [Documentation conventions](contribution/doc-conventions.md), [commit and PR rules](contribution/commit-and-pr.md), and [development guidelines](development/README.md).
- [Fork guide](fork-guide.md), [hardware reference experience](reference/README.md), and [upstream board/product background](brand/README.md).

AI-assisted work starts at [AGENTS.md](../AGENTS.md) and follows its task-specific routing.
