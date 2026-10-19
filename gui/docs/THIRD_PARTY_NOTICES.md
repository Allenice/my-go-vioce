[简体中文](THIRD_PARTY_NOTICES.zh_CN.md) · English

# Third-party notices

MyGo is an independently implemented Go Voice desktop host. The Go Voice artwork from `assets/brand/go-voice-logo.png` is composited onto a black rounded tile in `gui/resources/icon.png` for packaging; this document grants no additional artwork or trademark rights.

| Source | Use | License / notice |
| --- | --- | --- |
| FoloToy AI Passport and the imported Vokie reference at `ce26ea043251102155e54bc9503eb8133d5cafa8` | Bluetooth helper adapted from `vokie-plugin/helper/AiPassportHelper.swift`; ADPCM and session behavior ported from its worker | MIT, Copyright (c) 2026 FoloToy; full notice in `licenses/FoloToy-MIT.txt` |
| MyGo v0.5.5 | Native application, controls, packaging | MIT, Copyright (c) 2026 MyGo contributors; `licenses/MyGo-MIT.txt` |
| purego v0.11.1 | Security / CoreFoundation bindings and MyGo dependency | Apache-2.0; `licenses/purego-Apache-2.0.txt` |
| go-text/typesetting v0.3.5 | MyGo text layout dependency | Retained original LICENSE and HarfBuzz notice in `licenses/` |
| golang.org/x/image v0.46.0 | MyGo image dependency | BSD; `licenses/x-image-BSD.txt` |
| golang.org/x/text v0.43.0 | MyGo locale (i18n) dependency | BSD; `licenses/x-text-BSD.txt` |
| espflash 4.6.0, <https://github.com/esp-rs/espflash> | Rust library used by FirmwareHelper for USB flashing; an esp-rs project, not Espressif's official primary flashing tool | MIT OR Apache-2.0; full upstream texts in `licenses/espflash-MIT.txt` and `licenses/espflash-Apache-2.0.txt`; version pinned by `firmware-helper/Cargo.lock` |
| serialport 4.10.1, <https://crates.io/crates/serialport/4.10.1> | macOS serial enumeration and access used by FirmwareHelper and espflash | MPL-2.0; full crate license in `licenses/serialport-MPL-2.0.txt`; unmodified source is available at the exact crates.io version |
| Yap, <https://github.com/FrigadeHQ/yap> | Architecture reference for SpeechAnalyzer, target capture, conditional clipboard restore, local refinement | No Yap source was copied; implementation follows the repository's bilingual design notes and installed SDK APIs |

The local `mygo-vaultwarden` project was consulted only for public MyGo API usage. Its GPL business implementation was not copied. No Vokie icon, name, brand sound, firmware binary, or proprietary symbol is included in this application. Retained direct dependency licenses and the required serialport MPL notice ship in the app resources; the exact Rust dependency graph is pinned by `Cargo.lock`. The Go toolchain and Apple system frameworks remain subject to their own distributions' terms.
